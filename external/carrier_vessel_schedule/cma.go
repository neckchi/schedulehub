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

type CMAVesselScheduleResponse []CMAVesselSchedules

type CMAVesselSchedules struct {
	ID                              string      `json:"id"`
	Type                            string      `json:"type"`
	Activities                      []string    `json:"activities"`
	VoyageCode                      string      `json:"voyageCode"`
	Bound                           string      `json:"bound"`
	ShippingCompany                 string      `json:"shippingCompany"`
	Location                        CMALocation `json:"location"`
	Point                           CMAPoint    `json:"point"`
	Vessel                          CMAVessel   `json:"vessel"`
	Terminal                        CMATerminal `json:"terminal"`
	Service                         CMAService  `json:"service"`
	BerthDate                       CMADateTime `json:"berthDate"`
	UnberthDate                     CMADateTime `json:"unberthDate"`
	EospDate                        CMADateTime `json:"eospDate"`
	SospDate                        CMADateTime `json:"sospDate"`
	PortCutoff                      CMADateTime `json:"portCutoff"`
	VgmCutoff                       CMADateTime `json:"vgmCutoff"`
	StandardBookingAcceptanceCutoff CMADateTime `json:"standardBookingAcceptanceCutoff"`
	SpecialBookingAcceptanceCutoff  CMADateTime `json:"specialBookingAcceptanceCutoff"`
	ShippingInstructionCutoff       CMADateTime `json:"shippingInstructionCutoff"`
	ShippingInstructionFilingCutoff CMADateTime `json:"shippingInstructionFilingCutoff"`
	CustomsCutoff                   CMADateTime `json:"customsCutoff"`
	AdvanceFilingCutoff             CMADateTime `json:"advanceFilingCutoff"`
	EarliestReceivingDate           CMADateTime `json:"earliestReceivingDate,omitempty"`
	PreviousVoyage                  string      `json:"previousVoyage"`
	NextVoyage                      string      `json:"nextVoyage,omitempty"`
}

type CMALocationCodification struct {
	CodificationType string `json:"codificationType"`
	Codification     string `json:"codification"`
}

type CMAFacilityCodification struct {
	CodificationType string `json:"codificationType"`
	Codification     string `json:"codification"`
}

type CMAFacility struct {
	Name                  string                    `json:"name"`
	FacilityType          string                    `json:"facilityType"`
	InternalCode          string                    `json:"internalCode"`
	FacilityCodifications []CMAFacilityCodification `json:"facilityCodifications"`
}

type CMALocation struct {
	Name                  string                    `json:"name"`
	InternalCode          string                    `json:"internalCode"`
	TimeZone              string                    `json:"timeZone"`
	LocationCodifications []CMALocationCodification `json:"locationCodifications"`
	Facility              CMAFacility               `json:"facility"`
}

type CMAPoint struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Vessel represents vessel details
type CMAVessel struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Imo           string `json:"imo"`
	SmdgLinerCode string `json:"smdgLinerCode"`
}

// Terminal represents terminal information
type CMATerminal struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Service represents service details
type CMAService struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	ExternalCode string `json:"externalCode"`
}

// DateTime represents a local and UTC timestamp pair
type CMADateTime struct {
	Local string `json:"local"`
	Utc   string `json:"utc"`
}

var cmaEventType = map[string]string{
	"Load":      "Loading",
	"Discharge": "Unloading",
}

const cmaDateFormat string = "2006-01-02T15:04:05Z"

var cmaDirectionMapping = map[string]string{
	"WEST":  "WBO",
	"EAST":  "EBO",
	"NORTH": "NBO",
	"SOUTH": "SBO",
}

func (cvs *CMAVesselScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs[*schema.QueryParamsForVesselVoyage]) interfaces.HeaderParams {
	const defaultDateRange = 60
	var calculateEndDate = func(startDate string, dateRange int) string {
		maxDateRange := slices.Max([]int{dateRange, defaultDateRange})
		date, _ := time.Parse("2006-01-02", startDate)
		endDate := date.AddDate(0, 0, maxDateRange)
		return endDate.Format("2006-01-02")
	}

	scheduleHeaders := map[string]string{
		"KeyId": *p.Env.CmaToken,
	}
	scheduleParams := map[string]string{
		"shipcomp":  schema.InternalCodeMapping[p.Scac],
		"vesselIMO": p.Query.VesselIMO,
	}

	if p.Query.Voyage != "" {
		scheduleParams["voyageCode"] = p.Query.Voyage
	}

	if p.Query.StartDate != "" {
		scheduleParams["from"] = p.Query.StartDate
		scheduleParams["to"] = calculateEndDate(p.Query.StartDate, p.Query.DateRange)
	}

	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams
}

func (cvs *CMAVesselScheduleResponse) GenerateSchedule(responseJson []byte) (*schema.MasterVesselSchedule, error) {
	var cmaVesselScheduleResponse CMAVesselScheduleResponse
	err := json.Unmarshal(responseJson, &cmaVesselScheduleResponse)
	if len(cmaVesselScheduleResponse) == 0 {
		return nil, fmt.Errorf("CMA Vessel Schedule Response is empty")
	}
	if err != nil {
		return nil, err
	}
	mvsResult := &schema.MasterVesselSchedule{
		Scac:       string(schema.InternalCodeToScac[cmaVesselScheduleResponse[0].ShippingCompany]),
		Voyage:     cmaVesselScheduleResponse[0].VoyageCode,
		NextVoyage: cmaVesselScheduleResponse[0].NextVoyage,
		Vessel:     &schema.VesselDetails{VesselName: cmaVesselScheduleResponse[0].Vessel.Name, Imo: cmaVesselScheduleResponse[0].Vessel.Imo},
		Services:   &schema.Services{ServiceCode: cmaVesselScheduleResponse[0].Service.Code, ServiceName: cmaVesselScheduleResponse[0].Service.Name},
		Calls:      cvs.GenerateVesselCalls(cmaVesselScheduleResponse),
	}

	return mvsResult, nil
}

func (cvs *CMAVesselScheduleResponse) GenerateVesselCalls(vesselCalls CMAVesselScheduleResponse) []schema.PortCalls {
	var countPortCall int
	var cmaPortCalls = make([]schema.PortCalls, 0, len(vesselCalls))

	for _, portCalls := range vesselCalls {
		var getEventDateTime = func(eventType string) string {
			if eventType == "Load" {
				return external.ConvertDateFormat(&portCalls.UnberthDate.Utc, cmaDateFormat)
			}
			return external.ConvertDateFormat(&portCalls.BerthDate.Utc, cmaDateFormat)
		}
		for _, activity := range slices.Backward(portCalls.Activities) {
			countPortCall += 1
			portCallsResult := schema.PortCalls{
				Seq:       countPortCall,
				Key:       portCalls.ID,
				Bound:     cmp.Or(cmaDirectionMapping[portCalls.Bound], "UNK"),
				Voyage:    portCalls.VoyageCode,
				PortEvent: cmaEventType[activity],
				Service:   &schema.Services{ServiceCode: portCalls.Service.Code, ServiceName: portCalls.Service.Name},
				Port: &schema.Port{
					PortCode:     cmp.Or(portCalls.Location.InternalCode, portCalls.Location.LocationCodifications[0].Codification),
					PortName:     portCalls.Location.Name,
					TerminalName: portCalls.Location.Facility.Name,
					TerminalCode: cmp.Or(portCalls.Location.Facility.FacilityCodifications[0].Codification, portCalls.Location.Facility.InternalCode),
				},
				EstimatedEventDate: getEventDateTime(activity),
			}
			cmaPortCalls = append(cmaPortCalls, portCallsResult)
		}
	}
	return cmaPortCalls
}
