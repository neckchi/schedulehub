package schema

var EventType = map[string]string{
	"UNL": "Unloading",
	"LOA": "Loading",
	"PAS": "Pass",
}

type Port struct {
	PortName string `json:"portName,omitempty"`
	PortCode string `json:"portCode"`
}

type PortCalls struct {
	Seq          int         `json:"seq"`
	Key          interface{} `json:"key"`
	Bound        interface{} `json:"bound"`
	Voyage       interface{} `json:"voyage"`
	PortEvent    string      `json:"portEvent"`
	Port         Port        `json:"port,required,portCodeValidation"`
	EstimateDate string      `json:"estimateDate"`
}

type VesselDetails struct {
	VesselName string `json:"vesselName,required"`
	Imo        string `json:"imo,required"`
}

type Services struct {
	ServiceCode string `json:"serviceCode,omitempty"`
}

type MasterVoyage struct {
	Scac       string        `json:"scac,isValidCarrier"`
	Voyage     string        `json:"voyage"`
	NextVoyage string        `json:"nextVoyage,omitempty"`
	Vessel     VesselDetails `json:"vessel"`
	Services   Services      `json:"services"`
	Calls      []PortCalls   `json:"calls"`
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
