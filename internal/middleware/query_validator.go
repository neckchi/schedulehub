package middleware

import (
	"context"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/neckchi/schedulehub/internal/exceptions"
	"github.com/neckchi/schedulehub/internal/schema"
	log "github.com/sirupsen/logrus"
	"net/http"
	"reflect"
	"strconv"
)

type queryContextKey string

const (
	P2PQueryParamsKey queryContextKey = "p2PQueryParams"
	VVQueryParamsKey  queryContextKey = "VVQueryParams"
)

var allowParamsList = func(schemaStruct interface{}) map[string]bool {
	val := reflect.ValueOf(schemaStruct)
	var jsonTags = make(map[string]bool)
	for i := 0; i < val.Type().NumField(); i++ {
		jsonTag := val.Type().Field(i).Tag.Get("json")
		jsonTags[jsonTag] = true
	}
	return jsonTags
}

func pointerIf[T any](cond bool, val T) *T {
	if cond {
		return &val
	}
	return nil
}

func P2PQueryValidation(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		for params := range query {
			if !allowParamsList(schema.QueryParams{})[params] {
				wrongParmeters := fmt.Errorf("wrong parmeters provided: %s", params)
				log.Error(wrongParmeters)
				exceptions.RequestErrorHandler(w, wrongParmeters)
				return
			}
		}

		activeCarrierCodes := make([]schema.CarrierCode, 0, 15)
		scacConfig := r.Context().Value(scheduleConfig).(map[string]interface{})["activeCarriers"]
		excludedCarriers := map[schema.CarrierCode]bool{
			schema.ANNU: true,
			schema.CHNL: true,
		}
		switch scacList := query["scac"]; len(scacList) {
		case 0:
			for carrierCode := range scacConfig.(map[string]interface{}) {
				if !excludedCarriers[schema.CarrierCode(carrierCode)] {
					activeCarrierCodes = append(activeCarrierCodes, schema.CarrierCode(carrierCode))
				}
			}
		default:
			for _, carrierCode := range scacList {
				active, exist := scacConfig.(map[string]interface{})[carrierCode].(bool)
				if active && exist {
					activeCarrierCodes = append(activeCarrierCodes, schema.CarrierCode(carrierCode))
				} else {
					inactiveCarrierCodes := fmt.Errorf("inactive scac provided: %s", carrierCode)
					exceptions.RequestErrorHandler(w, inactiveCarrierCodes)
					return
				}
			}

		}

		pointFrom := query.Get("pointFrom")
		pointTo := query.Get("pointTo")
		startDateType := schema.StartDateType(query.Get("startDateType"))
		startDate := query.Get("startDate")
		searchRange, _ := strconv.Atoi(query.Get("searchRange"))

		requestParams := schema.QueryParams{
			PointFrom:     pointFrom,
			PointTo:       pointTo,
			StartDateType: startDateType,
			StartDate:     startDate,
			SearchRange:   searchRange,
			SCAC:          &activeCarrierCodes,
		}

		directOnly, _ := strconv.ParseBool(query.Get("directOnly"))
		requestParams.DirectOnly = pointerIf[bool](query.Has("directOnly"), directOnly)
		requestParams.Service = pointerIf[string](query.Has("service"), query.Get("service"))
		requestParams.TSP = pointerIf[string](query.Has("transhipmentPort"), query.Get("transhipmentPort"))
		requestParams.VesselIMO = pointerIf[string](query.Has("vesselIMO"), query.Get("vesselIMO"))

		if err := schema.RequestValidate.Struct(requestParams); err != nil {
			var errorField string
			var errorValue any
			for _, err := range err.(validator.ValidationErrors) {
				errorField = err.Field()
				errorValue = err.Value()
			}
			invalidQuery := fmt.Errorf("invalid field value found in '%v' : %v ", errorField, errorValue)
			exceptions.RequestErrorHandler(w, invalidQuery)
			return
		}
		ctx := context.WithValue(r.Context(), P2PQueryParamsKey, requestParams)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func VVQueryValidation(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		for params := range query {
			if !allowParamsList(schema.QueryParamsForVesselVoyage{})[params] {
				wrongParmeters := fmt.Errorf("wrong parmeters provided: %s", params)
				log.Error(wrongParmeters)
				exceptions.RequestErrorHandler(w, wrongParmeters)
				return
			}
		}

		scac := schema.CarrierCode(query.Get("scac"))
		vesselIMO := query.Get("vesselIMO")
		voyage := query.Get("voyageNum")
		requestParams := schema.QueryParamsForVesselVoyage{
			SCAC:      scac,
			VesselIMO: vesselIMO,
			Voyage:    &voyage,
		}

		requestParams.PointFrom = pointerIf[string](query.Has("pointFrom"), query.Get("pointFrom"))
		requestParams.PointTo = pointerIf[string](query.Has("pointTo"), query.Get("pointTo"))
		requestParams.StartDate = pointerIf[string](query.Has("startDate"), query.Get("startDate"))

		if err := schema.RequestValidate.Struct(requestParams); err != nil {
			var errorField string
			var errorValue any
			for _, err := range err.(validator.ValidationErrors) {
				errorField = err.Field()
				errorValue = err.Value()
			}
			invalidQuery := fmt.Errorf("invalid field value found in '%v' : %v ", errorField, errorValue)
			exceptions.RequestErrorHandler(w, invalidQuery)
			return
		}
		ctx := context.WithValue(r.Context(), VVQueryParamsKey, requestParams)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
