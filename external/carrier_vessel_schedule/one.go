package carrier_vessel_schedule

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/neckchi/schedulehub/external"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
)

type OneVesselSchedule struct {
	Vessel []OneVessel `json:"vessel,required"`
}

type OneVessel struct {
	Scac                   string    `json:"scac"`
	CarrierName            string    `json:"carrierName"`
	ServiceCode            string    `json:"serviceCode"`
	ServiceNameArrival     string    `json:"serviceNameArrival"`
	ServiceNameDeparture   string    `json:"serviceNameDeparture"`
	TerminalCutoff         string    `json:"terminalCutoff"`
	TerminalCutoffDay      string    `json:"terminalCutoffDay"`
	DocCutoff              string    `json:"docCutoff"`
	DocCutoffDay           string    `json:"docCutoffDay"`
	VgmCutoff              string    `json:"vgmCutoff"`
	VgmCutoffDay           string    `json:"vgmCutoffDay"`
	VesselName             string    `json:"vesselName"`
	VoyageNumberArrival    string    `json:"voyageNumberArrival"`
	VoyageNumberDeparture  string    `json:"voyageNumberDeparture"`
	ImoNumber              string    `json:"imoNumber"`
	Geo                    []float64 `json:"geo"`
	Port                   string    `json:"port"`
	PortName               string    `json:"portName"`
	Terminal               string    `json:"terminal"`
	ArrivalDateEstimated   string    `json:"arrivalDateEstimated"`
	ArrivalDateActual      string    `json:"arrivalDateActual,omitempty"`
	DepartureDateEstimated string    `json:"departureDateEstimated"`
	DepartureDateActual    string    `json:"departureDateActual,omitempty"`
	BerthingDateEstimated  string    `json:"berthingDateEstimated"`
}

const oneDateFormat = "2006-01-02 15:04:05"

func (ovs *OneVesselSchedule) TokenHeaderParams(e *env.Manager) interfaces.HeaderParams {

	tokenHeaders := map[string]string{
		"apikey":        *e.OneToken,
		"Authorization": *e.OneAuth,
		"Content-Type":  "application/json",
	}
	tokenParams := map[string]string{
		"grant_type": "client_credentials",
	}
	headerParams := interfaces.HeaderParams{Headers: tokenHeaders, Params: tokenParams}
	return headerParams
}

func (ovs *OneVesselSchedule) ScheduleHeaderParams(p *interfaces.ScheduleArgs[*schema.QueryParamsForVesselVoyage]) interfaces.HeaderParams {
	scheduleHeaders := map[string]string{
		"apikey":        *p.Env.OneToken,
		"Authorization": fmt.Sprintf("Bearer %s", p.Token.Data["access_token"].(string)),
		"Accept":        "application/json",
	}
	startDate, endDate := external.CalculateDateRangeForMVS(p.Query.StartDate, p.Query.DateRange)

	scheduleParams := map[string]string{
		"transportID":         p.Query.VesselIMO,
		"transportIDTypeCode": "I",
		"departureDate":       startDate,
		"arrivalDate":         endDate,
	}

	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}

	return headerParams
}

func (ovs *OneVesselSchedule) GenerateSchedule(responseJson []byte) (*schema.MasterVesselSchedule, error) {
	var oneVesselSchedule OneVesselSchedule
	err := json.Unmarshal(responseJson, &oneVesselSchedule)
	if err != nil {
		return nil, err
	}
	if oneVesselSchedule.Vessel == nil {
		return nil, errors.New("one vessel schedule response is empty")
	}

	ovsResult := &schema.MasterVesselSchedule{
		Scac:     "ONEY",
		Voyage:   cmp.Or(oneVesselSchedule.Vessel[0].VoyageNumberArrival, oneVesselSchedule.Vessel[0].VoyageNumberDeparture),
		Vessel:   &schema.VesselDetails{VesselName: oneVesselSchedule.Vessel[0].VesselName, Imo: oneVesselSchedule.Vessel[0].ImoNumber},
		Services: &schema.Services{ServiceCode: oneVesselSchedule.Vessel[0].ServiceCode, ServiceName: oneVesselSchedule.Vessel[0].ServiceNameArrival},
		Calls:    ovs.GenerateVesselCalls(oneVesselSchedule.Vessel),
	}
	return ovsResult, nil
}

func (ovs *OneVesselSchedule) GenerateVesselCalls(vesselCalls []OneVessel) []schema.PortCalls {
	var countPortCall int
	var onePortCalls = make([]schema.PortCalls, 0, len(vesselCalls))

	for _, portCalls := range vesselCalls {
		portEvents := []PortEvent{
			{"Unloading", portCalls.VoyageNumberArrival, portCalls.ServiceCode, portCalls.ServiceNameArrival,
				external.ConvertDateFormat(&portCalls.ArrivalDateEstimated, oneDateFormat), external.ConvertDateFormat(&portCalls.ArrivalDateActual, oneDateFormat)},
			{"Loading", portCalls.VoyageNumberDeparture, portCalls.ServiceCode, portCalls.ServiceNameDeparture,
				external.ConvertDateFormat(&portCalls.DepartureDateEstimated, oneDateFormat), external.ConvertDateFormat(&portCalls.DepartureDateActual, oneDateFormat)},
		}
		for _, pe := range portEvents {
			countPortCall += 1
			portCallsResult := schema.PortCalls{
				Seq:       countPortCall,
				Key:       imo + pe.eventVoyageNumber + pe.serviceCode,
				Bound:     cmp.Or(voyageDirection[pe.eventVoyageNumber[len(pe.eventVoyageNumber)-1:]], "UNK"),
				Voyage:    pe.eventVoyageNumber,
				PortEvent: pe.eventType,
				Service:   &schema.Services{ServiceCode: pe.serviceCode, ServiceName: pe.serviceName},
				Port: &schema.Port{
					PortCode:     portCalls.Port,
					PortName:     portCalls.PortName,
					TerminalName: portCalls.Terminal},
				EstimatedEventDate: pe.estEventDate,
				ActualEventDate:    pe.actEventDate,
			}
			onePortCalls = append(onePortCalls, portCallsResult)
		}

	}
	return onePortCalls
}
