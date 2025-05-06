package carrier_vessel_schedule

import (
	"cmp"
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/external"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	"slices"
	"time"
)

type HapagVesselScheduleResponse []HapagCarrierService

type HapagCarrierService struct {
	CarrierServiceName        string                `json:"carrierServiceName"`
	CarrierServiceCode        string                `json:"carrierServiceCode"`
	UniversalServiceReference string                `json:"universalServiceReference"`
	VesselSchedules           []HapagVesselSchedule `json:"vesselSchedules"`
}

type HapagVesselSchedule struct {
	VesselOperatorSMDGLinerCode string               `json:"vesselOperatorSMDGLinerCode"`
	VesselIMONumber             string               `json:"vesselIMONumber"`
	VesselName                  string               `json:"vesselName"`
	VesselCallSign              string               `json:"vesselCallSign"`
	IsDummyVessel               bool                 `json:"isDummyVessel"`
	TransportCalls              []HapagTransportCall `json:"transportCalls"`
}

type HapagTransportCall struct {
	TransportCallReference    string           `json:"transportCallReference"`
	CarrierImportVoyageNumber string           `json:"carrierImportVoyageNumber"`
	CarrierExportVoyageNumber string           `json:"carrierExportVoyageNumber"`
	Location                  HapagLocation    `json:"location"`
	StatusCode                string           `json:"statusCode,omitempty"`
	Timestamps                []HapagTimestamp `json:"timestamps"`
}

type HapagLocation struct {
	LocationType     string `json:"locationType"`
	UNLocationCode   string `json:"UNLocationCode"`
	FacilitySMDGCode string `json:"facilitySMDGCode"`
}

type HapagTimestamp struct {
	EventTypeCode       string `json:"eventTypeCode"`
	EventClassifierCode string `json:"eventClassifierCode"`
	EventDateTime       string `json:"eventDateTime"`
	ChangeRemark        string `json:"changeRemark"`
}

//var hapagEventType = map[string]string{
//	"ARRI": "Unloading",
//	"DEPA": "Loading",
//	"PLN":  "Planned",
//}

var hapagVoyageDirection = map[string]string{
	"W": "WBO",
	"E": "EBO",
	"N": "NBO",
	"S": "SBO",
}

const hapagDateFormat string = "2006-01-02T15:04:05-07:00"

func sortAndRemoveDuplicates(portCalls []schema.PortCalls) []schema.PortCalls {
	var countPortCall int
	slices.SortFunc(portCalls, func(a, b schema.PortCalls) int {
		return cmp.Or(
			cmp.Compare(a.EstimatedEventDate, b.EstimatedEventDate),
		)
	})
	type uniqueKey struct {
		port      string
		eventDate string
	}
	seen := make(map[uniqueKey]int)
	var portCallsWithOutduplicates []schema.PortCalls

	for _, item := range portCalls {

		unique := uniqueKey{item.Port.PortCode, item.EstimatedEventDate}
		seen[unique]++
		if seen[unique] <= 1 {
			countPortCall += 1
			item.Seq = countPortCall
			portCallsWithOutduplicates = append(portCallsWithOutduplicates, item)
		}
	}

	return portCallsWithOutduplicates
}

func (hvs *HapagVesselScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs[*schema.QueryParamsForVesselVoyage]) interfaces.HeaderParams {
	const defaultDateRange = 60
	var calculateEndDate = func(startDate string, dateRange int) string {
		maxDateRange := slices.Max([]int{dateRange, defaultDateRange})
		date, _ := time.Parse("2006-01-02", startDate)
		endDate := date.AddDate(0, 0, maxDateRange)
		return endDate.Format("2006-01-02")
	}

	scheduleHeaders := map[string]string{
		"X-IBM-Client-Id":     *p.Env.HapagClient,
		"X-IBM-Client-Secret": *p.Env.HapagSecret,
		"Accept":              "application/json",
	}

	scheduleParams := map[string]string{
		"vesselIMONumber":     p.Query.VesselIMO,
		"carrierVoyageNumber": p.Query.Voyage,
		"startDate":           p.Query.StartDate,
		"endDate":             calculateEndDate(p.Query.StartDate, p.Query.DateRange),
	}
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams
}

func (hvs *HapagVesselScheduleResponse) GenerateSchedule(responseJson []byte) (*schema.MasterVesselSchedule, error) {
	var hapagVesselScheduleResponse HapagVesselScheduleResponse
	err := json.Unmarshal(responseJson, &hapagVesselScheduleResponse)
	if len(hapagVesselScheduleResponse) == 0 {
		return nil, fmt.Errorf("hapag vessel schedule response is empty")
	}
	if err != nil {
		return nil, err
	}
	mvsResult := &schema.MasterVesselSchedule{
		Scac: "HLCU",
		Voyage: cmp.Or(
			hapagVesselScheduleResponse[0].VesselSchedules[0].TransportCalls[0].CarrierImportVoyageNumber,
			hapagVesselScheduleResponse[0].VesselSchedules[0].TransportCalls[0].CarrierExportVoyageNumber),
		Vessel: schema.VesselDetails{
			VesselName: hapagVesselScheduleResponse[0].VesselSchedules[0].VesselName,
			Imo:        hapagVesselScheduleResponse[0].VesselSchedules[0].VesselIMONumber},
		Services: schema.Services{
			ServiceCode: hapagVesselScheduleResponse[0].CarrierServiceCode,
			ServiceName: hapagVesselScheduleResponse[0].CarrierServiceName},
		Calls: hvs.GenerateVesselCalls(hapagVesselScheduleResponse),
	}
	return mvsResult, nil
}

func (hvs *HapagVesselScheduleResponse) GenerateVesselCalls(vesselSchedules HapagVesselScheduleResponse) []schema.PortCalls {
	var hapagPortCalls = make([]schema.PortCalls, 0, len(vesselSchedules))
	type KeyValueForPortEvent struct {
		eventType         string
		eventVoyageNumber string
	}
	for _, vesselSchedule := range vesselSchedules {
		for _, schedule := range vesselSchedule.VesselSchedules {
			for _, portCalls := range schedule.TransportCalls {
				var getEstimatedEventDate = func(eventType string) string {
					for _, eventDates := range portCalls.Timestamps {
						switch true {
						case eventDates.EventTypeCode == "ARRI" && eventDates.EventClassifierCode == "EST" && eventType == "Unloading":
							return external.ConvertDateFormat(&eventDates.EventDateTime, hapagDateFormat)
						case eventDates.EventTypeCode == "ARRI" && eventDates.EventClassifierCode == "PLN" && eventType == "Unloading":
							return external.ConvertDateFormat(&eventDates.EventDateTime, hapagDateFormat)
						case eventDates.EventTypeCode == "DEPA" && eventDates.EventClassifierCode == "EST" && eventType == "Loading":
							return external.ConvertDateFormat(&eventDates.EventDateTime, hapagDateFormat)
						case eventDates.EventTypeCode == "DEPA" && eventDates.EventClassifierCode == "PLN" && eventType == "Loading":
							return external.ConvertDateFormat(&eventDates.EventDateTime, hapagDateFormat)
						}
					}
					return ""
				}
				var getActualEventDate = func(eventType string) string {
					for _, eventDates := range portCalls.Timestamps {
						switch true {
						case eventDates.EventTypeCode == "ARRI" && eventDates.EventClassifierCode == "ACT" && eventType == "Unloading":
							return external.ConvertDateFormat(&eventDates.EventDateTime, hapagDateFormat)
						case eventDates.EventTypeCode == "DEPA" && eventDates.EventClassifierCode == "ACT" && eventType == "Loading":
							return external.ConvertDateFormat(&eventDates.EventDateTime, hapagDateFormat)
						}
					}
					return ""
				}

				portEvents := []KeyValueForPortEvent{
					{"Unloading", portCalls.CarrierImportVoyageNumber},
					{"Loading", portCalls.CarrierExportVoyageNumber},
				}
				for i, pe := range portEvents {
					if pe.eventVoyageNumber != "" {
						portCallsResult := schema.PortCalls{
							Seq:       i,
							Key:       portCalls.TransportCallReference,
							Bound:     cmp.Or(hapagVoyageDirection[pe.eventVoyageNumber[len(pe.eventVoyageNumber)-1:]], "UNK"),
							Voyage:    pe.eventVoyageNumber,
							PortEvent: pe.eventType,
							Service:   schema.Services{ServiceCode: vesselSchedule.CarrierServiceCode, ServiceName: vesselSchedule.CarrierServiceName},
							Port: schema.Port{
								PortCode:     portCalls.Location.UNLocationCode,
								TerminalCode: portCalls.Location.FacilitySMDGCode,
							},
							EstimatedEventDate: getEstimatedEventDate(pe.eventType),
							ActualEventDate:    getActualEventDate(pe.eventType),
						}
						hapagPortCalls = append(hapagPortCalls, portCallsResult)
					}
				}

			}
		}
	}
	finalResult := sortAndRemoveDuplicates(hapagPortCalls)
	return finalResult
}
