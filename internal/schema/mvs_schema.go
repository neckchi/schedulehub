package schema

import (
	"github.com/go-playground/validator/v10"
	"regexp"
	"time"
)

var MVSResponseValidate *validator.Validate

func init() {
	MVSResponseValidate = validator.New(validator.WithRequiredStructEnabled())
	// Function to validate if a string is in ISO8601 format
	errDate := MVSResponseValidate.RegisterValidation("isValidDate", func(fl validator.FieldLevel) bool {
		const layout1 = "2006-01-02T15:04:05"
		value := fl.Field().String()
		_, err := time.Parse(layout1, value)
		return err == nil
	})
	if errDate != nil {
		return
	}

	errPort := MVSResponseValidate.RegisterValidation("portCodeValidation", func(fl validator.FieldLevel) bool {
		regex := regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}$`)
		value := fl.Field().String()
		return regex.MatchString(value)
	})
	if errPort != nil {
		return
	}

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
	Seq                int         `json:"seq" validate:"required,numeric,gte=1"`
	Key                interface{} `json:"key" validate:"required"`
	Bound              interface{} `json:"bound" validate:"required"`
	Voyage             interface{} `json:"voyage" validate:"required"`
	Service            *Services   `json:"service" validate:"omitempty"`
	PortEvent          string      `json:"portEvent" validate:"required,oneof=Loading Unloading"`
	Port               *Port       `json:"port" validate:"omitempty"`
	EstimatedEventDate string      `json:"estimatedEventDate,omitempty" validate:"omitempty,isValidDate"`
	ActualEventDate    string      `json:"actualEventDate,omitempty" validate:"omitempty,isValidDate"`
}

type MasterVesselSchedule struct {
	Scac       string         `json:"scac" validate:"required"`
	Voyage     string         `json:"voyage" validate:"required"`
	NextVoyage string         `json:"nextVoyage,omitempty"`
	Vessel     *VesselDetails `json:"vessel" validate:"omitempty"`
	Services   *Services      `json:"services" validate:"omitempty"`
	Calls      []PortCalls    `json:"calls" validate:"required,dive"`
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
