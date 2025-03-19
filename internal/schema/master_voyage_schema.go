package schema

var EventType = map[string]string{
	"UNL": "Unloading",
	"LOA": "Loading",
	"PAS": "Pass",
}

type Port struct {
	PortName string `json:"portName,omitempty"`
	PortCode string `json:"portCode" validate:"required,portCodeValidation"`
}

type PortCalls struct {
	Seq          int         `json:"seq" validate:"required"`
	Key          interface{} `json:"key" validate:"required"`
	Bound        interface{} `json:"bound" validate:"required"`
	Voyage       interface{} `json:"voyage" validate:"required"`
	PortEvent    string      `json:"portEvent" validate:"required"`
	Port         Port        `json:"port" validate:"required"`
	EstimateDate string      `json:"estimateDate" validate:"required"`
}

type VesselDetails struct {
	VesselName string `json:"vesselName" validate:"required"`
	Imo        string `json:"imo" validate:"required"`
}

type Services struct {
	ServiceCode string `json:"serviceCode,omitempty"`
}

type MasterVoyage struct {
	Scac       string        `json:"scac" validate:"required"`
	Voyage     string        `json:"voyage" validate:"required"`
	NextVoyage string        `json:"nextVoyage,omitempty"`
	Vessel     VesselDetails `json:"vessel" validate:"required"`
	Services   Services      `json:"services"`
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
