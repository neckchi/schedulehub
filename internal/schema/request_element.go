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

//var ValidCarrier = map[CarrierCode]bool{
//	CMDU: true,
//	ANNU: true,
//	CHNL: true,
//	APLU: true,
//	ZIMU: true,
//	HLCU: true,
//	MSCU: true,
//	OOLU: true,
//	COSU: true,
//	ONEY: true,
//	MAEU: true,
//	MAEI: true,
//	YMJA: true,
//	EGLV: true,
//}

func CollectCarriers(scacProvider *[]CarrierCode) []CarrierCode {
	excludedCarriers := map[CarrierCode]bool{
		ANNU: true,
		CHNL: true,
	}
	filteredCarriers := make([]CarrierCode, 0)
	for _, scac := range *scacProvider {
		if !excludedCarriers[scac] {
			filteredCarriers = append(filteredCarriers, scac)
		}
	}
	*scacProvider = filteredCarriers
	return *scacProvider

}

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
