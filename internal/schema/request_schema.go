package schema

import (
	"github.com/go-playground/validator/v10"
	"regexp"
	"time"
)

// use a single instance of Validate, it caches struct info
var RequestValidate *validator.Validate

func init() {
	RequestValidate = validator.New(validator.WithRequiredStructEnabled())

	// Function to check if port code is valid format
	errPort := RequestValidate.RegisterValidation("portCodeValidation", func(fl validator.FieldLevel) bool {
		regex := regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}$`)
		value := fl.Field().String()
		return regex.MatchString(value)
	})
	if errPort != nil {
		return
	}

	// Function to check if a string is in the YYYY-MM-DD format
	errDate := RequestValidate.RegisterValidation("isValidDate", func(fl validator.FieldLevel) bool {
		const layout = "2006-01-02"
		value := fl.Field().String()
		_, err := time.Parse(layout, value)
		return err == nil
	})
	if errDate != nil {
		return
	}

	errFlag := RequestValidate.RegisterValidation("isVesselFlag", func(fl validator.FieldLevel) bool {
		regex := regexp.MustCompile(`^[A-Z]{2}$`)
		value := fl.Field().String()
		return regex.MatchString(value)
	})
	if errFlag != nil {
		return
	}

}

// Define the struct with field validations using Go tags
type QueryParams struct {
	PointFrom      string         `json:"pointFrom" validate:"required,portCodeValidation" description:"Port Of Loading" `
	PointTo        string         `json:"pointTo" validate:"required,portCodeValidation" description:"Port Of Discharge" `
	StartDateType  StartDateType  `json:"startDateType" validate:"required,oneof=Departure Arrival" description:"Search by either Departure or Arrival"`
	StartDate      string         `json:"startDate" validate:"required,isValidDate" description:"YYYY-MM-DD"`
	SearchRange    int            `json:"searchRange" validate:"required,oneof=1 2 3 4" description:"Search range based on start date and type, max 4 weeks"`
	SCAC           *[]CarrierCode `json:"scac" validate:"omitempty" example:"MSC,CMA"`
	DirectOnly     *bool          `json:"directOnly" validate:"omitempty" description:"Direct means only show direct schedule else show both (direct/transshipment)"`
	TSP            *string        `json:"transhipmentPort" validate:"omitempty,portCodeValidation" description:"Port Of Transshipment" example:"SGSIN"`
	VesselIMO      *string        `json:"vesselIMO" validate:"omitempty,max=7" description:"Restricts the search to a particular vessel IMO lloyds code"`
	VesselFlagCode *string        `json:"vesselFlagCode" validate:"omitempty,isVesselFlag" description:"Vessel flag"`
	Service        *string        `json:"service" validate:"omitempty" description:"Service code or service name"`
}

type QueryParamsForVesselVoyage struct {
	SCAC      CarrierCode `json:"scac" validate:"required" example:"MSC,CMA"`
	VesselIMO *string     `json:"vesselIMO" validate:"max=7" description:"vessel IMO lloyds code"`
	StartDate *string     `json:"startDate" validate:"omitempty,isValidDate" description:"YYYY-MM-DD"`
	Voyage    *string     `json:"voyageNum"  validate:"omitempty" description:"Voyage Number"`
}
