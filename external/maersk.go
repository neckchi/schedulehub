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

// Location represents common location fields used across different facility types
type MaerskLocation struct {
	CarrierCityGeoID   string `json:"carrierCityGeoID"`
	CityName           string `json:"cityName"`
	CarrierSiteGeoID   string `json:"carrierSiteGeoID"`
	LocationName       string `json:"locationName"`
	CountryCode        string `json:"countryCode"`
	LocationType       string `json:"locationType"`
	UNLocationCode     string `json:"UNLocationCode"`
	SiteUNLocationCode string `json:"siteUNLocationCode"`
	CityUNLocationCode string `json:"cityUNLocationCode"`
}

// DeliveryLocation extends Location with additional delivery-specific fields
type MaerskDeliveryLocation struct {
	MaerskLocation
	UNRegionCode string `json:"UNRegionCode"`
}

// Vessel represents vessel information
type MaerskVessel struct {
	VesselIMONumber   string `json:"vesselIMONumber"`
	CarrierVesselCode string `json:"carrierVesselCode"`
	VesselName        string `json:"vesselName"`
}

// Transport contains transportation details
type Transport struct {
	TransportMode                string       `json:"transportMode"`
	Vessel                       MaerskVessel `json:"vessel"`
	CarrierTradeLaneName         string       `json:"carrierTradeLaneName"`
	CarrierDepartureVoyageNumber string       `json:"carrierDepartureVoyageNumber"`
	CarrierServiceCode           string       `json:"carrierServiceCode"`
	CarrierServiceName           string       `json:"carrierServiceName"`
	LinkDirection                string       `json:"linkDirection"`
	CarrierCode                  string       `json:"carrierCode"`
	RoutingType                  string       `json:"routingType"`
}

// Facilities represents location facilities
type Facilities struct {
	CollectionOrigin    MaerskLocation         `json:"collectionOrigin"`
	DeliveryDestination MaerskDeliveryLocation `json:"deliveryDestination"`
}

// TransportLegFacilities represents facilities for transport legs
type TransportLegFacilities struct {
	StartLocation MaerskLocation         `json:"startLocation"`
	EndLocation   MaerskDeliveryLocation `json:"endLocation"`
}

// TransportLeg represents a single leg of transportation
type TransportLeg struct {
	DepartureDateTime string                 `json:"departureDateTime"`
	ArrivalDateTime   string                 `json:"arrivalDateTime"`
	Facilities        TransportLegFacilities `json:"facilities"`
	Transport         Transport              `json:"transport"`
}

// TransportSchedule represents a complete transport schedule
type TransportSchedule struct {
	DepartureDateTime    string          `json:"departureDateTime"`
	ArrivalDateTime      string          `json:"arrivalDateTime"`
	Facilities           Facilities      `json:"facilities"`
	FirstDepartureVessel MaerskVessel    `json:"firstDepartureVessel"`
	TransitTime          string          `json:"transitTime"`
	TransportLegs        []*TransportLeg `json:"transportLegs"`
}

// OceanProduct represents a single ocean product
type OceanProduct struct {
	CarrierProductID          string              `json:"carrierProductId"`
	CarrierProductSequenceID  string              `json:"carrierProductSequenceId"`
	NumberOfProductLinks      string              `json:"numberOfProductLinks"`
	TransportSchedules        []TransportSchedule `json:"transportSchedules"`
	VesselOperatorCarrierCode string              `json:"vesselOperatorCarrierCode"`
}

type MaerskScheduleResponse struct {
	OceanProducts []OceanProduct `json:"oceanProducts"`
}

func (maeusp *MaerskScheduleResponse) GenerateSchedule(responseJson []byte) ([]*schema.Schedule, error) {
	var maerskScheduleData MaerskScheduleResponse
	err := json.Unmarshal(responseJson, &maerskScheduleData)
	if err != nil {
		return nil, err
	}
	var maerskScheduleList = make([]*schema.Schedule, 0, len(maerskScheduleData.OceanProducts))
	for _, product := range maerskScheduleData.OceanProducts {
		for _, schedule := range product.TransportSchedules {
			tt, _ := strconv.Atoi(schedule.TransitTime)
			scheduleResult := &schema.Schedule{
				Scac:          product.VesselOperatorCarrierCode,
				PointFrom:     cmp.Or(schedule.Facilities.CollectionOrigin.CityUNLocationCode, schedule.Facilities.CollectionOrigin.CityUNLocationCode),
				PointTo:       cmp.Or(schedule.Facilities.DeliveryDestination.CityUNLocationCode, schedule.Facilities.DeliveryDestination.CityUNLocationCode),
				Etd:           schedule.DepartureDateTime,
				Eta:           schedule.ArrivalDateTime,
				TransitTime:   int(math.Floor((float64(tt) / 1400))),
				Transshipment: len(schedule.TransportLegs) > 1,
				Legs:          maeusp.GenerateScheduleLeg(schedule.TransportLegs),
			}
			maerskScheduleList = append(maerskScheduleList, scheduleResult)
		}

	}
	return maerskScheduleList, nil
}

func (maeusp *MaerskScheduleResponse) GenerateScheduleLeg(legResponse []*TransportLeg) []*schema.Leg {
	var maerskLegList = make([]*schema.Leg, 0, len(legResponse))
	for _, leg := range legResponse {
		pointBase := maeusp.GenerateLegPoints(leg)
		eventDate := maeusp.GenerateEventDate(leg)
		voyageService := maeusp.GenerateVoyageService(leg)
		legInstance := &schema.Leg{
			PointFrom:       pointBase.PointFrom,
			PointTo:         pointBase.PointTo,
			Etd:             eventDate.Etd,
			Eta:             eventDate.Eta,
			TransitTime:     eventDate.TransitTime,
			Transportations: maeusp.GenerateTransport(leg).Transportations,
			Voyages:         voyageService.Voyages,
			Services:        voyageService.Services,
		}
		maerskLegList = append(maerskLegList, legInstance)
	}
	return maerskLegList
}

func (maeusp *MaerskScheduleResponse) GenerateLegPoints(legDetails *TransportLeg) *schema.Leg {
	pointFrom := schema.PointBase{
		LocationName: legDetails.Facilities.StartLocation.CityName,
		LocationCode: cmp.Or(legDetails.Facilities.StartLocation.CityUNLocationCode, legDetails.Facilities.StartLocation.SiteUNLocationCode),
		TerminalName: legDetails.Facilities.StartLocation.LocationName,
	}

	pointTo := schema.PointBase{
		LocationName: legDetails.Facilities.EndLocation.CityName,
		LocationCode: cmp.Or(legDetails.Facilities.EndLocation.CityUNLocationCode, legDetails.Facilities.EndLocation.SiteUNLocationCode),
		TerminalName: legDetails.Facilities.EndLocation.LocationName,
	}

	portPairs := &schema.Leg{
		PointFrom: pointFrom,
		PointTo:   pointTo,
	}
	return portPairs
}

func (maeusp *MaerskScheduleResponse) GenerateEventDate(legDetails *TransportLeg) *schema.Leg {
	etd := legDetails.DepartureDateTime
	eta := legDetails.ArrivalDateTime
	transitTime := CalculateTransitTime(&etd, &eta)

	eventTime := &schema.Leg{
		Etd:         etd,
		Eta:         eta,
		TransitTime: transitTime,
	}

	return eventTime
}

func (maeusp *MaerskScheduleResponse) GenerateTransport(legDetails *TransportLeg) *schema.Leg {

	transportType, _ := getTransportType(legDetails.Transport.TransportMode)
	transportName := legDetails.Transport.Vessel.VesselName
	imoExist := legDetails.Transport.Vessel.VesselIMONumber != ""
	var imo string
	if imoExist {
		switch legDetails.Transport.Vessel.VesselIMONumber {
		case "999999":
			imo = "1"
		default:
			imo = legDetails.Transport.Vessel.VesselIMONumber
		}
	}
	tr := schema.Transportation{
		TransportType: schema.TransportType(transportType),
		TransportName: transportName,
		ReferenceType: "IMO",
		Reference:     imo,
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

func (maeusp *MaerskScheduleResponse) GenerateVoyageService(legDetails *TransportLeg) *schema.Leg {
	sc := cmp.Or(legDetails.Transport.CarrierServiceName, legDetails.Transport.CarrierServiceCode)
	vn := legDetails.Transport.CarrierDepartureVoyageNumber
	voyage := &schema.Voyage{
		InternalVoyage: cmp.Or(vn, "TBN"),
	}

	var service *schema.Service
	if sc != "" {
		service = &schema.Service{
			ServiceCode: &sc,
			ServiceName: &sc,
		}
	}
	voyageServices := &schema.Leg{
		Voyages:  voyage,
		Services: service,
	}

	return voyageServices
}

func (maeusp *MaerskScheduleResponse) LocationHeaderParams(e *env.Manager, port string) interfaces.HeaderParams {
	locationHeaders := map[string]string{
		"Consumer-Key": *e.MaerskToken2,
	}
	locationParams := map[string]string{
		"locationType":   "CITY",
		"UNLocationCode": port,
	}
	headerParams := interfaces.HeaderParams{Headers: locationHeaders, Params: locationParams}
	return headerParams
}

func (maeusp *MaerskScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs) interfaces.HeaderParams {
	scheduleHeaders := map[string]string{
		"Consumer-Key": *p.Env.MaerskToken,
	}
	scheduleParams := map[string]string{
		"collectionOriginCountryCode":       p.Origin[0]["countryCode"].(string),
		"collectionOriginCityName":          p.Origin[0]["cityName"].(string),
		"collectionOriginUNLocationCode":    *p.Query.PointFrom,
		"deliveryDestinationCountryCode":    p.Destination[0]["countryCode"].(string),
		"deliveryDestinationCityName":       p.Destination[0]["cityName"].(string),
		"deliveryDestinationUNLocationCode": *p.Query.PointTo,
		"dateRange":                         fmt.Sprintf("P%sW", strconv.Itoa(*p.Query.SearchRange)),
		"startDate":                         *p.Query.StartDate,
		"vesselOperatorCarrierCode":         string(p.Scac),
	}

	if *p.Query.StartDateType == schema.Departure {
		scheduleParams["startDateType"] = "D"
	} else {
		scheduleParams["startDateType"] = "A"
	}

	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}

	return headerParams
}
