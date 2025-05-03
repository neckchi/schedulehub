package external

import (
	"time"
)

func GetTransportType(key string) (string, bool) {
	transportTypeList := map[string]string{
		"Land Trans":  "Truck",
		"Feeder":      "Feeder",
		"TO BE NAMED": "Vessel",
		"BAR":         "Barge",
		"BCO":         "Barge",
		"FEF":         "Feeder",
		"FEO":         "Feeder",
		"MVS":         "Vessel",
		"RCO":         "Rail",
		"RR":          "Rail",
		"TRK":         "Truck",
		"VSF":         "Feeder",
		"VSL":         "Feeder",
		"VSM":         "Vessel",
	}

	if value, ok := transportTypeList[key]; ok {
		return value, true
	}
	return "Vessel", false // Default value if key is not found
}

func ConvertDateFormat(originalDate *string, originalLayout string) string {
	// originalLayout := "2006-01-02T15:04:05.000-07:00" // Layout matching the original format

	newLayout := "2006-01-02T15:04:05"
	_, ok := time.Parse(newLayout, *originalDate)
	if ok == nil {
		return *originalDate
	}

	parsedTime, err := time.Parse(originalLayout, *originalDate)
	if err != nil {
		return ""
	}
	*originalDate = parsedTime.Format(newLayout)
	return *originalDate

}

func CalculateTransitTime(etd, eta *string) int {
	layout := "2006-01-02T15:04:05"
	if eta == nil || etd == nil {
		return 0
	}
	etaTime, err1 := time.Parse(layout, *eta)
	etdTime, err2 := time.Parse(layout, *etd)
	if err1 != nil || err2 != nil {
		return 0
	}

	return int(etaTime.Sub(etdTime).Hours() / 24)
}
