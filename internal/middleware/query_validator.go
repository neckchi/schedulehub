package middleware

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/neckchi/schedulehub/internal/exceptions"
	"github.com/neckchi/schedulehub/internal/schema"
	log "github.com/sirupsen/logrus"
)

type queryContextKey string

const (
	P2PQueryParamsKey queryContextKey = "p2PQueryParams"
	VVQueryParamsKey  queryContextKey = "VVQueryParams"
)

// allowedParams creates a map of valid JSON field tags for a given struct.
func allowedParams(schemaStruct interface{}) map[string]struct{} {
	val := reflect.ValueOf(schemaStruct)
	jsonTags := make(map[string]struct{}, val.Type().NumField())
	for i := 0; i < val.Type().NumField(); i++ {
		if tag := val.Type().Field(i).Tag.Get("json"); tag != "" {
			jsonTags[tag] = struct{}{}
		}
	}
	return jsonTags
}

// validateQueryParams checks if query parameters are allowed for a given schema.
func validateQueryParams(w http.ResponseWriter, query map[string][]string, schemaStruct interface{}) bool {
	allowed := allowedParams(schemaStruct)
	for param := range query {
		if _, ok := allowed[param]; !ok {
			err := fmt.Errorf("invalid parameter: %s", param)
			log.Error(err)
			exceptions.RequestErrorHandler(w, err)
			return false
		}
	}
	return true
}

// validateStruct validates a struct and returns formatted error if validation fails.
func validateStruct(w http.ResponseWriter, params interface{}) bool {
	if err := schema.RequestValidate.Struct(params); err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			invalidQuery := fmt.Errorf("invalid field value in '%s': %v", e.Field(), e.Value())
			exceptions.RequestErrorHandler(w, invalidQuery)
			return false
		}
	}
	return true
}

// P2PQueryValidation validates query parameters for point-to-point requests.
func P2PQueryValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if !validateQueryParams(w, query, schema.QueryParams{}) {
			return
		}

		// Initialize active carriers
		activeCarrierCodes := make([]schema.CarrierCode, 0, 15)
		scacConfig, ok := r.Context().Value(ScheduleConfig).(map[string]interface{})["activeCarriers"].(map[string]interface{})
		if !ok {
			err := fmt.Errorf("invalid schedule configuration")
			log.Error(err)
			exceptions.RequestErrorHandler(w, err)
			return
		}

		excludedCarriers := map[schema.CarrierCode]bool{
			schema.ANNU: true,
			schema.CHNL: true,
		}

		// Process SCAC parameters
		if scacList := query["scac"]; len(scacList) > 0 {
			for _, carrierCode := range scacList {
				if active, ok := scacConfig[carrierCode].(bool); !ok || !active {
					err := fmt.Errorf("inactive or invalid SCAC: %s", carrierCode)
					log.Error(err)
					exceptions.RequestErrorHandler(w, err)
					return
				}
				activeCarrierCodes = append(activeCarrierCodes, schema.CarrierCode(carrierCode))
			}
		} else {
			for carrierCode := range scacConfig {
				if !excludedCarriers[schema.CarrierCode(carrierCode)] {
					activeCarrierCodes = append(activeCarrierCodes, schema.CarrierCode(carrierCode))
				}
			}
		}

		// Parse query parameters
		searchRange, _ := strconv.Atoi(query.Get("searchRange"))
		directOnly, _ := strconv.ParseBool(query.Get("directOnly"))
		requestParams := schema.QueryParams{
			PointFrom:     query.Get("pointFrom"),
			PointTo:       query.Get("pointTo"),
			StartDateType: schema.StartDateType(query.Get("startDateType")),
			StartDate:     query.Get("startDate"),
			SearchRange:   searchRange,
			SCAC:          activeCarrierCodes,
			DirectOnly:    directOnly,
			TSP:           query.Get("transhipmentPort"),
			VesselIMO:     query.Get("vesselIMO"),
			Service:       query.Get("service"),
		}

		if !validateStruct(w, requestParams) {
			return
		}

		ctx := context.WithValue(r.Context(), P2PQueryParamsKey, requestParams)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// VVQueryValidation validates query parameters for vessel-voyage requests.
func VVQueryValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if !validateQueryParams(w, query, schema.QueryParamsForVesselVoyage{}) {
			return
		}

		// Parse query parameters
		dateRange, _ := strconv.Atoi(query.Get("dateRange"))
		scacCarrierCodes := make([]schema.CarrierCode, 0, len(query["scac"]))
		for _, scac := range query["scac"] {
			scacCarrierCodes = append(scacCarrierCodes, schema.CarrierCode(scac))
		}

		requestParams := schema.QueryParamsForVesselVoyage{
			SCAC:      scacCarrierCodes,
			VesselIMO: query.Get("vesselIMO"),
			Voyage:    query.Get("voyageNum"),
			StartDate: query.Get("startDate"),
			DateRange: dateRange,
		}

		if !validateStruct(w, requestParams) {
			return
		}

		ctx := context.WithValue(r.Context(), VVQueryParamsKey, requestParams)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
