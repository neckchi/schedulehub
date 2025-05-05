package p2p_schedule_handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/neckchi/schedulehub/external/carrier_p2p_schedule"
	httpclient "github.com/neckchi/schedulehub/internal/http"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	"github.com/neckchi/schedulehub/internal/utils"
	log "github.com/sirupsen/logrus"
	"iter"
	"slices"
	"sync"
)

// ScheduleService encapsulates the dependencies and methods for handling schedules
type ScheduleStreamingService struct {
	ctx         context.Context
	done        <-chan int
	client      *httpclient.HttpClient
	env         *env.Manager
	p2p         *carrier_p2p_schedule.P2PScheduleServiceFactory
	queryParams *schema.QueryParams
}

// NewScheduleService creates a new instance of ScheduleService
func NewScheduleStreamingService(
	ctx context.Context,
	done <-chan int,
	client *httpclient.HttpClient,
	env *env.Manager,
	p2p *carrier_p2p_schedule.P2PScheduleServiceFactory,
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

func (sss *ScheduleStreamingService) FanOutScheduleChannels() []<-chan []*schema.P2PSchedule {
	fanOutChannels := make([]<-chan []*schema.P2PSchedule, 0, len(sss.queryParams.SCAC))
	compositeFilter := ScheduleFilters(WithDirectOnly(), WithTSP(), WithVesselIMO(), WithService())
	for _, scac := range sss.queryParams.SCAC {
		p2pScheduleChan := sss.ConsolidateSchedule(scac)
		if sss.queryParams.TSP != "" || sss.queryParams.VesselIMO != "" || sss.queryParams.Service != "" || sss.queryParams.DirectOnly {
			filterSchedule := sss.FilterSchedule(p2pScheduleChan, compositeFilter)
			fanOutChannels = append(fanOutChannels, sss.ValidateSchedules(filterSchedule))
		} else {
			fanOutChannels = append(fanOutChannels, sss.ValidateSchedules(p2pScheduleChan))
		}
	}
	return fanOutChannels

}

// ConsolidateSchedule creates a channel for schedule consolidation
func (sss *ScheduleStreamingService) ConsolidateSchedule(scac schema.CarrierCode) <-chan []*schema.P2PSchedule {
	stream := make(chan []*schema.P2PSchedule)
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

// FetchCarrierSchedule fetches schedule for a specific carrier
func (sss *ScheduleStreamingService) FetchCarrierSchedule(scac schema.CarrierCode) []*schema.P2PSchedule {
	if sss.ctx.Err() != nil {
		log.Infof("Context canceled before fetching schedule for %s", scac)
		return nil
	}
	service, err := sss.p2p.CreateScheduleService(scac)
	if err != nil {
		log.Errorf("Failed to create schedule service: %s", err)
		return nil
	}
	schedules, _ := service.FetchSchedule(sss.ctx, sss.client, sss.env, sss.queryParams, scac)
	return schedules
}

func (sss *ScheduleStreamingService) PostFilter(schedules []*schema.P2PSchedule, filter ScheduleFilterOption) iter.Seq[*schema.P2PSchedule] {
	return func(yield func(*schema.P2PSchedule) bool) {
		for _, schedule := range schedules {
			if filter(schedule, sss.queryParams) && !yield(schedule) {
				return
			}
		}
	}
}

func (sss *ScheduleStreamingService) FilterSchedule(stream <-chan []*schema.P2PSchedule, filter ScheduleFilterOption) <-chan []*schema.P2PSchedule {
	out := make(chan []*schema.P2PSchedule)

	go func() {
		defer close(out)
		for schedules := range stream {
			select {
			case <-sss.done:
				return
			case <-sss.ctx.Done():
				return
			case out <- slices.Collect(sss.PostFilter(schedules, filter)):
			}
		}
	}()
	return out
}

// ValidateSchedules validates the schedules and returns a channel
func (sss *ScheduleStreamingService) ValidateSchedules(stream <-chan []*schema.P2PSchedule) <-chan []*schema.P2PSchedule {
	out := make(chan []*schema.P2PSchedule)
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
func (sss *ScheduleStreamingService) validSchedulesFn(schedules []*schema.P2PSchedule) bool {
	for _, schedule := range schedules {
		if err := schema.P2PResponseValidate.Struct(schedule); err != nil {
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
func (sss *ScheduleStreamingService) FanIn(channels ...<-chan []*schema.P2PSchedule) <-chan []*schema.P2PSchedule {
	var wg sync.WaitGroup
	fannedInStream := make(chan []*schema.P2PSchedule)

	transfer := func(ch <-chan []*schema.P2PSchedule) {
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
func (sss *ScheduleStreamingService) StreamResponse(w utils.FlushWriter, fannedIn <-chan []*schema.P2PSchedule) {
	_, _ = w.Write([]byte(fmt.Sprintf(
		`{"origin":"%s","destination":"%s","schedules":[`, sss.queryParams.PointFrom, sss.queryParams.PointTo,
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
				for _, schedule := range schedules {
					if !first {
						_, _ = w.Write([]byte(","))
					}
					first = false
					scheduleJSON, _ := json.Marshal(schedule) // this need to be changed
					_, _ = w.Write(scheduleJSON)
					w.Flush()
					scheduleCount++
				}
			}
		}
	}()

	<-doneProcessing // Block until goroutine finishes (ensures JSON is properly closed)
	if scheduleCount == 0 {
		_, _ = w.Write([]byte(`],"message":"No available schedules for the requested route."}`))
	} else {
		_, _ = w.Write([]byte(`]}`))
	}
	w.Flush()
}
