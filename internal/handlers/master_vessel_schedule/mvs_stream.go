package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/neckchi/schedulehub/internal/database"
	"github.com/neckchi/schedulehub/internal/schema"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type MasterVesselSchedule struct {
	db          database.OracleRepository
	done        <-chan int
	queryParams schema.QueryParamsForVesselVoyage
}

// NewScheduleService creates a new instance of ScheduleService
func NewMastervVesselVoyageService(db database.OracleRepository, done <-chan int, queryParams schema.QueryParamsForVesselVoyage) *MasterVesselSchedule {
	return &MasterVesselSchedule{
		db:          db,
		done:        done,
		queryParams: queryParams,
	}
}

func (mvs *MasterVesselSchedule) FanOutMVSChannels() []<-chan *schema.MasterVoyage {
	fanOutChannels := make([]<-chan *schema.MasterVoyage, 0, len(mvs.queryParams.SCAC))
	for _, scac := range mvs.queryParams.SCAC {
		mvsChan := mvs.ConsolidateMasterVesselSchedule(scac)
		fanOutChannels = append(fanOutChannels, mvs.ValidateMasterVesselSchedules(mvsChan))
	}
	return fanOutChannels
}

// ConsolidateSchedule creates a channel for schedule consolidation
func (mvs *MasterVesselSchedule) ConsolidateMasterVesselSchedule(scac schema.CarrierCode) <-chan *schema.MasterVoyage {
	stream := make(chan *schema.MasterVoyage)
	go func() {
		defer close(stream)
		select {
		case <-mvs.done:
			return
		case stream <- mvs.FetchMasterVesselSchedule(scac):
		}
	}()
	return stream
}

func (mvs *MasterVesselSchedule) FetchMasterVesselSchedule(scac schema.CarrierCode) *schema.MasterVoyage {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	sqlResults, err := mvs.db.QueryContext(ctx, scac, mvs.queryParams)
	if err != nil {
		log.Error(err)
		return nil
	}
	overlappedPorts := findOverlappedPorts(sqlResults)
	uniqueVoyageNumbers, uniqueBounds, uniqueKeys := getUniqueData(sqlResults, overlappedPorts)
	portOfCalls := constructPortCalls(sqlResults, overlappedPorts, uniqueBounds, uniqueKeys, uniqueVoyageNumbers)
	finalCalls := removeDuplicates(portOfCalls, overlappedPorts)
	apiResult := constructAPIResult(sqlResults, finalCalls, uniqueVoyageNumbers)
	return apiResult
}

// ValidateSchedules validates the schedules and returns a channel
func (mvs *MasterVesselSchedule) ValidateMasterVesselSchedules(stream <-chan *schema.MasterVoyage) <-chan *schema.MasterVoyage {
	out := make(chan *schema.MasterVoyage)
	go func() {
		defer close(out)
		for schedule := range stream {
			if mvs.validMasterVesselSchedulesFn(schedule) {
				select {
				case <-mvs.done:
					return
				case out <- schedule:
				}
			}
		}
	}()
	return out
}

func (mvs *MasterVesselSchedule) validMasterVesselSchedulesFn(schedules *schema.MasterVoyage) bool {
	if err := schema.MVSResponseValidate.Struct(schedules); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			log.Errorf("%+v\n", validationErrors.Error())
			return false
		}
		return false
	}
	return true
}

// FanIn combines multiple schedule channels into one
func (mvs *MasterVesselSchedule) FanInMasterVesselSchedule(channels ...<-chan *schema.MasterVoyage) <-chan *schema.MasterVoyage {
	var wg sync.WaitGroup
	fannedInStream := make(chan *schema.MasterVoyage)

	transfer := func(ch <-chan *schema.MasterVoyage) {
		defer wg.Done()
		for i := range ch {
			select {
			case <-mvs.done:
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
func (mvs *MasterVesselSchedule) StreamMasterVesselSchedule(w flushWriter, fannedIn <-chan *schema.MasterVoyage) {
	_, _ = w.Write([]byte(fmt.Sprintf(
		`{"vesselIMO":"%s","vesselSchedules":[`, mvs.queryParams.VesselIMO,
	)))
	w.Flush() // Flush data right away

	scheduleCount := 0
	first := true
	doneProcessing := make(chan int) // Signal when processing is done
	go func() {
		defer close(doneProcessing)
		for schedules := range fannedIn {
			select {
			case <-mvs.done:
				return
			default:
				if !first {
					_, _ = w.Write([]byte(","))
				}
				first = false
				scheduleJSON, _ := json.Marshal(schedules) // this need to be changed
				_, _ = w.Write(scheduleJSON)
				w.Flush()
				scheduleCount++
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
