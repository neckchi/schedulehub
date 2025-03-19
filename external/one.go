package external

import (
	"cmp"
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	"math"
	"strconv"
)

type OneScheduleBody struct {
	Scac                            string    `json:"scac"`
	CarrierName                     string    `json:"carrierName"`
	ServiceCode                     string    `json:"serviceCode"`
	ServiceName                     string    `json:"serviceName"`
	TerminalCutoff                  string    `json:"terminalCutoff"`
	TerminalCutoffDay               string    `json:"terminalCutoffDay"`
	DocCutoff                       string    `json:"docCutoff"`
	DocCutoffDay                    string    `json:"docCutoffDay"`
	VgmCutoff                       string    `json:"vgmCutoff"`
	VgmCutoffDay                    string    `json:"vgmCutoffDay"`
	VesselName                      string    `json:"vesselName"`
	VoyageNumber                    string    `json:"voyageNumber"`
	ImoNumber                       string    `json:"imoNumber"`
	OriginUnloc                     string    `json:"originUnloc"`
	OriginGeo                       []float64 `json:"originGeo"`
	OriginTerminal                  string    `json:"originTerminal"`
	OriginDepartureDateEstimated    string    `json:"originDepartureDateEstimated"`
	OriginDepartureDateActual       string    `json:"originDepartureDateActual"`
	DestinationUnloc                string    `json:"destinationUnloc"`
	DestinationGeo                  []float64 `json:"destinationGeo"`
	DestinationTerminal             string    `json:"destinationTerminal"`
	DestinationArrivalDateEstimated string    `json:"destinationArrivalDateEstimated"`
	DestinationArrivalDateActual    string    `json:"destinationArrivalDateActual"`
	TransitDurationHrsUtc           int       `json:"transitDurationHrsUtc"`
}

type OneLeg struct {
	Sequence               int       `json:"sequence"`
	ServiceCode            string    `json:"serviceCode"`
	ServiceName            string    `json:"serviceName"`
	TransportType          string    `json:"transportType"`
	TransportName          string    `json:"transportName"`
	ConveyanceNumber       string    `json:"conveyanceNumber"`
	TransportID            string    `json:"transportID"`
	DepartureUnloc         string    `json:"departureUnloc"`
	DepartureGeo           []float64 `json:"departureGeo"`
	DepartureTerminal      string    `json:"departureTerminal"`
	DepartureDateEstimated string    `json:"departureDateEstimated"`
	DepartueDateActual     string    `json:"departueDateActual"`
	ArrivalUnloc           string    `json:"arrivalUnloc"`
	ArrivalGeo             []float64 `json:"arrivalGeo"`
	ArrivalTerminal        string    `json:"arrivalTerminal"`
	ArrivalDateEstimated   string    `json:"arrivalDateEstimated"`
	ArrivalDateActual      string    `json:"arrivalDateActual"`
	TransitDurationHrsUtc  int       `json:"transitDurationHrsUtc"`
	TransshipmentIndicator bool      `json:"transshipmentIndicator"`
}

type OneRoute struct {
	OneScheduleBody
	Legs []*OneLeg `json:"legs"`
}

type OneScheduleResponse struct {
	Direct        []*OneRoute `json:"Direct"`
	Transshipment []*OneRoute `json:"Transshipment"`
}

const oneDateFormat = "2006-01-02 15:04:05"

func (osp *OneScheduleResponse) GenerateSchedule(responseJson []byte) ([]*schema.Schedule, error) {
	var oneScheduleData OneScheduleResponse
	if err := json.Unmarshal(responseJson, &oneScheduleData); err != nil {
		return nil, err
	}
	totalSchedules := len(oneScheduleData.Direct) + len(oneScheduleData.Transshipment)
	var oneScheduleList = make([]*schema.Schedule, 0, totalSchedules)

	convertSchedule := func(schedule *OneRoute) *schema.Schedule {
		tt := float64(schedule.TransitDurationHrsUtc) / 24
		return &schema.Schedule{
			Scac:          schedule.Scac,
			PointFrom:     schedule.OriginUnloc,
			PointTo:       schedule.DestinationUnloc,
			Etd:           ConvertDateFormat(&schedule.OriginDepartureDateEstimated, oneDateFormat),
			Eta:           ConvertDateFormat(&schedule.DestinationArrivalDateEstimated, oneDateFormat),
			TransitTime:   int(math.Floor(tt + 0.5)),
			Transshipment: len(schedule.Legs) > 1,
			Legs:          osp.GenerateScheduleLeg(schedule, schedule.Legs),
		}
	}
	for _, schedule := range oneScheduleData.Direct {
		oneScheduleList = append(oneScheduleList, convertSchedule(schedule))
	}
	for _, schedule := range oneScheduleData.Transshipment {
		oneScheduleList = append(oneScheduleList, convertSchedule(schedule))
	}

	return oneScheduleList, nil
}

func (osp *OneScheduleResponse) GenerateScheduleLeg(schedule *OneRoute, legResponse []*OneLeg) []*schema.Leg {
	capacity := 1
	if len(schedule.Legs) > 0 {
		capacity = len(legResponse)
	}
	var oneLegList = make([]*schema.Leg, 0, capacity)
	generateLeg := func(leg *OneLeg) *schema.Leg {
		points := osp.GenerateLegPoints(schedule, leg)
		eventDates := osp.GenerateEventDate(schedule, leg)
		transport := osp.GenerateTransport(schedule, leg)
		voyageService := osp.GenerateVoyageService(schedule, leg)
		return &schema.Leg{
			PointFrom:       points.PointFrom,
			PointTo:         points.PointTo,
			Etd:             eventDates.Etd,
			Eta:             eventDates.Eta,
			TransitTime:     eventDates.TransitTime,
			Cutoffs:         eventDates.Cutoffs,
			Transportations: transport.Transportations,
			Voyages:         voyageService.Voyages,
			Services:        voyageService.Services,
		}
	}
	if len(schedule.Legs) > 0 {
		for _, leg := range legResponse {
			oneLegList = append(oneLegList, generateLeg(leg))
		}
	} else {
		oneLegList = append(oneLegList, generateLeg(nil))
	}
	return oneLegList
}

func (osp *OneScheduleResponse) GenerateLegPoints(schedule *OneRoute, legDetails *OneLeg) *schema.Leg {
	var pol, pod, originTerminal, desTerminal string
	switch legDetails {
	case nil:
		pol = schedule.OriginUnloc
		originTerminal = schedule.OriginTerminal
		pod = schedule.DestinationUnloc
		desTerminal = schedule.DestinationTerminal
	default:
		pol = legDetails.DepartureUnloc
		originTerminal = legDetails.DepartureTerminal
		pod = legDetails.ArrivalUnloc
		desTerminal = legDetails.ArrivalTerminal
	}

	pointFrom := schema.PointBase{
		LocationCode: pol,
		TerminalName: originTerminal,
	}

	pointTo := schema.PointBase{
		LocationCode: pod,
		TerminalName: desTerminal,
	}

	portPairs := &schema.Leg{
		PointFrom: pointFrom,
		PointTo:   pointTo,
	}
	return portPairs
}

func (osp *OneScheduleResponse) GenerateEventDate(schedule *OneRoute, legDetails *OneLeg) *schema.Leg {
	var convertEtd, convertEta string
	var tt int
	switch legDetails {
	case nil:
		convertEtd = ConvertDateFormat(&schedule.OriginDepartureDateEstimated, oneDateFormat)
		convertEta = ConvertDateFormat(&schedule.DestinationArrivalDateEstimated, oneDateFormat)
		tt = CalculateTransitTime(&convertEtd, &convertEta)

	default:
		convertEtd = ConvertDateFormat(&legDetails.DepartureDateEstimated, oneDateFormat)
		convertEta = ConvertDateFormat(&legDetails.ArrivalDateEstimated, oneDateFormat)
		tt = CalculateTransitTime(&convertEtd, &convertEta)
	}

	cyCutoffDate := ConvertDateFormat(&schedule.TerminalCutoff, oneDateFormat)
	docCutoffDate := ConvertDateFormat(&schedule.DocCutoff, oneDateFormat)
	vgmCutoffDate := ConvertDateFormat(&schedule.VgmCutoff, oneDateFormat)

	var cutoffs *schema.Cutoff
	if cyCutoffDate != "" || docCutoffDate != "" || vgmCutoffDate != "" {
		cutoffs = &schema.Cutoff{
			CyCutoffDate:  cyCutoffDate,
			DocCutoffDate: docCutoffDate,
			VgmCutoffDate: vgmCutoffDate,
		}
	}

	eventTime := &schema.Leg{
		Etd:         convertEtd,
		Eta:         convertEta,
		TransitTime: tt,
		Cutoffs:     cutoffs,
	}

	return eventTime
}

func (osp *OneScheduleResponse) GenerateTransport(schedule *OneRoute, legDetails *OneLeg) *schema.Leg {
	var tn, ref string
	switch legDetails {
	case nil:
		tn = schedule.VesselName
		ref = schedule.ImoNumber

	default:
		tn = legDetails.TransportName
		ref = legDetails.TransportID
	}

	tr := schema.Transportation{
		TransportType: schema.TransportType("Vessel"),
		TransportName: tn,
		ReferenceType: "IMO",
		Reference:     ref,
	}

	err := tr.MapTransport()
	if err != nil {
		panic(err)
	}
	transportDetails := &schema.Leg{
		Transportations: tr,
	}
	return transportDetails
}

func (osp *OneScheduleResponse) GenerateVoyageService(schedule *OneRoute, legDetails *OneLeg) *schema.Leg {
	var sc, sn, vn string
	switch legDetails {
	case nil:
		sc = schedule.ServiceCode
		sn = schedule.ServiceName
		vn = schedule.VoyageNumber

	default:
		sc = legDetails.ServiceCode
		sn = legDetails.ServiceName
		vn = legDetails.ConveyanceNumber
	}

	finalVoyage := &schema.Voyage{
		InternalVoyage: cmp.Or(vn, "TBN"),
	}

	var service *schema.Service
	if sc != "" {
		service = &schema.Service{
			ServiceCode: &sc,
			ServiceName: &sn,
		}
	}
	voyageServices := &schema.Leg{
		Voyages:  finalVoyage,
		Services: service,
	}

	return voyageServices
}

func (osp *OneScheduleResponse) TokenHeaderParams(e *env.Manager) interfaces.HeaderParams {

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

func (osp *OneScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs) interfaces.HeaderParams {
	scheduleHeaders := map[string]string{
		"apikey":        *p.Env.OneToken,
		"Authorization": fmt.Sprintf("Bearer %s", p.Token.Data["access_token"].(string)),
		"Accept":        "application/json",
	}
	scheduleParams := map[string]string{
		"originPort":      *p.Query.PointFrom,
		"destinationPort": *p.Query.PointTo,
		"searchDate":      *p.Query.StartDate,
		"weeksOut":        strconv.Itoa(*p.Query.SearchRange),
	}
	if *p.Query.StartDateType == schema.Departure {
		scheduleParams["searchDateType"] = "BY_DEPARTURE_DATE"
	} else {
		scheduleParams["searchDateType"] = "BY_ARRIVAL_DATE"
	}

	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}

	return headerParams
}
