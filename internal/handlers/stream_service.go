package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"
	"iter"
	"net/http"
	"schedulehub/external"
	httpclient "schedulehub/internal/http"
	"schedulehub/internal/schema"
	env "schedulehub/internal/secret"
	"slices"
	"sync"
)

// ScheduleService encapsulates the dependencies and methods for handling schedules
type ScheduleStreamingService struct {
	ctx         context.Context
	done        <-chan int
	client      *httpclient.HttpClient
	env         *env.Manager
	p2p         *external.ScheduleServiceFactory
	queryParams *schema.QueryParams
}

// NewScheduleService creates a new instance of ScheduleService
func NewScheduleStreamingService(
	ctx context.Context,
	done <-chan int,
	client *httpclient.HttpClient,
	env *env.Manager,
	p2p *external.ScheduleServiceFactory,
	queryParams *schema.QueryParams,
) *ScheduleStreamingService {
	return &ScheduleStreamingService{
		ctx:         ctx,
		done:        done,
		client:      client,
		env:         env,
		p2p:         p2p,
		queryParams: queryParams,
	}
}

func (sss *ScheduleStreamingService) GenerateScheduleChannels() []<-chan any {
	scacList := schema.CollectCarriers(sss.queryParams.SCAC)
	fanOutChannels := make([]<-chan any, 0, len(scacList))
	for _, scac := range scacList {
		p2pScheduleChan := sss.ConsolidateSchedule(scac)
		if sss.queryParams.TSP != nil || sss.queryParams.VesselIMO != nil || sss.queryParams.Service != nil || sss.queryParams.DirectOnly != nil {
			filterSchedule := sss.FilterSchedule(p2pScheduleChan)
			fanOutChannels = append(fanOutChannels, sss.ValidateSchedules(filterSchedule))
		} else {
			fanOutChannels = append(fanOutChannels, sss.ValidateSchedules(p2pScheduleChan))
		}
	}
	return fanOutChannels

}

// FetchCarrierSchedule fetches schedule for a specific carrier
func (sss *ScheduleStreamingService) FetchCarrierSchedule(scac schema.CarrierCode) []*schema.Schedule {
	if sss.ctx.Err() != nil {
		log.Infof("Context canceled before fetching schedule for %s", scac)
		return nil
	}
	service, err := sss.p2p.CreateScheduleService(scac)
	if err != nil {
		log.Errorf("Failed to create schedule service: %w", err)
		return nil
	}
	schedules, _ := service.FetchSchedule(sss.ctx, sss.client, sss.env, sss.queryParams, scac)
	return schedules
}

// ConsolidateSchedule creates a channel for schedule consolidation
func (sss *ScheduleStreamingService) ConsolidateSchedule(scac schema.CarrierCode) <-chan []*schema.Schedule {
	stream := make(chan []*schema.Schedule)
	go func() {
		defer close(stream)
		select {
		case <-sss.done:
			return
		case <-sss.ctx.Done():
			return
		case stream <- sss.FetchCarrierSchedule(scac):

		}
	}()
	return stream
}

// Gather schedule and filter it based on the query
func (sss *ScheduleStreamingService) PostFilter(schedules []*schema.Schedule,
	filters func(schedule *schema.Schedule, query *schema.QueryParams) bool) iter.Seq[*schema.Schedule] {
	return func(yield func(*schema.Schedule) bool) {
		for _, schedule := range schedules {
			if filters(schedule, sss.queryParams) && !yield(schedule) {
				return
			}

		}
	}
}

func (sss *ScheduleStreamingService) FilterSchedule(stream <-chan []*schema.Schedule) <-chan []*schema.Schedule {
	out := make(chan []*schema.Schedule)

	go func() {
		defer close(out)
		for schedule := range stream {
			select {
			case <-sss.done:
				return
			case <-sss.ctx.Done():
				return
			case out <- slices.Collect(sss.PostFilter(schedule, ScheduleFilters)):
			}

		}
	}()
	return out

}

// ValidateSchedules validates the schedules and returns a channel
func (sss *ScheduleStreamingService) ValidateSchedules(stream <-chan []*schema.Schedule) <-chan any {
	out := make(chan any)
	go func() {
		defer close(out)
		for schedule := range stream {
			if sss.validSchedulesFn(schedule) {
				select {
				case <-sss.done:
					return
				case <-sss.ctx.Done():
					return
				case out <- schedule:
				}
			}
		}
	}()
	return out
}

// validSchedulesFn validates individual schedules
func (sss *ScheduleStreamingService) validSchedulesFn(schedules []*schema.Schedule) bool {
	for _, schedule := range schedules {
		if err := schema.ResponseValidate.Struct(schedule); err != nil {
			if validationErrors, ok := err.(validator.ValidationErrors); ok {
				log.Errorf("%+v\n", validationErrors.Error())
				return false
			}
		} else {
			return true
		}
	}
	return true
}

// FanIn combines multiple schedule channels into one
func (sss *ScheduleStreamingService) FanIn(channels ...<-chan any) <-chan any {
	var wg sync.WaitGroup
	fannedInStream := make(chan any)

	transfer := func(ch <-chan any) {
		defer wg.Done()
		for i := range ch {
			select {
			case <-sss.done:
				return
			case <-sss.ctx.Done():
				return
			case fannedInStream <- i:
			}
		}
	}
	//Transfer all those carrier schedule channel to fannedInStream. if parent main function cancel the done channel which means nothing more to be looped over, this function will be returned(closed)
	for _, c := range channels {
		wg.Add(1)
		go transfer(c)
	} //Spin a goroutine for each channel in order to process concurrently

	go func() {
		wg.Wait()
		close(fannedInStream)
	}() //Close waitgroup and channel

	return fannedInStream
}

// StreamResponse handles the streaming of response data
func (sss *ScheduleStreamingService) StreamResponse(w flushWriter, r *http.Request, fannedIn <-chan any) {
	w.Write([]byte(fmt.Sprintf(
		`{"origin":"%s","destination":"%s","schedules":[`, *sss.queryParams.PointFrom, *sss.queryParams.PointTo,
	)))
	w.Flush() // Flush data right away

	scheduleCount := 0
	first := true
	doneProcessing := make(chan int) // Signal when processing is done

	go func() {
		defer close(doneProcessing)
		for schedules := range fannedIn {
			select {
			case <-sss.done:
				return
			case <-sss.ctx.Done():
				return
			default:
				scheduleBatch, _ := schedules.([]*schema.Schedule)
				for _, schedule := range scheduleBatch {
					if !first {
						w.Write([]byte(","))
					}
					first = false
					scheduleJSON, _ := json.Marshal(schedule) // this need to be changed
					w.Write(scheduleJSON)
					w.Flush()
					scheduleCount++
				}
			}
		}
	}()

	<-doneProcessing // Block until goroutine finishes (ensures JSON is properly closed)
	if scheduleCount == 0 {
		w.Write([]byte(`],"message":"No available schedules for the requested route."}`))
	} else {
		w.Write([]byte(`]}`))
	}
	w.Flush()
}
