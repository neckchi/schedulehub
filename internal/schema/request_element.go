package schema

// Enum for StartDateType
type StartDateType string

const (
	Departure StartDateType = "Departure"
	Arrival   StartDateType = "Arrival"
)

type CarrierCode string

const (
	CMDU CarrierCode = "CMDU"
	ANNU CarrierCode = "ANNU"
	CHNL CarrierCode = "CHNL"
	APLU CarrierCode = "APLU"
	ZIMU CarrierCode = "ZIMU"
	HLCU CarrierCode = "HLCU"
	MSCU CarrierCode = "MSCU"
	COSU CarrierCode = "COSU"
	OOLU CarrierCode = "OOLU"
	ONEY CarrierCode = "ONEY"
	MAEU CarrierCode = "MAEU"
	MAEI CarrierCode = "MAEI"
	YMJA CarrierCode = "YMJA"
	EGLV CarrierCode = "EGLV"
)

// Mapping of SCAC to Internal Carrier Code
var InternalCodeMapping = map[CarrierCode]string{
	CMDU: "0001",
	ANNU: "0002",
	CHNL: "0011",
	APLU: "0015",
}

var InternalCodeToScac = map[string]CarrierCode{
	"0001": CMDU,
	"0002": ANNU,
	"0011": CHNL,
	"0015": APLU,
}
