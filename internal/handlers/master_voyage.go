package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/internal/database"
	"github.com/neckchi/schedulehub/internal/exceptions"
	"github.com/neckchi/schedulehub/internal/middleware"
	"github.com/neckchi/schedulehub/internal/schema"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

func VoyageHandler(or database.OracleRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		queryParams, _ := r.Context().Value(middleware.VVQueryParamsKey).(schema.QueryParamsForVesselVoyage)
		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()
		sqlResults, err := or.QueryContext(ctx, queryParams)
		if err != nil {
			errMsg := fmt.Errorf("Database query failed: %v", err)
			exceptions.InternalErrorHandler(w, errMsg)
			return
		}
		log.Infof("Fetched vessel voyages from database %.3fs", time.Since(startTime).Seconds())
		if len(sqlResults) > 1 {
			dataProcessTime := time.Now()
			duplicates := findDuplicates(sqlResults)
			uniqueVoyageNumbers := getUniqueVoyageNumbers(sqlResults, queryParams.Voyage)
			uniqueBounds, uniqueKeys := getUniqueBoundsAndKeys(sqlResults, &duplicates)
			portOfCalls := constructPortCalls(sqlResults, &duplicates, uniqueBounds, uniqueKeys, uniqueVoyageNumbers, queryParams.Voyage)
			finalCalls := removeDuplicates(portOfCalls, duplicates)
			addSequenceNumbers(finalCalls)
			apiResult := constructAPIResult(queryParams, sqlResults, finalCalls, uniqueVoyageNumbers)
			log.Infof("Finished processing master voyages  %.3fs", time.Since(dataProcessTime).Seconds())
			jsonBytes, err := json.Marshal(apiResult)
			if err != nil {
				errMsg := fmt.Errorf("JSON marshaling failed: %v", err)
				exceptions.InternalErrorHandler(w, errMsg)
				return
			}
			w.Write(jsonBytes)
		} else {
			w.Write([]byte(`{"message":"No available vessel voyage"}`))
		}
	})
}

type groupKey struct {
	PortEvent string
	PortCode  string
	EventTime string
}

func findDuplicates(sqlResults []schema.ScheduleRow) map[groupKey]bool {
	counts := make(map[groupKey]int, len(sqlResults))
	for _, item := range sqlResults {
		key := groupKey{
			PortEvent: item.PortEvent,
			PortCode:  item.PortCode,
			EventTime: item.EventTime,
		}
		counts[key]++
	}
	duplicates := make(map[groupKey]bool, len(counts)/2)
	for key, count := range counts {
		if count > 1 {
			duplicates[key] = true
		}
	}
	return duplicates
}

func getUniqueVoyageNumbers(sqlResults []schema.ScheduleRow, voyage *string) []string {
	uniqueVoyageNumbers := make([]string, 0)
	voyageSet := make(map[string]bool)
	for _, result := range sqlResults {
		if result.VoyageNum != *voyage && !voyageSet[result.VoyageNum] {
			uniqueVoyageNumbers = append(uniqueVoyageNumbers, result.VoyageNum)
			voyageSet[result.VoyageNum] = true
		}
	}
	return uniqueVoyageNumbers
}

func getUniqueBoundsAndKeys(sqlResults []schema.ScheduleRow, duplicates *map[groupKey]bool) ([]string, []string) {
	uniqueBounds := make([]string, 0)
	uniqueKeys := make([]string, 0)
	if len(*duplicates) > 0 {
		boundsSet := make(map[string]bool)
		voyageKeySet := make(map[string]bool)
		for _, result := range sqlResults {
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
	return uniqueBounds, uniqueKeys
}

func constructPortCalls(sqlResults []schema.ScheduleRow, duplicates *map[groupKey]bool, uniqueBounds, uniqueKeys, uniqueVoyageNumbers []string, voyage *string) []schema.PortCalls {
	var portOfCalls []schema.PortCalls
	for _, port := range sqlResults {
		key := groupKey{port.PortEvent, port.PortCode, port.EventTime}
		var boundValue any
		var uniqueKeyVals any
		var voyageValue any
		if (*duplicates)[key] {
			boundValue = uniqueBounds
			uniqueKeyVals = uniqueKeys
			if len(uniqueVoyageNumbers) > 0 {
				voyageValue = []string{*voyage, uniqueVoyageNumbers[0]}
			} else {
				voyageValue = []string{*voyage}
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

func removeDuplicates(portOfCalls []schema.PortCalls, duplicates map[groupKey]bool) []schema.PortCalls {
	var finalCalls []schema.PortCalls
	if len(duplicates) > 0 {
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
	return finalCalls
}

func addSequenceNumbers(finalCalls []schema.PortCalls) {
	for i := range finalCalls {
		finalCalls[i].Seq = i + 1
	}
}

func constructAPIResult(queryParams schema.QueryParamsForVesselVoyage, sqlResults []schema.ScheduleRow, finalCalls []schema.PortCalls, uniqueVoyageNumbers []string) schema.MasterVoyage {
	var nextVoyage string
	if len(uniqueVoyageNumbers) > 0 {
		nextVoyage = uniqueVoyageNumbers[0]
	}
	return schema.MasterVoyage{
		Scac:       string(*queryParams.SCAC),
		Voyage:     *queryParams.Voyage,
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
