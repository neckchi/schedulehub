package mvs_handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/neckchi/schedulehub/external/carrier_vessel_schedule"
	"github.com/neckchi/schedulehub/internal/database"
	httpclient "github.com/neckchi/schedulehub/internal/http"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	"github.com/neckchi/schedulehub/internal/utils"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type MasterVesselSchedule struct {
	ctx            context.Context
	db             database.OracleRepository
	done           <-chan int
	client         *httpclient.HttpClient
	env            *env.Manager
	vv             *carrier_vessel_schedule.VesselScheduleServiceFactory
	queryParams    *schema.QueryParamsForVesselVoyage
	scheduleConfig map[string]interface{}
}

// NewScheduleService creates a new instance of ScheduleService
func NewMastervVesselVoyageService(
	ctx context.Context,
	db database.OracleRepository,
	done <-chan int,
	client *httpclient.HttpClient,
	env *env.Manager,
	vv *carrier_vessel_schedule.VesselScheduleServiceFactory,
	queryParams *schema.QueryParamsForVesselVoyage,
	scheduleConfig map[string]any) *MasterVesselSchedule {
	return &MasterVesselSchedule{
		ctx:            ctx,
		db:             db,
		done:           done,
		client:         client,
		env:            env,
		vv:             vv,
		queryParams:    queryParams,
		scheduleConfig: scheduleConfig,
	}
}

func (mvs *MasterVesselSchedule) FanOutMVSChannels() []<-chan *schema.MasterVesselSchedule {
	fanOutChannels := make([]<-chan *schema.MasterVesselSchedule, 0, len(mvs.queryParams.SCAC))
	for _, scac := range mvs.queryParams.SCAC {
		mvsChan := mvs.ConsolidateMasterVesselSchedule(scac)
		fanOutChannels = append(fanOutChannels, mvs.ValidateMasterVesselSchedules(mvsChan))
	}
	return fanOutChannels
}

// ConsolidateSchedule creates a channel for schedule consolidation
func (mvs *MasterVesselSchedule) ConsolidateMasterVesselSchedule(scac schema.CarrierCode) <-chan *schema.MasterVesselSchedule {
	stream := make(chan *schema.MasterVesselSchedule)
	go func() {
		defer close(stream)
		select {
		case <-mvs.ctx.Done():
			return
		case <-mvs.done:
			return
		case stream <- mvs.FetchMasterVesselSchedule(scac):
		}
	}()
	return stream
}

func (mvs *MasterVesselSchedule) FetchMasterVesselSchedule(scac schema.CarrierCode) *schema.MasterVesselSchedule {
	if active, ok := mvs.scheduleConfig[string(scac)].(bool); active && ok {
		service, err := mvs.vv.CreateVesselScheduleService(scac)
		if err != nil {
			log.Errorf("Failed to create schedule service: %s", err)
			return nil
		}
		masterVesselSchedule, _ := service.FetchSchedule(context.Background(), mvs.client, mvs.env, mvs.queryParams, scac)
		return masterVesselSchedule
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	sqlResults, err := mvs.db.QueryContext(ctx, scac, *mvs.queryParams)
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
func (mvs *MasterVesselSchedule) ValidateMasterVesselSchedules(stream <-chan *schema.MasterVesselSchedule) <-chan *schema.MasterVesselSchedule {
	out := make(chan *schema.MasterVesselSchedule)
	go func() {
		defer close(out)
		for schedule := range stream {
			if mvs.validMasterVesselSchedulesFn(schedule) {
				select {
				case <-mvs.ctx.Done():
					return
				case <-mvs.done:
					return
				case out <- schedule:
				}
			}
		}
	}()
	return out
}

func (mvs *MasterVesselSchedule) validMasterVesselSchedulesFn(schedules *schema.MasterVesselSchedule) bool {
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
func (mvs *MasterVesselSchedule) FanInMasterVesselSchedule(channels ...<-chan *schema.MasterVesselSchedule) <-chan *schema.MasterVesselSchedule {
	var wg sync.WaitGroup
	fannedInStream := make(chan *schema.MasterVesselSchedule)

	transfer := func(ch <-chan *schema.MasterVesselSchedule) {
		defer wg.Done()
		for i := range ch {
			select {
			case <-mvs.ctx.Done():
				return
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
func (mvs *MasterVesselSchedule) StreamMasterVesselSchedule(w utils.FlushWriter, fannedIn <-chan *schema.MasterVesselSchedule) {
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
			case <-mvs.ctx.Done():
				return
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

type groupKey struct {
	PortEvent string
	PortCode  string
	EventTime string
}

func findOverlappedPorts(sqlResults []schema.ScheduleRow) map[groupKey]bool {
	counts := make(map[groupKey]int, len(sqlResults))
	for _, item := range sqlResults {
		key := groupKey{
			PortEvent: item.PortEvent,
			PortCode:  item.PortCode,
			EventTime: item.EventTime,
		}
		counts[key]++
	}
	overlappedPorts := make(map[groupKey]bool, len(counts)/2)
	for key, count := range counts {
		if count > 1 {
			overlappedPorts[key] = true
		}
	}
	return overlappedPorts
}

func getUniqueData(sqlResults []schema.ScheduleRow, overlappedPorts map[groupKey]bool) ([]string, []string, []string) {
	uniqueVoyageNumbers := make([]string, 0, 2)
	uniqueBounds := make([]string, 0, 2)
	uniqueKeys := make([]string, 0, 2)

	voyageSet := make(map[string]bool)
	boundsSet := make(map[string]bool)
	voyageKeySet := make(map[string]bool)

	currentVoyage := sqlResults[0].VoyageNum

	for _, result := range sqlResults {
		// Collect unique voyage numbers
		if result.VoyageNum != currentVoyage && !voyageSet[result.VoyageNum] {
			uniqueVoyageNumbers = append(uniqueVoyageNumbers, result.VoyageNum)
			voyageSet[result.VoyageNum] = true
		}

		// Collect unique bounds and keys if there are overlapping ports
		if len(overlappedPorts) > 0 {
			if !boundsSet[result.VoyageDirection] {
				uniqueBounds = append(uniqueBounds, result.VoyageDirection)
				boundsSet[result.VoyageDirection] = true
			}
			if !voyageKeySet[result.ProvideVoyageID] {
				uniqueKeys = append(uniqueKeys, result.ProvideVoyageID)
				voyageKeySet[result.ProvideVoyageID] = true
			}
		}
	}

	return uniqueVoyageNumbers, uniqueBounds, uniqueKeys
}

func constructPortCalls(sqlResults []schema.ScheduleRow, overlappedPorts map[groupKey]bool, uniqueBounds, uniqueKeys, uniqueVoyageNumbers []string) []schema.PortCalls {
	var portOfCalls []schema.PortCalls
	currentVoyage := sqlResults[0].VoyageNum
	for _, port := range sqlResults {
		key := groupKey{port.PortEvent, port.PortCode, port.EventTime}
		var boundValue any
		var uniqueKeyVals any
		var voyageValue any
		if (overlappedPorts)[key] {
			boundValue = uniqueBounds
			uniqueKeyVals = uniqueKeys
			if len(uniqueVoyageNumbers) > 0 {
				voyageValue = []string{currentVoyage, uniqueVoyageNumbers[0]}
			} else {
				voyageValue = []string{currentVoyage}
			}
		} else {
			boundValue = port.VoyageDirection
			uniqueKeyVals = port.ProvideVoyageID
			voyageValue = port.VoyageNum
		}
		portCall := schema.PortCalls{
			Key:          uniqueKeyVals,
			Bound:        boundValue,
			Voyage:       voyageValue,
			Service:      schema.Services{ServiceCode: sqlResults[0].ServiceCode},
			PortEvent:    schema.EventType[port.PortEvent],
			Port:         schema.Port{PortName: port.PortName, PortCode: port.PortCode},
			EstimateDate: port.EventTime,
		}
		portOfCalls = append(portOfCalls, portCall)
	}
	return portOfCalls
}

func removeDuplicates(portOfCalls []schema.PortCalls, overlappedPorts map[groupKey]bool) []schema.PortCalls {
	var finalCalls []schema.PortCalls
	if len(overlappedPorts) > 0 {
		seen := make(map[string]bool)
		for _, call := range portOfCalls {
			callBytes, _ := json.Marshal(call)
			callStr := string(callBytes)
			if !seen[callStr] {
				seen[callStr] = true
				finalCalls = append(finalCalls, call)
			}
		}
	} else {
		finalCalls = portOfCalls
	}
	for i := range finalCalls {
		finalCalls[i].Seq = i + 1
	}
	return finalCalls
}

func constructAPIResult(sqlResults []schema.ScheduleRow, finalCalls []schema.PortCalls, uniqueVoyageNumbers []string) *schema.MasterVesselSchedule {

	var nextVoyage string
	if len(uniqueVoyageNumbers) > 0 {
		nextVoyage = uniqueVoyageNumbers[0]
	}
	return &schema.MasterVesselSchedule{
		Scac:       sqlResults[0].SCAC,
		Voyage:     sqlResults[0].VoyageNum,
		NextVoyage: nextVoyage,
		Vessel: schema.VesselDetails{
			VesselName: sqlResults[0].VesselName,
			Imo:        sqlResults[0].VesselIMO,
		},
		Services: schema.Services{
			ServiceCode: sqlResults[0].ServiceCode,
		},
		Calls: finalCalls,
	}
}
