package external

import (
	"cmp"
	"encoding/json"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"strconv"
	"strings"
	"time"
)

type CmaScheduleResponse []CmaSchedule

type CmaSchedule struct {
	ShippingCompany string           `json:"shippingCompany"`
	SolutionNo      int              `json:"solutionNo"`
	TransitTime     int              `json:"transitTime"`
	RoutingDetails  []*RoutingDetail `json:"routingDetails"`
}

type RoutingDetail struct {
	PointFrom      PointBase      `json:"pointFrom"`
	PointTo        PointBase      `json:"pointTo"`
	Transportation Transportation `json:"transportation"`
	LegTransitTime int            `json:"legTransitTime"`
}

type PointBase struct {
	Location           Location `json:"location"`
	DepartureDateLocal string   `json:"departureDateLocal,omitempty"`
	DepartureDateGmt   string   `json:"departureDateGmt,omitempty"`
	ArrivalDateLocal   string   `json:"arrivalDateLocal,omitempty"`
	ArrivalDateGmt     string   `json:"arrivalDateGmt,omitempty"`
	CutOff             CutOff   `json:"cutOff,omitempty"`
}

type Location struct {
	Name                  string         `json:"name"`
	InternalCode          string         `json:"internalCode"`
	LocationCodifications []Codification `json:"locationCodifications"`
	Facility              Facility       `json:"facility"`
}

type Codification struct {
	CodificationType string `json:"codificationType"`
	Codification     string `json:"codification"`
}

type Facility struct {
	FacilityType          string         `json:"facilityType"`
	InternalCode          string         `json:"internalCode"`
	FacilityCodifications []Codification `json:"facilityCodifications"`
	Name                  string         `json:"name"`
}

type CutOff struct {
	PortCutoff                    *TimeDetails `json:"portCutoff"`
	ShippingInstructionAcceptance *TimeDetails `json:"shippingInstructionAcceptance"`
	Vgm                           *TimeDetails `json:"vgm"`
}

type TimeDetails struct {
	Local string `json:"local"`
	Utc   string `json:"utc"`
}

type Transportation struct {
	MeanOfTransport string   `json:"meanOfTransport"`
	Vehicule        Vehicule `json:"vehicule"`
	Voyage          Voyage   `json:"voyage"`
}

type Vehicule struct {
	VehiculeType      string `json:"vehiculeType"`
	VehiculeName      string `json:"vehiculeName"`
	Reference         string `json:"reference"`
	ReferenceType     string `json:"referenceType"`
	InternalReference string `json:"internalReference"`
	SmdgLinerCode     string `json:"smdgLinerCode"`
}

type Voyage struct {
	VoyageReference string  `json:"voyageReference"`
	Service         Service `json:"service"`
}

type Service struct {
	Code         *string `json:"code"`
	InternalCode *string `json:"internalCode"`
}

var getLocationCode = func(cmaSchedule *CmaSchedule, portType string) string {
	pol := cmaSchedule.RoutingDetails[0].PointFrom.Location
	pod := cmaSchedule.RoutingDetails[len(cmaSchedule.RoutingDetails)-1].PointTo.Location
	switch {
	case portType == "pol" && pol.InternalCode != "":
		return pol.InternalCode
	case portType == "pol" && len(pol.LocationCodifications) > 0:
		return pol.LocationCodifications[0].Codification
	case pod.InternalCode != "":
		return pod.InternalCode
	case len(pod.LocationCodifications) > 0:
		return pod.LocationCodifications[0].Codification
	default:
		return ""
	}
}

const cmaDateFormat string = "2006-01-02T15:04:05Z"

func (csp *CmaScheduleResponse) GenerateSchedule(responseJson []byte) ([]*schema.Schedule, error) {
	var getFirstEtdLastEta = func(cmaSchedule *CmaSchedule, portType string) string {
		var reformatDate string
		if portType == "etd" {
			for _, routeDetails := range cmaSchedule.RoutingDetails {
				if routeDetails.PointFrom.DepartureDateGmt != "" {
					reformatDate = ConvertDateFormat(&routeDetails.PointFrom.DepartureDateGmt, cmaDateFormat)
					break
				}
			}
		} else {
			for i := len(cmaSchedule.RoutingDetails) - 1; i >= 0; i-- {
				if cmaSchedule.RoutingDetails[i].PointTo.ArrivalDateGmt != "" {
					reformatDate = ConvertDateFormat(&cmaSchedule.RoutingDetails[i].PointTo.ArrivalDateGmt, cmaDateFormat)
					break
				}
			}
		}
		return reformatDate
	}
	var CMAScheduleData CmaScheduleResponse
	if err := json.Unmarshal(responseJson, &CMAScheduleData); err != nil {
		return nil, err
	}
	var cmaScheduleList = make([]*schema.Schedule, 0, len(CMAScheduleData))
	for _, route := range CMAScheduleData {
		scheduleResult := &schema.Schedule{
			Scac:          string(schema.InternalCodeToScac[route.ShippingCompany]),
			PointFrom:     getLocationCode(&route, "pol"),
			PointTo:       getLocationCode(&route, "pod"),
			Etd:           cmp.Or(getFirstEtdLastEta(&route, "etd"), time.Now().Format("2006-01-02T15:04:05")),
			Eta:           cmp.Or(getFirstEtdLastEta(&route, "eta"), time.Now().Format("2006-01-02T15:04:05")),
			TransitTime:   route.TransitTime,
			Transshipment: len(route.RoutingDetails) > 1,
			Legs:          csp.GenerateScheduleLeg(route.RoutingDetails),
		}
		cmaScheduleList = append(cmaScheduleList, scheduleResult)

	}
	return cmaScheduleList, nil
}

func (csp *CmaScheduleResponse) GenerateScheduleLeg(legResponse []*RoutingDetail) []*schema.Leg {
	var cmaLegList = make([]*schema.Leg, 0, len(legResponse))
	for _, leg := range legResponse {
		pointBase := csp.GenerateLegPoints(leg)
		eventDate := csp.GenerateEventDate(leg)
		voyageService := csp.GenerateVoyageService(&leg.Transportation.Voyage)
		legInstance := &schema.Leg{
			PointFrom:       pointBase.PointFrom,
			PointTo:         pointBase.PointTo,
			Etd:             eventDate.Etd,
			Eta:             eventDate.Eta,
			TransitTime:     eventDate.TransitTime,
			Cutoffs:         eventDate.Cutoffs,
			Transportations: csp.GenerateTransport(&leg.Transportation).Transportations,
			Voyages:         voyageService.Voyages,
			Services:        voyageService.Services,
		}

		cmaLegList = append(cmaLegList, legInstance)
	}
	return cmaLegList
}

func (csp *CmaScheduleResponse) GenerateLegPoints(legDetails *RoutingDetail) *schema.Leg {
	pointFrom := schema.PointBase{
		LocationName: legDetails.PointFrom.Location.Name,
		LocationCode: cmp.Or(legDetails.PointFrom.Location.InternalCode, legDetails.PointFrom.Location.LocationCodifications[0].Codification),
		TerminalName: legDetails.PointFrom.Location.Facility.Name,
		//TerminalCode: legDetails.PointFrom.Location.Facility.FacilityCodifications[0].Codification ,
	}

	if len(legDetails.PointFrom.Location.Facility.FacilityCodifications) != 0 {
		pointFrom.TerminalCode = legDetails.PointFrom.Location.Facility.FacilityCodifications[0].Codification
	}

	pointTo := schema.PointBase{
		LocationName: legDetails.PointTo.Location.Name,
		LocationCode: cmp.Or(legDetails.PointTo.Location.InternalCode, legDetails.PointTo.Location.LocationCodifications[0].Codification),
		TerminalName: legDetails.PointTo.Location.Facility.Name,
		//TerminalCode: legDetails.PointTo.Location.Facility.FacilityCodifications[0].Codification,
	}

	if len(legDetails.PointTo.Location.Facility.FacilityCodifications) != 0 {
		pointTo.TerminalCode = legDetails.PointTo.Location.Facility.FacilityCodifications[0].Codification
	}

	portPairs := &schema.Leg{
		PointFrom: pointFrom,
		PointTo:   pointTo,
	}
	return portPairs
}

func (csp *CmaScheduleResponse) GenerateEventDate(legDetails *RoutingDetail) *schema.Leg {
	etd := cmp.Or(ConvertDateFormat(&legDetails.PointFrom.DepartureDateGmt, cmaDateFormat), time.Now().Format("2006-01-02T15:04:05"))
	eta := cmp.Or(ConvertDateFormat(&legDetails.PointTo.ArrivalDateGmt, cmaDateFormat), time.Now().Format("2006-01-02T15:04:05"))
	transitTime := legDetails.LegTransitTime
	var cutoffs *schema.Cutoff
	var cyCutoffDate, docCutoffDate, vgmCutoffDate string
	c := &legDetails.PointFrom.CutOff
	if c.PortCutoff != nil {
		cyCutoffDate = ConvertDateFormat(&c.PortCutoff.Utc, cmaDateFormat)
	}
	if c.ShippingInstructionAcceptance != nil {
		docCutoffDate = ConvertDateFormat(&c.ShippingInstructionAcceptance.Utc, cmaDateFormat)
	}
	if c.Vgm != nil {
		vgmCutoffDate = ConvertDateFormat(&c.Vgm.Utc, cmaDateFormat)
	}

	if cyCutoffDate != "" && docCutoffDate != "" && vgmCutoffDate != "" {
		cutoffs = &schema.Cutoff{
			CyCutoffDate:  cyCutoffDate,
			DocCutoffDate: docCutoffDate,
			VgmCutoffDate: vgmCutoffDate,
		}
	}

	eventTime := &schema.Leg{
		Etd:         etd,
		Eta:         eta,
		TransitTime: transitTime,
		Cutoffs:     cutoffs,
	}

	return eventTime
}

func (csp *CmaScheduleResponse) GenerateTransport(transportDetails *Transportation) *schema.Leg {
	meanOfTransport := cases.Title(language.Und).String(strings.Replace(transportDetails.MeanOfTransport, "/", "", -1))
	vehicule := transportDetails.Vehicule
	vehicleType := transportDetails.Vehicule.VehiculeType
	vesselIMO := transportDetails.Vehicule.Reference

	var referenceType, reference string
	switch {
	case len(vesselIMO) > 0 && len(vesselIMO) < 9:
		referenceType = "IMO"
		reference = vesselIMO
	case vehicleType == "Barge":
		referenceType = "IMO"
		reference = "9"
	}

	tr := schema.Transportation{
		TransportType: schema.TransportType(meanOfTransport),
		TransportName: vehicule.VehiculeName,
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

func (csp *CmaScheduleResponse) GenerateVoyageService(voyageDetails *Voyage) *schema.Leg {
	serviceCode := voyageDetails.Service.Code
	voyage := &schema.Voyage{
		InternalVoyage: cmp.Or(voyageDetails.VoyageReference, "TBN"),
	}

	var service *schema.Service
	if serviceCode != nil {
		service = &schema.Service{ServiceCode: serviceCode}
	}

	voyageServices := &schema.Leg{
		Voyages:  voyage,
		Services: service,
	}

	return voyageServices
}

func (csp *CmaScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs) interfaces.HeaderParams {
	var specificRoutings string

	extraCondition := strings.HasPrefix(*p.Query.PointFrom, "US") && strings.HasPrefix(*p.Query.PointTo, "US")
	cmaCarrierCode := schema.InternalCodeMapping[p.Scac]
	if cmaCarrierCode == schema.InternalCodeMapping[schema.APLU] && extraCondition {
		specificRoutings = "USGovernment"
	} else {
		specificRoutings = "Commercial"
	}

	scheduleHeaders := map[string]string{
		"keyID": *p.Env.CmaToken,
	}
	scheduleParams := map[string]string{
		"shippingCompany":  cmaCarrierCode,
		"placeOfLoading":   *p.Query.PointFrom,
		"placeOfDischarge": *p.Query.PointTo,
		"searchRange":      strconv.Itoa((*p.Query.SearchRange) * 7),
		"specificRoutings": specificRoutings,
	}

	if *p.Query.StartDateType == schema.Departure {
		scheduleParams["departureDate"] = *p.Query.StartDate
	} else {
		scheduleParams["arrivalDate"] = *p.Query.StartDate
	}
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams

}
