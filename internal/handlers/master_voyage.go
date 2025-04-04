package handlers

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/internal/database"
	"github.com/neckchi/schedulehub/internal/exceptions"
	"github.com/neckchi/schedulehub/internal/middleware"
	"github.com/neckchi/schedulehub/internal/schema"
	"net/http"
	"time"
)

func VoyageHandler(or database.OracleRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryParams, _ := r.Context().Value(middleware.VVQueryParamsKey).(schema.QueryParamsForVesselVoyage)
		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()
		sqlResults, err := or.QueryContext(ctx, queryParams)
		if err != nil {
			errMsg := fmt.Errorf("Database query failed: %v", err)
			exceptions.InternalErrorHandler(w, errMsg)
			return
		}
		switch exist := len(sqlResults); exist > 1 {
		case true:
			overlappedPorts := findOverlappedPorts(sqlResults)
			uniqueVoyageNumbers, uniqueBounds, uniqueKeys := getUniqueData(sqlResults, overlappedPorts)
			portOfCalls := constructPortCalls(sqlResults, overlappedPorts, uniqueBounds, uniqueKeys, uniqueVoyageNumbers)
			finalCalls := removeDuplicates(portOfCalls, overlappedPorts)
			apiResult := constructAPIResult(queryParams, sqlResults, finalCalls, uniqueVoyageNumbers)
			jsonBytes, err := json.Marshal(apiResult)
			if err != nil {
				errMsg := fmt.Errorf("JSON marshaling failed: %v", err)
				exceptions.InternalErrorHandler(w, errMsg)
			}
			_, _ = w.Write(jsonBytes)

		case false:
			_, _ = w.Write([]byte(`{"message":"No available vessel voyage"}`))
		}
	})
}

type groupKey struct {
	PortEvent string
	PortCode  string
	EventTime string
}

func findOverlappedPorts(sqlResults []schema.ScheduleRow) map[groupKey]bool {
	counts := make(map[groupKey]int, len(sqlResults))
	for _, item := range sqlResults {
		key := groupKey{
			PortEvent: item.PortEvent,
			PortCode:  item.PortCode,
			EventTime: item.EventTime,
		}
		counts[key]++
	}
	overlappedPorts := make(map[groupKey]bool, len(counts)/2)
	for key, count := range counts {
		if count > 1 {
			overlappedPorts[key] = true
		}
	}
	return overlappedPorts
}

func getUniqueData(sqlResults []schema.ScheduleRow, overlappedPorts map[groupKey]bool) ([]string, []string, []string) {
	uniqueVoyageNumbers := make([]string, 0, 2)
	uniqueBounds := make([]string, 0, 2)
	uniqueKeys := make([]string, 0, 2)

	voyageSet := make(map[string]bool)
	boundsSet := make(map[string]bool)
	voyageKeySet := make(map[string]bool)

	currentVoyage := sqlResults[0].VoyageNum

	for _, result := range sqlResults {
		// Collect unique voyage numbers
		if result.VoyageNum != currentVoyage && !voyageSet[result.VoyageNum] {
			uniqueVoyageNumbers = append(uniqueVoyageNumbers, result.VoyageNum)
			voyageSet[result.VoyageNum] = true
		}

		// Collect unique bounds and keys if there are overlapping ports
		if len(overlappedPorts) > 0 {
			if !boundsSet[result.VoyageDirection] {
				uniqueBounds = append(uniqueBounds, result.VoyageDirection)
				boundsSet[result.VoyageDirection] = true
			}
			if !voyageKeySet[result.ProvideVoyageID] {
				uniqueKeys = append(uniqueKeys, result.ProvideVoyageID)
				voyageKeySet[result.ProvideVoyageID] = true
			}
		}
	}

	return uniqueVoyageNumbers, uniqueBounds, uniqueKeys
}

func constructPortCalls(sqlResults []schema.ScheduleRow, overlappedPorts map[groupKey]bool, uniqueBounds, uniqueKeys, uniqueVoyageNumbers []string) []schema.PortCalls {
	var portOfCalls []schema.PortCalls
	currentVoyage := sqlResults[0].VoyageNum
	for _, port := range sqlResults {
		key := groupKey{port.PortEvent, port.PortCode, port.EventTime}
		var boundValue any
		var uniqueKeyVals any
		var voyageValue any
		if (overlappedPorts)[key] {
			boundValue = uniqueBounds
			uniqueKeyVals = uniqueKeys
			if len(uniqueVoyageNumbers) > 0 {
				voyageValue = []string{currentVoyage, uniqueVoyageNumbers[0]}
			} else {
				voyageValue = []string{currentVoyage}
			}
		} else {
			boundValue = port.VoyageDirection
			uniqueKeyVals = port.ProvideVoyageID
			voyageValue = port.VoyageNum
		}
		portCall := schema.PortCalls{
			Key:          uniqueKeyVals,
			Bound:        boundValue,
			Voyage:       voyageValue,
			PortEvent:    schema.EventType[port.PortEvent],
			Port:         schema.Port{PortName: port.PortName, PortCode: port.PortCode},
			EstimateDate: port.EventTime,
		}
		portOfCalls = append(portOfCalls, portCall)
	}
	return portOfCalls
}

func removeDuplicates(portOfCalls []schema.PortCalls, overlappedPorts map[groupKey]bool) []schema.PortCalls {
	var finalCalls []schema.PortCalls
	if len(overlappedPorts) > 0 {
		seen := make(map[string]bool)
		for _, call := range portOfCalls {
			callBytes, _ := json.Marshal(call)
			callStr := string(callBytes)
			if !seen[callStr] {
				seen[callStr] = true
				finalCalls = append(finalCalls, call)
			}
		}
	} else {
		finalCalls = portOfCalls
	}
	for i := range finalCalls {
		finalCalls[i].Seq = i + 1
	}
	return finalCalls
}

func constructAPIResult(queryParams schema.QueryParamsForVesselVoyage, sqlResults []schema.ScheduleRow, finalCalls []schema.PortCalls, uniqueVoyageNumbers []string) schema.MasterVoyage {
	var nextVoyage string
	if len(uniqueVoyageNumbers) > 0 {
		nextVoyage = uniqueVoyageNumbers[0]
	}
	return schema.MasterVoyage{
		Scac:       string(queryParams.SCAC),
		Voyage:     cmp.Or(*queryParams.Voyage, sqlResults[0].VoyageNum),
		NextVoyage: nextVoyage,
		Vessel: schema.VesselDetails{
			VesselName: sqlResults[0].VesselName,
			Imo:        sqlResults[0].VesselIMO,
		},
		Services: schema.Services{
			ServiceCode: sqlResults[0].ServiceCode,
		},
		Calls: finalCalls,
	}
}
