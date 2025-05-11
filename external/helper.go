package external

import (
	"fmt"
	"github.com/neckchi/schedulehub/internal/schema"
	"regexp"
	"slices"
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

func ValidateIMO(imo string) bool {
	regex := regexp.MustCompile(`^[0-9]{7}$`)
	return regex.MatchString(imo)
}

func CalculateDateRangeForP2P(q *schema.QueryParams, timeFormat string) (startTime, endTime string, err error) {
	date, err := time.Parse("2006-01-02", q.StartDate)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse date: %w", err)
	}
	startDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, q.SearchRange*7)
	startTime = startDate.Format(timeFormat)
	endTime = endDate.Format(timeFormat)
	return startTime, endTime, nil
}

const defaultStartDayNum = 20
const defaultEndDayNum = 150

func CalculateDateRangeForMVS(startDate string, dateRange int) (string, string) {
	date, _ := time.Parse("2006-01-02", startDate)
	maxStartDateRange := slices.Max([]int{dateRange, defaultStartDayNum})
	maxEndDateRange := slices.Max([]int{dateRange, defaultEndDayNum})
	startDateAdjusted := date.AddDate(0, 0, -maxStartDateRange)
	endDateAdjusted := date.AddDate(0, 0, maxEndDateRange)
	return startDateAdjusted.Format("2006-01-02"), endDateAdjusted.Format("2006-01-02")
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
