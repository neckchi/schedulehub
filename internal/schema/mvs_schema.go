package schema

import (
	"github.com/go-playground/validator/v10"
	"regexp"
)

var MVSResponseValidate *validator.Validate

func init() {
	MVSResponseValidate = validator.New(validator.WithRequiredStructEnabled())

	errIMO := MVSResponseValidate.RegisterValidation("imoValidation", func(fl validator.FieldLevel) bool {
		regex := regexp.MustCompile(`^[0-9]{7}$`)
		value := fl.Field().String()
		return regex.MatchString(value)
	})
	if errIMO != nil {
		return
	}

}

var EventType = map[string]string{
	"UNL": "Unloading",
	"LOA": "Loading",
	"PAS": "Pass",
}

type Port struct {
	PortName     string `json:"portName,omitempty"`
	PortCode     string `json:"portCode" validate:"required,portCodeValidation"`
	TerminalName string `json:"terminalName,omitempty" validate:"omitempty"`
	TerminalCode string `json:"terminalCode,omitempty" validate:"omitempty"`
}

type VesselDetails struct {
	VesselName string `json:"vesselName" validate:"required"`
	Imo        string `json:"imo" validate:"required,imoValidation"`
}

type Services struct {
	ServiceCode string `json:"serviceCode,omitempty"`
	ServiceName string `json:"service,omitempty"`
}

type PortCalls struct {
	Seq                int         `json:"seq" validate:"required"`
	Key                interface{} `json:"key" validate:"required"`
	Bound              interface{} `json:"bound" validate:"required"`
	Voyage             interface{} `json:"voyage" validate:"required"`
	Service            Services    `json:"service" validate:"omitempty"`
	PortEvent          string      `json:"portEvent" validate:"required"`
	Port               Port        `json:"port" validate:"required"`
	EstimatedEventDate string      `json:"estimatedEventDate,omitempty"`
	ActualEventDate    string      `json:"actualEventDate,omitempty"`
}

type MasterVesselSchedule struct {
	Scac       string        `json:"scac" validate:"required"`
	Voyage     string        `json:"voyage" validate:"required"`
	NextVoyage string        `json:"nextVoyage,omitempty"`
	Vessel     VesselDetails `json:"vessel" validate:"required"`
	Services   Services      `json:"services" validate:"required"`
	Calls      []PortCalls   `json:"calls" `
}

type ScheduleRow struct {
	DataSource      string
	SCAC            string
	ProvideVoyageID string
	VoyageNum       string
	VesselName      string
	VesselIMO       string
	VoyageDirection string
	ServiceCode     string
	PortCode        string
	PortName        string
	PortEvent       string
	EventTime       string
	Rank            string
}
