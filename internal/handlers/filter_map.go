package handlers

import (
	"schedulehub/internal/schema"
	"slices"
)

func ScheduleFilters(schedule *schema.Schedule, query *schema.QueryParams) bool {
	result := true
	if query.DirectOnly != nil {
		result = result && (!schedule.Transshipment == *query.DirectOnly)
	}

	// Define post filter functions in a map with string pointer to query parameters
	filters := map[*string]func() bool{
		query.TSP: func() bool {
			return slices.ContainsFunc(schedule.Legs[1:], func(leg *schema.Leg) bool {
				return leg.PointFrom.LocationCode == *query.TSP ||
					leg.PointTo.LocationCode == *query.TSP
			})
		},
		query.VesselIMO: func() bool {
			return slices.ContainsFunc(schedule.Legs, func(leg *schema.Leg) bool {
				return leg.Transportations.Reference == *query.VesselIMO
			})
		},
		query.Service: func() bool {
			return slices.ContainsFunc(schedule.Legs, func(leg *schema.Leg) bool {
				return leg.Services.ServiceCode != nil &&
					*leg.Services.ServiceCode == *query.Service
			})
		},
	}

	for param, filterFunc := range filters {
		if param != nil {
			result = result && filterFunc()
		}
	}

	return result
}
