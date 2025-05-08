package carrier_vessel_schedule

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	"time"
)

type MaerskVesselSchedule struct {
	Vessel      *MaerskVessel      `json:"vessel"`
	VesselCalls []MaerskVesselCall `json:"vesselCalls"`
}

// Vessel represents vessel details
type MaerskVessel struct {
	VesselIMONumber   string `json:"vesselIMONumber"`
	CarrierVesselCode string `json:"carrierVesselCode"`
	VesselName        string `json:"vesselName"`
	VesselFlagCode    string `json:"vesselFlagCode"`
	VesselCallSign    string `json:"vesselCallSign"`
}

// VesselCall represents a single vessel call
type MaerskVesselCall struct {
	Facility      MaerskFacility       `json:"facility"`
	Transport     MaerskTransport      `json:"transport"`
	CallSchedules []MaerskCallSchedule `json:"callSchedules"`
}

// Facility represents facility details
type MaerskFacility struct {
	LocationType         string `json:"locationType"`
	LocationName         string `json:"locationName"`
	CarrierTerminalCode  string `json:"carrierTerminalCode"`
	CarrierTerminalGeoID string `json:"carrierTerminalGeoID"`
	CountryCode          string `json:"countryCode"`
	CountryName          string `json:"countryName"`
	CityName             string `json:"cityName"`
	PortName             string `json:"portName"`
	CarrierCityGeoID     string `json:"carrierCityGeoID"`
	UNLocationCode       string `json:"UNLocationCode"`
	UNRegionCode         string `json:"UNRegionCode"`
}

// Transport represents transport details
type MaerskTransport struct {
	InboundService  MaerskInboundService  `json:"inboundService"`
	OutboundService MaerskOutboundService `json:"outboundService"`
}

// InboundService represents inbound service details
type MaerskInboundService struct {
	CarrierVoyageNumber string `json:"carrierVoyageNumber"`
	CarrierServiceCode  string `json:"carrierServiceCode"`
	CarrierServiceName  string `json:"carrierServiceName"`
}

// OutboundService represents outbound service details
type MaerskOutboundService struct {
	CarrierVoyageNumber string `json:"carrierVoyageNumber"`
	CarrierServiceCode  string `json:"carrierServiceCode"`
	CarrierServiceName  string `json:"carrierServiceName"`
}

// CallSchedule represents a call schedule
type MaerskCallSchedule struct {
	TransportEventTypeCode string `json:"transportEventTypeCode"`
	EventClassifierCode    string `json:"eventClassifierCode"`
	ClassifierDateTime     string `json:"classifierDateTime"`
}

var scac schema.CarrierCode
var imo string

var maeuEventType = map[string]string{
	"ARRI": "Unloading",
	"DEPA": "Loading",
}

func (mvs *MaerskVesselSchedule) ScheduleHeaderParams(p *interfaces.ScheduleArgs[*schema.QueryParamsForVesselVoyage]) interfaces.HeaderParams {
	var calculateStartDate = func(startDate string, dateRange int) string {
		date, _ := time.Parse("2006-01-02", startDate)
		endDate := date.AddDate(0, 0, -dateRange)
		return endDate.Format("2006-01-02")
	}
	scheduleHeaders := map[string]string{
		"Consumer-Key": *p.Env.MaerskToken,
	}
	scheduleParams := map[string]string{
		"vesselIMONumber": p.Query.VesselIMO,
		"carrierCodes":    string(p.Scac),
		"startDate":       calculateStartDate(p.Query.StartDate, p.Query.DateRange),
		"dateRange":       fmt.Sprintf("P%sW", "12"),
	}
	scac = p.Scac
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams
}

func (mvs *MaerskVesselSchedule) GenerateSchedule(responseJson []byte) (*schema.MasterVesselSchedule, error) {
	var maerskVesselSchedule MaerskVesselSchedule
	err := json.Unmarshal(responseJson, &maerskVesselSchedule)
	if err != nil {
		return nil, err
	}
	if maerskVesselSchedule.Vessel == nil || len(maerskVesselSchedule.VesselCalls) == 0 {
		return nil, errors.New("MaerskVesselScheduleResponse is empty")
	}
	imo = maerskVesselSchedule.Vessel.VesselIMONumber
	mvsResult := &schema.MasterVesselSchedule{
		Scac:     string(scac),
		Voyage:   maerskVesselSchedule.VesselCalls[0].Transport.InboundService.CarrierVoyageNumber,
		Vessel:   &schema.VesselDetails{VesselName: maerskVesselSchedule.Vessel.VesselName, Imo: imo},
		Services: &schema.Services{ServiceCode: maerskVesselSchedule.VesselCalls[0].Transport.InboundService.CarrierServiceName},
		Calls:    mvs.GenerateVesselCalls(maerskVesselSchedule.VesselCalls),
	}
	return mvsResult, nil
}

func (mvs *MaerskVesselSchedule) GenerateVesselCalls(vesselCalls []MaerskVesselCall) []schema.PortCalls {
	var countPortCall int
	var maerskPortCalls = make([]schema.PortCalls, 0, len(vesselCalls))
	for _, portCalls := range vesselCalls {
		var getVoyageNumberAndDirection = func(call MaerskCallSchedule) (string, string, string) {
			voyageDirection := map[string]string{
				"W": "WBO",
				"E": "EBO",
				"N": "NBO",
				"S": "SBO",
			}

			if call.TransportEventTypeCode == "ARRI" {
				inboundService := portCalls.Transport.InboundService
				return inboundService.CarrierVoyageNumber,
					cmp.Or(voyageDirection[inboundService.CarrierVoyageNumber[len(inboundService.CarrierVoyageNumber)-1:]], "UNK"),
					inboundService.CarrierServiceName
			}
			outboundService := portCalls.Transport.OutboundService
			return outboundService.CarrierVoyageNumber,
				cmp.Or(voyageDirection[outboundService.CarrierVoyageNumber[len(outboundService.CarrierVoyageNumber)-1:]], "UNK"),
				outboundService.CarrierServiceName
		}
		for _, scheduleCalls := range portCalls.CallSchedules {
			var getEventDate = func(eventDateType string) string {
				switch scheduleCalls.EventClassifierCode == eventDateType {
				case true:
					return scheduleCalls.ClassifierDateTime
				default:
					return ""
				}
			}

			countPortCall += 1
			voyageNum, voyageDirection, serviceCode := getVoyageNumberAndDirection(scheduleCalls)
			portCallsResult := schema.PortCalls{
				Seq:       countPortCall,
				Key:       imo + voyageNum + serviceCode,
				Bound:     voyageDirection,
				Voyage:    voyageNum,
				PortEvent: maeuEventType[scheduleCalls.TransportEventTypeCode],
				Service:   &schema.Services{ServiceCode: serviceCode},
				Port: &schema.Port{
					PortCode:     portCalls.Facility.UNLocationCode,
					PortName:     portCalls.Facility.PortName,
					TerminalName: portCalls.Facility.LocationName,
					TerminalCode: portCalls.Facility.CarrierTerminalCode},
				EstimatedEventDate: getEventDate("EST"),
				ActualEventDate:    getEventDate("ACT"),
			}
			maerskPortCalls = append(maerskPortCalls, portCallsResult)
		}
	}
	return maerskPortCalls
}
