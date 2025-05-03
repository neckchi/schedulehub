package carrier_vessel_schedule

import (
	"cmp"
	"encoding/json"
	"github.com/neckchi/schedulehub/external"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	"time"
)

type CMAVesselScheduleResponse []CMAVesselSchedules

type CMAVesselSchedules struct {
	// Basic voyage information
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

// LocationCodification represents a codification entry for a location
type CMALocationCodification struct {
	CodificationType string `json:"codificationType"`
	Codification     string `json:"codification"`
}

// FacilityCodification represents a codification entry for a facility
type CMAFacilityCodification struct {
	CodificationType string `json:"codificationType"`
	Codification     string `json:"codification"`
}

// Facility represents facility details within a location
type CMAFacility struct {
	Name                  string                    `json:"name"`
	FacilityType          string                    `json:"facilityType"`
	InternalCode          string                    `json:"internalCode"`
	FacilityCodifications []CMAFacilityCodification `json:"facilityCodifications"`
}

// Location represents location details
type CMALocation struct {
	Name                  string                    `json:"name"`
	InternalCode          string                    `json:"internalCode"`
	TimeZone              string                    `json:"timeZone"`
	LocationCodifications []CMALocationCodification `json:"locationCodifications"`
	Facility              CMAFacility               `json:"facility"`
}

// Point represents point information
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

func (cvs *CMAVesselScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs[*schema.QueryParamsForVesselVoyage]) interfaces.HeaderParams {
	var calculateEndDate = func(startDate string, dateRange int) string {
		date, _ := time.Parse("2006-01-02", startDate)
		endDate := date.AddDate(0, 0, dateRange)
		return endDate.Format("2006-01-02")
	}

	scheduleHeaders := map[string]string{
		"KeyId": *p.Env.CmaToken,
	}
	scheduleParams := map[string]string{
		"shipcomp":  schema.InternalCodeMapping[p.Scac],
		"vesselIMO": p.Query.VesselIMO,
		"from":      p.Query.StartDate,
		"to":        calculateEndDate(p.Query.StartDate, p.Query.DateRange),
	}
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams
}

func (cvs *CMAVesselScheduleResponse) GenerateSchedule(responseJson []byte) (*schema.MasterVesselSchedule, error) {
	var cmaVesselSchedules CMAVesselScheduleResponse
	err := json.Unmarshal(responseJson, &cmaVesselSchedules)
	if err != nil {
		return nil, err
	}
	mvsResult := &schema.MasterVesselSchedule{
		Scac:       string(schema.InternalCodeToScac[cmaVesselSchedules[0].ShippingCompany]),
		Voyage:     cmaVesselSchedules[0].VoyageCode,
		NextVoyage: cmaVesselSchedules[0].NextVoyage,
		Vessel:     schema.VesselDetails{VesselName: cmaVesselSchedules[0].Vessel.Name, Imo: cmaVesselSchedules[0].Vessel.Imo},
		Services:   schema.Services{ServiceCode: cmaVesselSchedules[0].Service.Code, ServiceName: cmaVesselSchedules[0].Service.Name},
		Calls:      cvs.GenerateVesselCalls(cmaVesselSchedules),
	}

	return mvsResult, nil
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

func (cvs *CMAVesselScheduleResponse) GenerateVesselCalls(vesselCalls CMAVesselScheduleResponse) []schema.PortCalls {
	var countPortCall int
	var cmaPortCalls = make([]schema.PortCalls, 0, len(vesselCalls))

	for _, portCalls := range vesselCalls {
		var getEventDateTime = func(eventType string) string {
			if eventType == "Load" {
				return external.ConvertDateFormat(&portCalls.BerthDate.Utc, cmaDateFormat)
			}
			return external.ConvertDateFormat(&portCalls.UnberthDate.Utc, cmaDateFormat)
		}
		for _, activity := range portCalls.Activities {
			countPortCall += 1
			portCallsResult := schema.PortCalls{
				Seq:       countPortCall,
				Key:       portCalls.ID,
				Bound:     cmp.Or(cmaDirectionMapping[portCalls.Bound], "UNK"),
				Voyage:    portCalls.VoyageCode,
				PortEvent: cmaEventType[activity],
				Service:   schema.Services{ServiceCode: portCalls.Service.Code, ServiceName: portCalls.Service.Name},
				Port: schema.Port{
					PortCode: cmp.Or(portCalls.Location.InternalCode, portCalls.Location.LocationCodifications[0].Codification),
					PortName: portCalls.Location.Name,
				},
				EstimateDate: getEventDateTime(activity),
			}
			cmaPortCalls = append(cmaPortCalls, portCallsResult)
		}
	}
	return cmaPortCalls
}
