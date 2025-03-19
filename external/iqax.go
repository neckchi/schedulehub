package external

import (
	"cmp"
	"encoding/json"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"strconv"
	"time"
)

type IqaxScheduleResponse struct {
	DataRange       DataRange        `json:"dataRange"`
	RequestRefNo    string           `json:"requestRefNo"`
	RouteGroupsList []IqaxRouteGroup `json:"routeGroupsList"`
}

type DataRange struct {
	DepartureFrom string `json:"departureFrom"`
	DepartureTo   string `json:"departureTo"`
}

type IqaxRouteGroup struct {
	Identification Identification `json:"identification"`
	Carrier        Carrier        `json:"carrier"`
	Por            Port           `json:"por"`
	Fnd            Port           `json:"fnd"`
	Route          []IqaxRoute    `json:"route"`
}

// Identification block within a route group
type Identification struct {
	DataSourceType string `json:"dataSourceType"`
	RequestRefNo   string `json:"requestRefNo"`
}

// Carrier holds carrier details
type Carrier struct {
	ID               string `json:"_id"`
	CarrierID        int    `json:"carrierID"`
	Name             string `json:"name"`
	Scac             string `json:"scac"`
	ShortName        string `json:"shortName"`
	URL              string `json:"url"`
	Enabled          bool   `json:"enabled"`
	SsmEnabled       bool   `json:"ssmEnabled"`
	SortingKey       string `json:"sortingKey"`
	UpdatedAt        string `json:"updatedAt"`
	UpdatedBy        string `json:"updatedBy"`
	AnalyticsEnabled bool   `json:"analyticsEnabled"`
	SsSearchEnabled  bool   `json:"ssSearchEnabled"`
	VsSearchEnabled  bool   `json:"vsSearchEnabled"`
	ChineseName      string `json:"chineseName"`
}

type Port struct {
	Location IqaxLocation `json:"location"`
}

type IqaxLocation struct {
	ID                string    `json:"_id"`
	Source            string    `json:"source"`
	Unlocode          string    `json:"unlocode"`
	Name              string    `json:"name"`
	UcName            string    `json:"uc_name"`
	Geo               []float64 `json:"geo"`
	ChineseName       string    `json:"chineseName"`
	SsmLastUpdateTime string    `json:"ssmLastUpdateTime"`
	LocationID        string    `json:"locationID"`
	CsID              int       `json:"csID"`
	CsCityID          int       `json:"csCityID"`
	Type              string    `json:"type"`
	FullName          string    `json:"fullName"`
	Timezone          string    `json:"timezone"`
	RefreshDateTime   string    `json:"refreshDateTime"`
}

type IqaxRoute struct {
	CsRouteID              int64             `json:"csRouteID"`
	CsPointPairID          int               `json:"csPointPairID"`
	CarrierScac            string            `json:"carrierScac"`
	Por                    IqaxRouteLocation `json:"por"`
	Fnd                    RouteFnd          `json:"fnd"`
	TransitTime            int               `json:"transitTime"`
	Direct                 bool              `json:"direct"`
	ImportHaulage          string            `json:"importHaulage"`
	ExportHaulage          string            `json:"exportHaulage"`
	TouchTime              string            `json:"touchTime"`
	Leg                    []*IqaxLeg        `json:"leg"`
	TransportSummary       string            `json:"transportSummary"`
	DefaultCutoff          DefaultCutoff     `json:"defaultCutoff"`
	IsPossibleDirect       bool              `json:"isPossibleDirect"`
	IsUncertainTransitTime bool              `json:"isUncertainTransitTime"`
}

type IqaxRouteLocation struct {
	Location struct {
		LocationID string `json:"locationID"`
		Name       string `json:"name"`
		Unlocode   string `json:"unlocode"`
		CsID       int    `json:"csID"`
		CsCityID   int    `json:"csCityId"`
		Timezone   string `json:"timezone"`
	} `json:"location"`
	Etd *string `json:"etd"`
}

// RouteFnd is the structure of "fnd" inside each Route
type RouteFnd struct {
	Location struct {
		LocationID string `json:"locationID"`
		Name       string `json:"name"`
		Unlocode   string `json:"unlocode"`
		CsID       int    `json:"csID"`
		CsCityID   int    `json:"csCityId"`
		Timezone   string `json:"timezone"`
		Facility   struct {
			Name string `json:"name"`
			Code string `json:"code"`
			ID   int    `json:"id"`
			Type string `json:"type"`
		} `json:"facility"`
	} `json:"location"`
	Eta                 string `json:"eta"`
	ArrivalTimeLocation struct {
		LocationID string `json:"locationID"`
		Name       string `json:"name"`
		Unlocode   string `json:"unlocode"`
		CsID       int    `json:"csID"`
		CsCityID   int    `json:"csCityId"`
		Timezone   string `json:"timezone"`
		Facility   struct {
			Name string `json:"name"`
			Code string `json:"code"`
			ID   int    `json:"id"`
			Type string `json:"type"`
		} `json:"facility"`
	} `json:"arrivalTimeLocation"`
}

// Leg describes each "leg" segment of the route
type IqaxLeg struct {
	FromPoint struct {
		Location struct {
			LocationID string `json:"locationID"`
			Name       string `json:"name"`
			Unlocode   string `json:"unlocode"`
			CsID       int    `json:"csID"`
			CsCityID   int    `json:"csCityId"`
			Timezone   string `json:"timezone"`
			Facility   struct {
				Name string `json:"name"`
				Code string `json:"code"`
				ID   int    `json:"id"`
				Type string `json:"type"`
			} `json:"facility"`
		} `json:"location"`
		DefaultCutoff string `json:"defaultCutoff"`
		Etd           string `json:"etd"`
		GmtEtd        string `json:"gmtEtd"`
	} `json:"fromPoint"`
	ToPoint struct {
		Location struct {
			LocationID string `json:"locationID"`
			Name       string `json:"name"`
			Unlocode   string `json:"unlocode"`
			CsID       int    `json:"csID"`
			CsCityID   int    `json:"csCityId"`
			Timezone   string `json:"timezone"`
			Facility   struct {
				Name string `json:"name"`
				Code string `json:"code"`
				ID   int    `json:"id"`
				Type string `json:"type"`
			} `json:"facility"`
		} `json:"location"`
		Eta    string `json:"eta"`
		GmtEta string `json:"gmtEta"`
	} `json:"toPoint"`
	TransportMode string `json:"transportMode"`
	Service       struct {
		ServiceID int     `json:"serviceID"`
		Code      *string `json:"code"`
		Name      *string `json:"name"`
	} `json:"service"`
	Vessel struct {
		VesselGID string `json:"vesselGID"`
		Name      string `json:"name"`
		Code      string `json:"code"`
	} `json:"vessel"`
	TransitTime          int    `json:"transitTime"`
	InternalVoyageNumber string `json:"internalVoyageNumber"`
	ImoNumber            int    `json:"imoNumber,omitempty"`
	ExternalVoyageNumber string `json:"externalVoyageNumber,omitempty"`
}

// DefaultCutoff holds cutoffTime
type DefaultCutoff struct {
	CutoffTime time.Time `json:"cutoffTime"`
}

type FirstEtdLastEta struct{ firstEtd, lastEta string }

const iqaxDateFormat string = "2006-01-02T15:04:05.000Z"

func (isp *IqaxScheduleResponse) GenerateSchedule(responseJson []byte) ([]*schema.Schedule, error) {
	var iqaxScheduleData IqaxScheduleResponse
	err := json.Unmarshal(responseJson, &iqaxScheduleData)
	if err != nil {
		return nil, err
	}
	var iqaxScheduleList = make([]*schema.Schedule, 0, len(iqaxScheduleData.RouteGroupsList))
	for _, scheduleList := range iqaxScheduleData.RouteGroupsList {
		for _, schedule := range scheduleList.Route {
			scheduleDate := FirstEtdLastEta{firstEtd: ConvertDateFormat(schedule.Por.Etd, iqaxDateFormat), lastEta: ConvertDateFormat(&schedule.Fnd.Eta, iqaxDateFormat)}
			scheduleResult := &schema.Schedule{
				Scac:          schedule.CarrierScac,
				PointFrom:     schedule.Por.Location.Unlocode,
				PointTo:       schedule.Fnd.Location.Unlocode,
				Etd:           scheduleDate.firstEtd,
				Eta:           scheduleDate.lastEta,
				TransitTime:   schedule.TransitTime,
				Transshipment: !schedule.Direct,
				Legs:          isp.GenerateScheduleLeg(schedule.Leg, scheduleDate),
			}
			iqaxScheduleList = append(iqaxScheduleList, scheduleResult)
		}
	}
	return iqaxScheduleList, nil
}

func (isp *IqaxScheduleResponse) GenerateScheduleLeg(legResponse []*IqaxLeg, scheduleDate FirstEtdLastEta) []*schema.Leg {
	var iqaxLegList = make([]*schema.Leg, 0, len(legResponse))
	for index, leg := range legResponse {
		pointBase := isp.GenerateLegPoints(leg)
		origin := pointBase.PointFrom
		destination := pointBase.PointTo
		if origin.LocationCode != destination.LocationCode {
			eventDate := isp.GenerateEventDate(index, scheduleDate, leg)
			voyageService := isp.GenerateVoyageService(leg)
			legInstance := &schema.Leg{
				PointFrom:       origin,
				PointTo:         destination,
				Etd:             eventDate.Etd,
				Eta:             eventDate.Eta,
				TransitTime:     eventDate.TransitTime,
				Cutoffs:         eventDate.Cutoffs,
				Transportations: isp.GenerateTransport(leg).Transportations,
				Voyages:         voyageService.Voyages,
				Services:        voyageService.Services,
			}
			iqaxLegList = append(iqaxLegList, legInstance)
		}
	}
	return iqaxLegList
}

func (isp *IqaxScheduleResponse) GenerateLegPoints(legResponse *IqaxLeg) *schema.Leg {
	pointFrom := schema.PointBase{
		LocationName: legResponse.FromPoint.Location.Name,
		LocationCode: legResponse.FromPoint.Location.Unlocode,
		TerminalName: legResponse.FromPoint.Location.Facility.Name,
		TerminalCode: legResponse.FromPoint.Location.Facility.Code,
	}

	pointTo := schema.PointBase{
		LocationName: legResponse.ToPoint.Location.Name,
		LocationCode: legResponse.ToPoint.Location.Unlocode,
		TerminalName: legResponse.ToPoint.Location.Facility.Name,
		TerminalCode: legResponse.ToPoint.Location.Facility.Code,
	}

	portPairs := &schema.Leg{
		PointFrom: pointFrom,
		PointTo:   pointTo,
	}
	return portPairs
}

func (isp *IqaxScheduleResponse) GenerateEventDate(index int, scheduleDate FirstEtdLastEta, legResponse *IqaxLeg) *schema.Leg {
	var cutoffs *schema.Cutoff
	if legResponse.FromPoint.DefaultCutoff != "" {
		cutoffs = &schema.Cutoff{
			CyCutoffDate: ConvertDateFormat(&legResponse.FromPoint.DefaultCutoff, iqaxDateFormat),
		}
	}
	var etd, eta string
	const parseDateFormat string = "2006-01-02T15:04:05"
	if index == 1 {
		checkEtd := cmp.Or(ConvertDateFormat(&legResponse.FromPoint.Etd, iqaxDateFormat), scheduleDate.firstEtd)
		if legResponse.Vessel.Name == "TRUCK" {
			parseEtd, _ := time.Parse(parseDateFormat, checkEtd)
			etd = parseEtd.AddDate(0, 0, -legResponse.TransitTime).Format(parseDateFormat)
		} else {
			etd = checkEtd
			parseEtd, _ := time.Parse(parseDateFormat, etd)
			eta = cmp.Or(ConvertDateFormat(&legResponse.ToPoint.Eta, iqaxDateFormat), parseEtd.AddDate(0, 0, legResponse.TransitTime).Format(parseDateFormat))
		}
	} else {
		eta = cmp.Or(ConvertDateFormat(&legResponse.ToPoint.Eta, iqaxDateFormat), scheduleDate.lastEta)
		parseEta, _ := time.Parse(parseDateFormat, eta)
		etd = cmp.Or(ConvertDateFormat(&legResponse.FromPoint.Etd, iqaxDateFormat), parseEta.AddDate(0, 0, -legResponse.TransitTime).Format(parseDateFormat))
	}

	eventTime := &schema.Leg{
		Etd:         etd,
		Eta:         eta,
		TransitTime: legResponse.TransitTime,
		Cutoffs:     cutoffs,
	}

	return eventTime
}

func (isp *IqaxScheduleResponse) GenerateTransport(legResponse *IqaxLeg) *schema.Leg {
	var transportName, imoCode, refType, ref string
	switch legResponse.Vessel.Name {
	case "---":
		transportName = "TBA"
	default:
		transportName = legResponse.Vessel.Name
	}
	switch legResponse.ImoNumber {
	case 0:
		imoCode = ""

	default:
		convertIntToString := strconv.Itoa(legResponse.ImoNumber)
		imoCode = convertIntToString
	}
	legTransportMode := cases.Title(language.Und).String(legResponse.TransportMode)
	tbnFeeder := imoCode == "" && transportName == "TBA" && (legTransportMode == "Feeder" || legTransportMode == "Barge")
	dummyVehicle := (!tbnFeeder && imoCode == "") || imoCode == "9999999"
	if tbnFeeder {
		refType = "IMO"
		ref = "1"
	} else if dummyVehicle {
		refType = ""
		ref = ""
	} else {
		refType = "IMO"
		ref = imoCode
	}

	tr := schema.Transportation{
		TransportType: schema.TransportType(cases.Title(language.Und).String(legResponse.TransportMode)),
		TransportName: transportName,
		ReferenceType: refType,
		Reference:     ref,
	}
	err := tr.MapTransport()
	if err != nil {
		log.Panic(err)
	}
	transportDetails := &schema.Leg{
		Transportations: tr,
	}
	return transportDetails
}

func (isp *IqaxScheduleResponse) GenerateVoyageService(legResponse *IqaxLeg) *schema.Leg {
	var internalVoyage, externalVoyage string
	internalVoyage = cmp.Or(legResponse.InternalVoyageNumber, "TBN")
	externalVoyage = cmp.Or(legResponse.ExternalVoyageNumber, "TBN")
	voyage := &schema.Voyage{
		InternalVoyage: internalVoyage,
		ExternalVoyage: externalVoyage,
	}
	var service *schema.Service
	serviceCode := legResponse.Service.Code
	if serviceCode != nil {
		service = &schema.Service{ServiceCode: serviceCode, ServiceName: legResponse.Service.Name}
	}

	voyageServices := &schema.Leg{
		Voyages:  voyage,
		Services: service,
	}

	return voyageServices
}

func (isp *IqaxScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs) interfaces.HeaderParams {
	scheduleHeaders := map[string]string{"appKey": *p.Env.IqaxToken}
	scheduleParams := map[string]string{"porID": *p.Query.PointFrom, "fndID": *p.Query.PointTo, "searchDuration": strconv.Itoa(*p.Query.SearchRange)}

	if *p.Query.StartDateType == schema.Departure {
		scheduleParams["departureFrom"] = *p.Query.StartDate
	} else {
		scheduleParams["arrivalFrom"] = *p.Query.StartDate
	}
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams
}
