package carrier_p2p_schedule

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/neckchi/schedulehub/external"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	"time"
)

type MscScheduleResponse struct {
	MSCSchedule MscSchedules `json:"MSCSchedule"`
}

type MscSchedules struct {
	Transactions []MscTransaction `json:"Transactions"`
}

type MscTransaction struct {
	Schedules      []*MscSchedule `json:"Schedules"`
	SeqNoSpecified bool           `json:"SeqNoSpecified"`
}

type MscSchedule struct {
	Voyages                      []MscVoyage `json:"Voyages"`
	TransportationMeansCode      string      `json:"TransportationMeansCode"`
	TransportationMeansName      string      `json:"TransportationMeansName"`
	IMONumber                    string      `json:"IMONumber"`
	Carrier                      MscCarrier  `json:"Carrier"`
	Service                      *MscService `json:"Service"`
	Calls                        []MscCall   `json:"Calls"`
	SeqNo                        int         `json:"SeqNo"`
	SeqNoSpecified               bool        `json:"SeqNoSpecified"`
	TransportationMeansBuiltYear int         `json:"TransportationMeansBuiltYear"`
	TransportationMeansFlag      string      `json:"TransportationMeansFlag"`
}

type MscVoyage struct {
	Type           string `json:"Type"`
	Description    string `json:"Description"`
	SeqNo          int    `json:"SeqNo"`
	SeqNoSpecified bool   `json:"SeqNoSpecified"`
}

type MscCarrier struct {
	Code string `json:"Code"`
}

type MscService struct {
	Description string `json:"Description"`
}

type MscCall struct {
	Type                 string        `json:"Type"`
	Code                 string        `json:"Code"`
	Name                 string        `json:"Name"`
	EHF                  MscEHF        `json:"EHF"`
	CallDates            []MscCallDate `json:"CallDates"`
	DepartureEHFSMDGCode string        `json:"DepartureEHFSMDGCode,omitempty"`
	SeqNo                int           `json:"SeqNo"`
	SeqNoSpecified       bool          `json:"SeqNoSpecified"`
	ArrivalEHFSMDGCode   string        `json:"ArrivalEHFSMDGCode,omitempty"`
}

type MscEHF struct {
	Description string `json:"Description"`
}

type MscCallDate struct {
	Type           string `json:"Type"`
	CallDateTime   string `json:"CallDateTime,omitempty"`
	SeqNo          int    `json:"SeqNo"`
	SeqNoSpecified bool   `json:"SeqNoSpecified"`
}

func (msp *MscScheduleResponse) GenerateSchedule(responseJson []byte) ([]*schema.P2PSchedule, error) {

	var getFirstEtdLastEta = func(mscCall MscCall, dateType string) string {
		var result string
		for _, callDate := range mscCall.CallDates {
			if callDate.Type == dateType {
				result = callDate.CallDateTime
				break
			}
		}
		return result
	}
	var mscScheduleData MscScheduleResponse
	err := json.Unmarshal(responseJson, &mscScheduleData)
	if err != nil {
		return nil, err
	}
	var mscScheduleList = make([]*schema.P2PSchedule, 0, len(mscScheduleData.MSCSchedule.Transactions))
	for _, schedule := range mscScheduleData.MSCSchedule.Transactions {
		origin := schedule.Schedules[0].Calls[0]
		destination := schedule.Schedules[len(schedule.Schedules)-1].Calls[1]
		etd := getFirstEtdLastEta(origin, "ETD")
		eta := getFirstEtdLastEta(destination, "ETA")

		scheduleResult := &schema.P2PSchedule{
			Scac:          "MSCU",
			PointFrom:     origin.Code,
			PointTo:       destination.Code,
			Etd:           etd,
			Eta:           eta,
			TransitTime:   external.CalculateTransitTime(&etd, &eta),
			Transshipment: len(schedule.Schedules) > 1,
			Legs:          msp.GenerateScheduleLeg(schedule.Schedules),
		}

		mscScheduleList = append(mscScheduleList, scheduleResult)
	}
	return mscScheduleList, nil
}

func (msp *MscScheduleResponse) GenerateScheduleLeg(legResponse []*MscSchedule) []*schema.Leg {
	var mscLegList = make([]*schema.Leg, 0, len(legResponse))
	for _, leg := range legResponse {
		pointBase := msp.GenerateLegPoints(leg)
		eventDate := msp.GenerateEventDate(leg)
		voyageService := msp.GenerateVoyageService(leg)
		legInstance := &schema.Leg{
			PointFrom:       pointBase.PointFrom,
			PointTo:         pointBase.PointTo,
			Etd:             eventDate.Etd,
			Eta:             eventDate.Eta,
			TransitTime:     eventDate.TransitTime,
			Cutoffs:         eventDate.Cutoffs,
			Transportations: msp.GenerateTransport(leg).Transportations,
			Voyages:         voyageService.Voyages,
			Services:        voyageService.Services,
		}
		mscLegList = append(mscLegList, legInstance)
	}
	return mscLegList
}

func (msp *MscScheduleResponse) GenerateLegPoints(legDetails *MscSchedule) *schema.Leg {
	pointFrom := schema.PointBase{
		LocationName: legDetails.Calls[0].Name,
		LocationCode: legDetails.Calls[0].Code,
		TerminalName: legDetails.Calls[0].EHF.Description,
		TerminalCode: legDetails.Calls[0].DepartureEHFSMDGCode,
	}

	pointTo := schema.PointBase{
		LocationName: legDetails.Calls[1].Name,
		LocationCode: legDetails.Calls[1].Code,
		TerminalName: legDetails.Calls[1].EHF.Description,
		TerminalCode: legDetails.Calls[1].DepartureEHFSMDGCode,
	}

	portPairs := &schema.Leg{
		PointFrom: &pointFrom,
		PointTo:   &pointTo,
	}
	return portPairs
}

func (msp *MscScheduleResponse) GenerateEventDate(legDetails *MscSchedule) *schema.Leg {
	var getEventDate = func(mscCall MscCall, dateType string) string {
		var result string
		for _, callDate := range mscCall.CallDates {
			if callDate.Type == dateType {
				result = callDate.CallDateTime
				break
			}
		}
		return result
	}
	etd := getEventDate(legDetails.Calls[0], "ETD")
	eta := getEventDate(legDetails.Calls[1], "ETA")
	transitTime := external.CalculateTransitTime(&etd, &eta)
	cyCutoffDate := getEventDate(legDetails.Calls[0], "CYCUTOFF")
	docCutoffDate := getEventDate(legDetails.Calls[0], "SI")
	vgmCutoffDate := getEventDate(legDetails.Calls[0], "VGM")

	var cutoffs *schema.Cutoff
	if cyCutoffDate != "" || docCutoffDate != "" || vgmCutoffDate != "" {
		cutoffs = &schema.Cutoff{
			CyCutoffDate:  cyCutoffDate,
			DocCutoffDate: docCutoffDate,
			VgmCutoffDate: vgmCutoffDate,
		}

	} else {
		cutoffs = nil
	}

	eventTime := &schema.Leg{
		Etd:         etd,
		Eta:         eta,
		TransitTime: transitTime,
		Cutoffs:     cutoffs,
	}

	return eventTime
}

func (msp *MscScheduleResponse) GenerateVoyageService(legDetails *MscSchedule) *schema.Leg {
	var internalVoyage string
	voyageNumber := legDetails.Voyages
	serviceCode := legDetails.Service
	if voyageNumber != nil {
		internalVoyage = voyageNumber[0].Description
	} else {
		internalVoyage = "TBN"
	}
	voyage := &schema.Voyage{
		InternalVoyage: internalVoyage,
	}

	var service *schema.Service
	if serviceCode != nil {
		service = &schema.Service{
			ServiceCode: serviceCode.Description,
		}
	} else {
		service = nil
	}

	voyageServices := &schema.Leg{
		Voyages:  voyage,
		Services: service,
	}

	return voyageServices
}

func (msp *MscScheduleResponse) GenerateTransport(legDetails *MscSchedule) *schema.Leg {
	tr := schema.Transportation{
		TransportType: schema.TransportType("Vessel"),
		TransportName: legDetails.TransportationMeansName,
		ReferenceType: "IMO",
		Reference:     legDetails.IMONumber,
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

func (msp *MscScheduleResponse) TokenHeaderParams(e *env.Manager) interfaces.HeaderParams {
	thumbprintBytes, _ := hex.DecodeString(*e.MscThumbPrint)
	x5t := base64.StdEncoding.EncodeToString(thumbprintBytes)
	rsaKeyBytes, _ := base64.StdEncoding.DecodeString(*e.MscRsa)
	block, _ := pem.Decode(rsaKeyBytes)
	privateKey, _ := x509.ParsePKCS8PrivateKey(block.Bytes)
	now := time.Now()
	claims := jwt.MapClaims{
		"aud": e.MscAudience,
		"iss": e.MscClient,
		"sub": e.MscClient,
		"exp": now.Add(2 * time.Hour).Unix(),
		"nbf": now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["x5t"] = x5t
	token.Header["typ"] = "JWT"
	signedToken, _ := token.SignedString(privateKey)
	tokenHeaders := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	tokenParams := map[string]string{"scope": *e.MscScope, "client_id": *e.MscClient,
		"client_assertion_type": "urn:ietf:params:oauth:client-assertion-type:jwt-bearer",
		"grant_type":            "client_credentials", "client_assertion": signedToken}
	headerParams := interfaces.HeaderParams{Headers: tokenHeaders, Params: tokenParams}
	return headerParams
}

func (msp *MscScheduleResponse) ScheduleHeaderParams(p *interfaces.ScheduleArgs[*schema.QueryParams]) interfaces.HeaderParams {
	var calculateEndDate = func(startDate string, searchRange int) string {
		date, _ := time.Parse("2006-01-02", startDate)
		endDate := date.AddDate(0, 0, searchRange*7)
		return endDate.Format("2006-01-02")
	}
	scheduleHeaders := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", p.Token.Data["access_token"].(string)),
	}
	scheduleParams := map[string]string{
		"fromPortUNCode": p.Query.PointFrom,
		"toPortUNCode":   p.Query.PointTo,
		"fromDate":       p.Query.StartDate,
		"toDate":         calculateEndDate(p.Query.StartDate, p.Query.SearchRange),
	}

	if p.Query.StartDateType == schema.Departure {
		scheduleParams["datesRelated"] = "POL"
	} else {
		scheduleParams["datesRelated"] = "POD"
	}
	headerParams := interfaces.HeaderParams{Headers: scheduleHeaders, Params: scheduleParams}
	return headerParams
}
