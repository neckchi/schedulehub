package schema

import (
	"github.com/go-playground/validator/v10"
	"regexp"
	"time"
)

var P2PResponseValidate *validator.Validate

func init() {
	P2PResponseValidate = validator.New(validator.WithRequiredStructEnabled())

	// Function to validate if a string is in ISO8601 format
	errDate := P2PResponseValidate.RegisterValidation("isValidDate", func(fl validator.FieldLevel) bool {
		const layout1 = "2006-01-02T15:04:05"
		value := fl.Field().String()
		_, err := time.Parse(layout1, value)
		return err == nil
	})
	if errDate != nil {
		return
	}

	errPort := P2PResponseValidate.RegisterValidation("portCodeValidation", func(fl validator.FieldLevel) bool {
		regex := regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}$`)
		value := fl.Field().String()
		return regex.MatchString(value)
	})
	if errPort != nil {
		return
	}

	P2PResponseValidate.RegisterStructValidation(TransportationValidation, Transportation{})
	P2PResponseValidate.RegisterStructValidation(LegEventDateValidation, Leg{})
	P2PResponseValidate.RegisterStructValidation(ScheduleEventDateValidation, P2PSchedule{})

}

type PointBase struct {
	LocationName string `json:"locationName,omitempty"`
	LocationCode string `json:"locationCode" validate:"required,portCodeValidation" description:"Location Code"`
	TerminalName string `json:"terminalName,omitempty"`
	TerminalCode string `json:"terminalCode,omitempty"`
}

type Cutoff struct {
	CyCutoffDate  string `json:"cyCutoffDate,omitempty" validate:"omitempty,isValidDate"`
	DocCutoffDate string `json:"docCutoffDate,omitempty" validate:"omitempty,isValidDate"`
	VgmCutoffDate string `json:"vgmCutoffDate,omitempty" validate:"omitempty,isValidDate"`
}

type TransportType string

const (
	Vessel     TransportType = "Vessel"
	Barge      TransportType = "Barge"
	Feeder     TransportType = "Feeder"
	Truck      TransportType = "Truck"
	Rail       TransportType = "Rail"
	Truckrail  TransportType = "Truckrail"
	Roadrail   TransportType = "Roadrail"
	Road       TransportType = "Road"
	Intermodal TransportType = "Intermodal"
)

var referenceMapping = map[TransportType]string{
	Vessel:     "1",
	Truck:      "3",
	Road:       "3",
	Intermodal: "5",
	Barge:      "9",
	Feeder:     "9",
	Roadrail:   "11", // "Road/Rail"
	Rail:       "11",
	Truckrail:  "11", // "Truck/Rail"

}

type Transportation struct {
	TransportType TransportType `json:"transportType,required" validate:"required"`
	TransportName string        `json:"transportName,omitempty"`
	ReferenceType string        `json:"referenceType,omitempty"`
	Reference     string        `json:"reference,omitempty"`
}

func TransportationValidation(sl validator.StructLevel) {

	t := sl.Current().Interface().(Transportation)

	if (t.ReferenceType == "") != (t.Reference == "") {
		sl.ReportError(t.TransportType, "TransportType", "TransportType", "TransportationValidation", string(t.TransportType))
		sl.ReportError(t.Reference, "reference", "Reference", "TransportationValidation", t.Reference)
	}

}

func (t *Transportation) MapTransport() error {
	if t.Reference == "" && t.TransportType != "" {
		if t.TransportName == "" {
			t.TransportName = "TBN"
		}
		t.ReferenceType = "IMO"
		if refVal, ok := referenceMapping[t.TransportType]; ok {
			t.Reference = refVal
		}
	}
	return nil
}

type Voyage struct {
	InternalVoyage string `json:"internalVoyage" validate:"required"`
	ExternalVoyage string `json:"externalVoyage,omitempty" validate:"omitempty"`
}

type Service struct {
	ServiceCode string `json:"serviceCode,omitempty" validate:"omitempty"`
	ServiceName string `json:"serviceName,omitempty" validate:"omitempty"`
}

type Leg struct {
	PointFrom       *PointBase     `json:"pointFrom" validate:"omitempty"`
	PointTo         *PointBase     `json:"pointTo" validate:"omitempty"`
	Etd             string         `json:"etd" validate:"required,isValidDate"`
	Eta             string         `json:"eta" validate:"required,isValidDate"`
	TransitTime     int            `json:"transitTime" validate:"gte=0"`
	Cutoffs         *Cutoff        `json:"cutoffs,omitempty" validate:"omitempty"`
	Transportations Transportation `json:"transportations"`
	Voyages         *Voyage        `json:"voyages" validate:"omitempty"`
	Services        *Service       `json:"services,omitempty" validate:"omitempty"`
}

func LegEventDateValidation(sl validator.StructLevel) {
	layout := "2006-01-02T15:04:05"
	l := sl.Current().Interface().(Leg)
	etd, _ := time.Parse(layout, l.Etd)
	eta, _ := time.Parse(layout, l.Eta)

	if eta.Before(etd) || etd.After(eta) {
		sl.ReportError(l.Etd, "etd", "Etd", "Non-chronological event", "")
		sl.ReportError(l.Eta, "eta", "Eta", "Non-chronological event", "")
	}
}

// Schedule struct equivalent in Go
type P2PSchedule struct {
	Scac          string `json:"scac" validate:"required"`
	PointFrom     string `json:"pointFrom" validate:"required,portCodeValidation"`
	PointTo       string `json:"pointTo" validate:"required,portCodeValidation"`
	Etd           string `json:"etd" validate:"required,isValidDate"`
	Eta           string `json:"eta" validate:"required,isValidDate"`
	TransitTime   int    `json:"transitTime" validate:"gte=0"`
	Transshipment bool   `json:"transshipment"`
	Legs          []*Leg `json:"legs" validate:"required,dive"`
}

func ScheduleEventDateValidation(sl validator.StructLevel) {
	layout := "2006-01-02T15:04:05"
	s := sl.Current().Interface().(P2PSchedule)
	etd, _ := time.Parse(layout, s.Etd)
	eta, _ := time.Parse(layout, s.Eta)

	if eta.Before(etd) || etd.After(eta) {
		sl.ReportError(s.Etd, "etd", "Etd", "isEligible", "")
		sl.ReportError(s.Eta, "eta", "Eta", "isEligible", "")
	}
}

// Product struct equivalent in Go
type Product struct {
	Origin      string         `json:"origin" validate:"required,portCodeValidation"`
	Destination string         `json:"destination" validate:"required,portCodeValidation"`
	Schedules   []*P2PSchedule `json:"schedules" validate:"dive"`
}

// HealthCheck struct equivalent in Go
type HealthCheck struct {
	Status string `json:"status" validate:"required"`
}
