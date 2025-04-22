package handlers

import (
	"github.com/neckchi/schedulehub/internal/schema"
	"slices"
)

type ScheduleFilterOption func(*schema.Schedule, *schema.QueryParams) bool

func WithDirectOnly() ScheduleFilterOption {
	return func(schedule *schema.Schedule, query *schema.QueryParams) bool {
		if query.DirectOnly {
			return !schedule.Transshipment == query.DirectOnly
		}
		return true
	}
}

func WithTSP() ScheduleFilterOption {
	return func(schedule *schema.Schedule, query *schema.QueryParams) bool {
		if query.TSP != "" {
			return slices.ContainsFunc(schedule.Legs[1:], func(leg *schema.Leg) bool {
				return leg.PointFrom.LocationCode == query.TSP ||
					leg.PointTo.LocationCode == query.TSP
			})
		}
		return true
	}
}

func WithVesselIMO() ScheduleFilterOption {
	return func(schedule *schema.Schedule, query *schema.QueryParams) bool {
		if query.VesselIMO != "" {
			return slices.ContainsFunc(schedule.Legs, func(leg *schema.Leg) bool {
				return leg.Transportations.Reference == query.VesselIMO
			})
		}
		return true
	}
}

func WithService() ScheduleFilterOption {
	return func(schedule *schema.Schedule, query *schema.QueryParams) bool {
		if query.Service != "" {
			return slices.ContainsFunc(schedule.Legs, func(leg *schema.Leg) bool {
				return leg.Services != nil && leg.Services.ServiceCode == query.Service
			})
		}
		return true
	}
}

func ScheduleFilters(opts ...ScheduleFilterOption) ScheduleFilterOption {
	return func(schedule *schema.Schedule, query *schema.QueryParams) bool {
		result := true
		for _, opt := range opts {
			result = result && opt(schedule, query)
		}
		return result
	}
}
