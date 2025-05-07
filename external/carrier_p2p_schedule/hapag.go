package carrier_p2p_schedule

import (
	"cmp"
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/external"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"time"
)

// Response represents the array of shipping schedules
type HapagScheduleResponse []ShippingSchedule

// ShippingSchedule represents a complete shipping schedule
type ShippingSchedule struct {
	PlaceOfReceipt  terminal      `json:"placeOfReceipt"`
	PlaceOfDelivery terminal      `json:"placeOfDelivery"`
	TransitTime     int           `json:"transitTime"`
	CutOffTimes     []*cutOffTime `json:"cutOffTimes"`
	Legs            []*hleg       `json:"legs"`
}

type hleg struct {
	SequenceNumber                 int      `json:"sequenceNumber"`
	ModeOfTransport                string   `json:"modeOfTransport"`
	VesselOperatorSMDGLinerCode    string   `json:"vesselOperatorSMDGLinerCode"`
	VesselIMONumber                string   `json:"vesselIMONumber"`
	VesselName                     string   `json:"vesselName"`
	CarrierServiceName             string   `json:"carrierServiceName"`
	CarrierServiceCode             string   `json:"carrierServiceCode"`
	UniversalImportVoyageReference *string  `json:"universalImportVoyageReference"`
	UniversalExportVoyageReference string   `json:"universalExportVoyageReference"`
	CarrierImportVoyageNumber      *string  `json:"carrierImportVoyageNumber"`
	CarrierExportVoyageNumber      *string  `json:"carrierExportVoyageNumber"`
	Departure                      terminal `json:"departure"`
	Arrival                        terminal `json:"arrival"`
}

type cutOffTime struct {
	CutOffDateTimeCode string `json:"cutOffDateTimeCode"`
	CutOffDateTime     string `json:"cutOffDateTime"`
}

type terminal struct {
	FacilityTypeCode string   `json:"facilityTypeCode"`
	Location         location `json:"location"`
	DateTime         string   `json:"dateTime"`
}

type location struct {
	LocationName     string   `json:"locationName"`
	LocationType     string   `json:"locationType"`
	UNLocationCode   string   `json:"UNLocationCode"`
	FacilitySMDGCode string   `json:"facilitySMDGCode,omitempty"`
	Address          *address `json:"address,omitempty"`
}

type address struct {
	Name     string `json:"name"`
	Street   string `json:"street"`
	PostCode string `json:"postCode"`
	City     string `json:"city"`
	Country  string `json:"country"`
}

const hapagDateFormat string = "2006-01-02T15:04:05-07:00"

func (hsp *HapagScheduleResponse) GenerateSchedule(responseJson []byte) ([]*schema.P2PSchedule, error) {
	var hapagScheduleData HapagScheduleResponse
	err := json.Unmarshal(responseJson, &hapagScheduleData)
	if err != nil {
		return nil, err
	}
	var hapagScheduleList = make([]*schema.P2PSchedule, 0, len(hapagScheduleData))
	for _, route := range hapagScheduleData {
		etd := external.ConvertDateFormat(&route.PlaceOfReceipt.DateTime, hapagDateFormat)
		eta := external.ConvertDateFormat(&route.PlaceOfDelivery.DateTime, hapagDateFormat)
		scheduleResult := &schema.P2PSchedule{
			Scac:          "HLCU",
			PointFrom:     route.PlaceOfReceipt.Location.UNLocationCode,
			PointTo:       route.PlaceOfDelivery.Location.UNLocationCode,
			Etd:           etd,
			Eta:           eta,
			TransitTime:   cmp.Or(route.TransitTime, external.CalculateTransitTime(&etd, &eta)),
			Transshipment: len(route.Legs) > 1,
			Legs:          hsp.GenerateScheduleLeg(route.CutOffTimes, route.Legs),
		}
		hapagScheduleList = append(hapagScheduleList, scheduleResult)

	}
	return hapagScheduleList, nil
}

func (hsp *HapagScheduleResponse) GenerateScheduleLeg(cutOffs []*cutOffTime, legResponse []*hleg) []*schema.Leg {
	var hapagLegList = make([]*schema.Leg, 0, len(legResponse))
	for seq, leg := range legResponse {
		pointBase := hsp.GenerateLegPoints(leg)
		eventDate := hsp.GenerateEventDate(seq, cutOffs, leg)
		voyageService := hsp.GenerateVoyageService(leg)
		legInstance := &schema.Leg{
			PointFrom:       pointBase.PointFrom,
			PointTo:         pointBase.PointTo,
			Etd:             eventDate.Etd,
			Eta:             eventDate.Eta,
			TransitTime:     eventDate.TransitTime,
			Cutoffs:         eventDate.Cutoffs,
			Transportations: hsp.GenerateTransport(leg).Transportations,
			Voyages:         voyageService.Voyages,
			Services:        voyageService.Services,
		}

		hapagLegList = append(hapagLegList, legInstance)
	}
	return hapagLegList
}

func (hsp *HapagScheduleResponse) GenerateLegPoints(legDetails *hleg) *schema.Leg {
	pointFrom := schema.PointBase{
		LocationName: legDetails.Departure.Location.LocationName,
		LocationCode: legDetails.Departure.Location.UNLocationCode,
		TerminalCode: legDetails.Departure.Location.FacilitySMDGCode,
	}

	pointTo := schema.PointBase{
		LocationName: legDetails.Arrival.Location.LocationName,
		LocationCode: legDetails.Arrival.Location.UNLocationCode,
		TerminalCode: legDetails.Arrival.Location.FacilitySMDGCode,
	}

	portPairs := &schema.Leg{
		PointFrom: &pointFrom,
		PointTo:   &pointTo,
	}
	return portPairs
}

func (hsp *HapagScheduleResponse) GenerateEventDate(seq int, cutOffs []*cutOffTime, legDetails *hleg) *schema.Leg {
	etd := external.ConvertDateFormat(&legDetails.Departure.DateTime, hapagDateFormat)
	eta := external.ConvertDateFormat(&legDetails.Arrival.DateTime, hapagDateFormat)
	var cyCutoffDate, docCutoffDate, vgmCutoffDate string
	if seq == 0 {
		for _, cutOff := range cutOffs {
			switch cutOff.CutOffDateTimeCode {
			case "DCO":
				docCutoffDate = external.ConvertDateFormat(&cutOff.CutOffDateTime, hapagDateFormat)
			case "VCO":
				vgmCutoffDate = external.ConvertDateFormat(&cutOff.CutOffDateTime, hapagDateFormat)
			case "FCO":
				cyCutoffDate = external.ConvertDateFormat(&cutOff.CutOffDateTime, hapagDateFormat)
			}

		}
	}
	var cf *schema.Cutoff
	if cyCutoffDate != "" || docCutoffDate != "" || vgmCutoffDate != "" {
		cf = &schema.Cutoff{
			CyCutoffDate:  cyCutoffDate,
			DocCutoffDate: docCutoffDate,
			VgmCutoffDate: vgmCutoffDate,
		}
	}

	eventTime := &schema.Leg{
		Etd:         etd,
		Eta:         eta,
		TransitTime: external.CalculateTransitTime(&etd, &eta),
		Cutoffs:     cf,
	}

	return eventTime
}

func (hsp *HapagScheduleResponse) GenerateTransport(legDetails *hleg) *schema.Leg {
	meanOfTransport := cmp.Or(cases.Title(language.Und).String(legDetails.ModeOfTransport), "Vessel")
	vesselName := legDetails.VesselName
	vesselIMO := legDetails.VesselIMONumber

	var referenceType, reference string
	switch {
	case len(vesselIMO) > 0 && len(vesselIMO) < 9 && vesselIMO != "0000000":
		referenceType = "IMO"
		reference = vesselIMO
	}

	tr := schema.Transportation{
		TransportType: schema.TransportType(meanOfTransport),
		TransportName: vesselName,
		ReferenceType: referenceType,
		Reference:     reference,
	}
	err := tr.MapTransport()
	if err != nil {
		panic(err)
	}
	trDetails := &schema.Leg{
		Transportations: tr,
	}
	return trDetails
}

func (hsp *HapagScheduleResponse) GenerateVoyageService(legDetails *hleg) *schema.Leg {
	voyage := &schema.Voyage{
		InternalVoyage: cmp.Or(legDetails.UniversalExportVoyageReference, "TBN"),
	}

	var service *schema.Service
	serviceCode := legDetails.CarrierServiceCode
	if serviceCode != "" {
		service = &schema.Service{ServiceCode: serviceCode, ServiceName: legDetails.CarrierServiceName}
	}

	voyageServices := &schema.Leg{
		Voyages:  voyage,
		Services: service,
	}

	return voyageServices
}

func (hsp *HapagScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs[*schema.QueryParams]) interfaces.HeaderParams {

	const queryTimeFormat = "2006-01-02T15:04:05.000Z"

	var calculateDateRange = func(q *schema.QueryParams) (startTime, endTime string, err error) {
		date, err := time.Parse("2006-01-02", q.StartDate)
		if err != nil {
			return "", "", fmt.Errorf("failed to parse date: %w", err)
		}
		startDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		endDate := startDate.AddDate(0, 0, q.SearchRange*7)
		startTime = startDate.Format(queryTimeFormat)
		endTime = endDate.Format(queryTimeFormat)
		return startTime, endTime, nil
	}

	scheduleHeaders := map[string]string{
		"X-IBM-Client-Id":     *p.Env.HapagClient,
		"X-IBM-Client-Secret": *p.Env.HapagSecret,
		"Accept":              "application/json",
	}

	startDate, endDate, _ := calculateDateRange(p.Query)

	scheduleParams := map[string]string{
		"placeOfReceipt":  p.Query.PointFrom,
		"placeOfDelivery": p.Query.PointTo,
	}

	if p.Query.StartDateType == schema.Departure {
		scheduleParams["departureDateTime:gte"] = startDate
		scheduleParams["departureDateTime:lte"] = endDate
	} else {
		scheduleParams["arrivalDateTime:gte"] = startDate
		scheduleParams["arrivalDateTime:lte"] = endDate
	}
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams

}
