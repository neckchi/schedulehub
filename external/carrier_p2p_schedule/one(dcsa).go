package carrier_p2p_schedule

import (
	"cmp"
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/external"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Response represents the array of shipping schedules
type OneDCSAScheduleResponse []OneDCSAShippingSchedule

// ShippingSchedule represents a complete shipping schedule
type OneDCSAShippingSchedule struct {
	PlaceOfReceipt  OneDCSATerminal      `json:"placeOfReceipt"`
	PlaceOfDelivery OneDCSATerminal      `json:"placeOfDelivery"`
	TransitTime     int                  `json:"transitTime"`
	CutOffTimes     []*OneDCSACutOffTime `json:"cutOffTimes"`
	Legs            []*OneDCSALegs       `json:"legs"`
}

type OneDCSALegs struct {
	SequenceNumber int              `json:"sequenceNumber"`
	Transport      OneDCSATransport `json:"transport"`
	Departure      OneDCSATerminal  `json:"departure"`
	Arrival        OneDCSATerminal  `json:"arrival"`
}

type OneDCSATransport struct {
	ModeOfTransport        string                   `json:"modeOfTransport"`
	TransportCallReference string                   `json:"transportCallReference"`
	ServicePartner         []OneDCSAServicePartners `json:"servicePartners"`
	Vessel                 OneDCSAVessel            `json:"vessel"`
}

type OneDCSACutOffTime struct {
	CutOffDateTimeCode string `json:"cutOffDateTimeCode"`
	CutOffDateTime     string `json:"cutOffDateTime"`
}
type OneDCSAVessel struct {
	VesselIMONumber                 string `json:"vesselIMONumber"`
	Name                            string `json:"name"`
	CallSign                        string `json:"callSign"`
	OperatorCarrierCode             string `json:"operatorCarrierCode"`
	OperatorCarrierCodeListProvider string `json:"operatorCarrierCodeListProvider"`
}

type OneDCSAServicePartners struct {
	CarrierCode               string `json:"carrierCode"`
	CarrierCodeListProvider   string `json:"carrierCodeListProvider"`
	CarrierServiceName        string `json:"carrierServiceName"`
	CarrierServiceCode        string `json:"carrierServiceCode"`
	CarrierImportVoyageNumber string `json:"carrierImportVoyageNumber"`
	CarrierExportVoyageNumber string `json:"carrierExportVoyageNumber"`
}

type OneDCSATerminal struct {
	FacilityTypeCode string          `json:"facilityTypeCode"`
	Location         OneDCSALocation `json:"location"`
	DateTime         string          `json:"dateTime"`
}

type OneDCSALocation struct {
	LocationName   string          `json:"locationName"`
	LocationType   string          `json:"locationType"`
	UNLocationCode string          `json:"UNLocationCode"`
	Facility       OneDCSAFacility `json:"facility,omitempty"`
	Address        *OneDCSAddress  `json:"address,omitempty"`
}

type OneDCSAddress struct {
	Name     string `json:"name"`
	Street   string `json:"street"`
	PostCode string `json:"postCode"`
	City     string `json:"city"`
	Country  string `json:"country"`
}

type OneDCSAFacility struct {
	FacilityCode             string `json:"facilityCode"`
	FacilityCodeListProvider string `json:"facilityCodeListProvider"`
}

func (ocr *OneDCSAScheduleResponse) GenerateSchedule(responseJson []byte) ([]*schema.P2PSchedule, error) {
	var oneScheduleData OneDCSAScheduleResponse
	err := json.Unmarshal(responseJson, &oneScheduleData)
	if err != nil {
		return nil, err
	}
	var oneScheduleList = make([]*schema.P2PSchedule, 0, len(oneScheduleData))
	for _, route := range oneScheduleData {
		etd := external.ConvertDateFormat(&route.PlaceOfReceipt.DateTime, dcsaDateFormat)
		eta := external.ConvertDateFormat(&route.PlaceOfDelivery.DateTime, dcsaDateFormat)
		scheduleResult := &schema.P2PSchedule{
			Scac:          "ONEY",
			PointFrom:     route.PlaceOfReceipt.Location.UNLocationCode,
			PointTo:       route.PlaceOfDelivery.Location.UNLocationCode,
			Etd:           etd,
			Eta:           eta,
			TransitTime:   cmp.Or(route.TransitTime, external.CalculateTransitTime(&etd, &eta)),
			Transshipment: len(route.Legs) > 1,
			Legs:          ocr.GenerateScheduleLeg(route.CutOffTimes, route.Legs),
		}
		oneScheduleList = append(oneScheduleList, scheduleResult)

	}
	return oneScheduleList, nil
}

func (ocr *OneDCSAScheduleResponse) GenerateScheduleLeg(cutOffs []*OneDCSACutOffTime, legResponse []*OneDCSALegs) []*schema.Leg {
	var oneLegList = make([]*schema.Leg, 0, len(legResponse))
	for seq, leg := range legResponse {
		pointBase := ocr.GenerateLegPoints(leg)
		eventDate := ocr.GenerateEventDate(seq, cutOffs, leg)
		voyageService := ocr.GenerateVoyageService(leg)
		legInstance := &schema.Leg{
			PointFrom:       pointBase.PointFrom,
			PointTo:         pointBase.PointTo,
			Etd:             eventDate.Etd,
			Eta:             eventDate.Eta,
			TransitTime:     eventDate.TransitTime,
			Cutoffs:         eventDate.Cutoffs,
			Transportations: ocr.GenerateTransport(leg).Transportations,
			Voyages:         voyageService.Voyages,
			Services:        voyageService.Services,
		}

		oneLegList = append(oneLegList, legInstance)
	}
	return oneLegList
}

func (ocr *OneDCSAScheduleResponse) GenerateLegPoints(legDetails *OneDCSALegs) *schema.Leg {
	pointFrom := schema.PointBase{
		LocationName: legDetails.Departure.Location.Address.City,
		LocationCode: legDetails.Departure.Location.UNLocationCode,
		TerminalCode: legDetails.Departure.Location.Facility.FacilityCode,
		TerminalName: legDetails.Departure.Location.LocationName,
	}

	pointTo := schema.PointBase{
		LocationName: legDetails.Arrival.Location.Address.City,
		LocationCode: legDetails.Arrival.Location.UNLocationCode,
		TerminalCode: legDetails.Arrival.Location.Facility.FacilityCode,
		TerminalName: legDetails.Arrival.Location.LocationName,
	}

	portPairs := &schema.Leg{
		PointFrom: &pointFrom,
		PointTo:   &pointTo,
	}
	return portPairs
}

func (ocr *OneDCSAScheduleResponse) GenerateEventDate(seq int, cutOffs []*OneDCSACutOffTime, legDetails *OneDCSALegs) *schema.Leg {
	etd := external.ConvertDateFormat(&legDetails.Departure.DateTime, dcsaDateFormat)
	eta := external.ConvertDateFormat(&legDetails.Arrival.DateTime, dcsaDateFormat)
	var cyCutoffDate, docCutoffDate, vgmCutoffDate string
	if seq == 0 {
		for _, cutOff := range cutOffs {
			switch cutOff.CutOffDateTimeCode {
			case "DCO":
				docCutoffDate = external.ConvertDateFormat(&cutOff.CutOffDateTime, dcsaDateFormat)
			case "VCO":
				vgmCutoffDate = external.ConvertDateFormat(&cutOff.CutOffDateTime, dcsaDateFormat)
			case "FCO":
				cyCutoffDate = external.ConvertDateFormat(&cutOff.CutOffDateTime, dcsaDateFormat)
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

func (ocr *OneDCSAScheduleResponse) GenerateTransport(legDetails *OneDCSALegs) *schema.Leg {
	meanOfTransport := cmp.Or(cases.Title(language.Und).String(legDetails.Transport.ModeOfTransport), "Vessel")
	vesselName := legDetails.Transport.Vessel.Name
	vesselIMO := legDetails.Transport.Vessel.VesselIMONumber

	var referenceType, reference string
	switch {
	case external.ValidateIMO(vesselIMO) && vesselIMO != "0000000":
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

func (ocr *OneDCSAScheduleResponse) GenerateVoyageService(legDetails *OneDCSALegs) *schema.Leg {
	voyage := &schema.Voyage{
		InternalVoyage: cmp.Or(legDetails.Transport.ServicePartner[0].CarrierExportVoyageNumber, "TBN"),
	}

	var service *schema.Service
	serviceCode := legDetails.Transport.ServicePartner[0].CarrierServiceCode
	if serviceCode != "" {
		service = &schema.Service{ServiceCode: serviceCode, ServiceName: legDetails.Transport.ServicePartner[0].CarrierServiceName}
	}

	voyageServices := &schema.Leg{
		Voyages:  voyage,
		Services: service,
	}

	return voyageServices
}

func (ocr *OneDCSAScheduleResponse) TokenHeaderParams(e *env.Manager) interfaces.HeaderParams {

	tokenHeaders := map[string]string{
		"apikey":        *e.OneToken,
		"Authorization": *e.OneDCSAAuth,
		"Content-Type":  "application/json",
	}
	tokenParams := map[string]string{
		"grant_type": "client_credentials",
	}
	headerParams := interfaces.HeaderParams{Headers: tokenHeaders, Params: tokenParams}
	return headerParams
}

func (ocr *OneDCSAScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs[*schema.QueryParams]) interfaces.HeaderParams {

	const queryTimeFormat = "2006-01-02"

	scheduleHeaders := map[string]string{
		"apikey":        *p.Env.OneClientID,
		"Authorization": fmt.Sprintf("Bearer %s", p.Token.Data["access_token"].(string)),
		"Accept":        "application/json",
	}
	startDate, endDate, _ := external.CalculateDateRangeForP2P(p.Query, queryTimeFormat)

	scheduleParams := map[string]string{
		"placeOfReceipt":  p.Query.PointFrom,
		"placeOfDelivery": p.Query.PointTo,
	}

	if p.Query.StartDateType == schema.Departure {
		scheduleParams["departureStartDate"] = startDate
		scheduleParams["departureEndDate"] = endDate
	} else {
		scheduleParams["arrivalStartDate"] = startDate
		scheduleParams["arrivalEndDate"] = endDate
	}
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams

}
