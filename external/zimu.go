package external

import (
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	"time"
)

type ZimScheduleResponse struct {
	Response response `json:"response"`
}

type response struct {
	Routes []route `json:"routes"`
}

type route struct {
	DeparturePort string  `json:"departurePort"`
	ArrivalPort   string  `json:"arrivalPort"`
	DepartureDate string  `json:"departureDate"`
	ArrivalDate   string  `json:"arrivalDate"`
	TransitTime   float64 `json:"transitTime"`
	RouteLegs     []*zleg `json:"routeLegs"`
}

type zleg struct {
	DeparturePort        string `json:"departurePort"`
	DeparturePortName    string `json:"departurePortName"`
	ArrivalPort          string `json:"arrivalPort"`
	ArrivalPortName      string `json:"arrivalPortName"`
	DepartureDate        string `json:"departureDate"`
	ArrivalDate          string `json:"arrivalDate"`
	VesselName           string `json:"vesselName"`
	LloydsCode           string `json:"lloydsCode"`
	Voyage               string `json:"voyage"`
	ConsortSailingNumber string `json:"consortSailingNumber"`
	Leg                  string `json:"leg"`
	Line                 string `json:"line"`
	ContainerClosingDate string `json:"containerClosingDate"`
	DocClosingDate       string `json:"docClosingDate"`
	VgmClosingDate       string `json:"vgmClosingDate"`
}

const zimDateFormat string = "2006-01-02T15:04:05.000-07:00"

func (zs *ZimScheduleResponse) GenerateSchedule(responseJson []byte) ([]*schema.Schedule, error) {
	var zimScheduleData ZimScheduleResponse
	err := json.Unmarshal(responseJson, &zimScheduleData)
	if err != nil {
		return nil, err
	}
	var zimScheduleList = make([]*schema.Schedule, 0, len(zimScheduleData.Response.Routes))
	for _, route := range zimScheduleData.Response.Routes {
		scheduleResult := &schema.Schedule{
			Scac:          "ZIMU",
			PointFrom:     route.DeparturePort,
			PointTo:       route.ArrivalPort,
			Etd:           ConvertDateFormat(&route.DepartureDate, zimDateFormat),
			Eta:           ConvertDateFormat(&route.ArrivalDate, zimDateFormat),
			TransitTime:   int(route.TransitTime),
			Transshipment: len(route.RouteLegs) > 1,
			Legs:          zs.GenerateScheduleLeg(route.RouteLegs),
		}
		zimScheduleList = append(zimScheduleList, scheduleResult)
	}
	return zimScheduleList, nil
}

func (zs *ZimScheduleResponse) GenerateScheduleLeg(legResponse []*zleg) []*schema.Leg {
	var zimLegList = make([]*schema.Leg, 0, len(legResponse))
	for _, leg := range legResponse {
		pointBase := zs.GenerateLegPoints(leg)
		eventDate := zs.GenerateEventDate(leg)
		voyageService := zs.GenerateVoyageService(leg)
		legInstance := &schema.Leg{
			PointFrom:       pointBase.PointFrom,
			PointTo:         pointBase.PointTo,
			Etd:             eventDate.Etd,
			Eta:             eventDate.Eta,
			TransitTime:     eventDate.TransitTime,
			Cutoffs:         eventDate.Cutoffs,
			Transportations: zs.GenerateTransport(leg).Transportations,
			Voyages:         voyageService.Voyages,
			Services:        voyageService.Services,
		}
		zimLegList = append(zimLegList, legInstance)
	}
	return zimLegList
}

func (zs *ZimScheduleResponse) GenerateLegPoints(legDetails *zleg) *schema.Leg {
	pointFrom := schema.PointBase{
		LocationName: legDetails.DeparturePortName,
		LocationCode: legDetails.DeparturePort,
	}

	pointTo := schema.PointBase{
		LocationName: legDetails.ArrivalPortName,
		LocationCode: legDetails.ArrivalPort,
	}

	portPairs := &schema.Leg{
		PointFrom: pointFrom,
		PointTo:   pointTo,
	}
	return portPairs
}

func (zs *ZimScheduleResponse) GenerateEventDate(legDetails *zleg) *schema.Leg {
	etd := ConvertDateFormat(&legDetails.DepartureDate, zimDateFormat)
	eta := ConvertDateFormat(&legDetails.ArrivalDate, zimDateFormat)
	transitTime := CalculateTransitTime(&etd, &eta)
	cyCutoffDate := ConvertDateFormat(&legDetails.ContainerClosingDate, zimDateFormat)
	docCutoffDate := ConvertDateFormat(&legDetails.DocClosingDate, zimDateFormat)
	vgmCutoffDate := ConvertDateFormat(&legDetails.VgmClosingDate, zimDateFormat)

	var cutoffs *schema.Cutoff
	if cyCutoffDate != "" || docCutoffDate != "" || vgmCutoffDate != "" {
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

func (zs *ZimScheduleResponse) GenerateTransport(legDetails *zleg) *schema.Leg {

	var mapIMO = func(legIMO, vesselName, line, transport string) string {
		switch {
		case vesselName != "TO BE NAMED" && transport != "Truck":
			return legIMO
		case (line == "UNK" && transport != "Truck") || transport == "Feeder":
			return "9"
		case transport == "Truck":
			return "3"
		default:
			return "1"
		}
	}
	transportName := legDetails.VesselName
	transportType, _ := getTransportType(transportName)
	imoCode := legDetails.LloydsCode
	legLine := legDetails.Line
	tr := schema.Transportation{
		TransportType: schema.TransportType(transportType),
		TransportName: transportName,
		ReferenceType: "IMO",
		Reference:     mapIMO(imoCode, transportName, legLine, transportType),
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

func (zs *ZimScheduleResponse) GenerateVoyageService(legDetails *zleg) *schema.Leg {
	var internalVoyage string
	voyageNumber := legDetails.Voyage
	direction := legDetails.Leg
	externalVoyage := legDetails.ConsortSailingNumber
	legLine := legDetails.Line
	if voyageNumber != "" {
		internalVoyage = voyageNumber + direction
	} else {
		internalVoyage = "TBN"
	}
	voyage := &schema.Voyage{
		InternalVoyage: internalVoyage,
		ExternalVoyage: externalVoyage,
	}

	var service *schema.Service
	if voyageNumber != "" {
		service = &schema.Service{
			ServiceCode: &legLine,
		}
	}

	voyageServices := &schema.Leg{
		Voyages:  voyage,
		Services: service,
	}

	return voyageServices
}

func (zs *ZimScheduleResponse) TokenHeaderParams(e *env.Manager) interfaces.HeaderParams {

	tokenHeaders := map[string]string{
		"Ocp-Apim-Subscription-Key": *e.ZimToken,
		"Content-Type":              "application/x-www-form-urlencoded",
	}
	tokenParams := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     *e.ZimClient,
		"client_secret": *e.ZimSecret,
		"scope":         "Vessel Schedule",
	}
	headerParams := interfaces.HeaderParams{Headers: tokenHeaders, Params: tokenParams}
	return headerParams
}

func (zs *ZimScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs) interfaces.HeaderParams {
	parsedTime, _ := time.Parse("2006-01-02", *p.Query.StartDate)
	scheduleHeaders := map[string]string{
		"Ocp-Apim-Subscription-Key": *p.Env.ZimToken,
		"Authorization":             fmt.Sprintf("Bearer %s", p.Token.Data["access_token"].(string)),
		"Accept":                    "application/json",
	}
	scheduleParams := map[string]string{
		"originCode":               *p.Query.PointFrom,
		"destCode":                 *p.Query.PointTo,
		"fromDate":                 *p.Query.StartDate,
		"toDate":                   parsedTime.AddDate(0, 0, *p.Query.SearchRange*7).Format("2006-01-02"),
		"sortByDepartureOrArrival": string(*p.Query.StartDateType),
	}
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams
}
