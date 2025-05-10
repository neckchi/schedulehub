package carrier_vessel_schedule

import (
	"fmt"
	"github.com/neckchi/schedulehub/external/interfaces"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type CarrierConfig struct {
	Name           string
	BaseURL        string
	AuthURL        string
	LocURL         string
	Method         string
	CacheDuration  time.Duration
	CacheKey       string
	RequiresAuth   bool
	AuthExpiration time.Duration
	AuthSchema     interfaces.TokenProvider
	BaseSchema     interfaces.ScheduleProvider[*schema.MasterVesselSchedule, *schema.QueryParamsForVesselVoyage]
}

// Factory for creating schedule services
type VesselScheduleServiceFactory struct {
	configs map[schema.CarrierCode]CarrierConfig
}

func NewVesselScheduleServiceFactory(e *env.Manager) *VesselScheduleServiceFactory {
	return &VesselScheduleServiceFactory{
		configs: map[schema.CarrierCode]CarrierConfig{
			schema.MAEU: {
				Name:          "MAEU",
				BaseURL:       *e.MaerskVSURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "maersk a/s vessel schedule",
				RequiresAuth:  false,
				BaseSchema:    &MaerskVesselSchedule{},
			},
			schema.MAEI: {
				Name:          "MAEI",
				BaseURL:       *e.MaerskVSURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "maersk line vessel schedule",
				RequiresAuth:  false,
				BaseSchema:    &MaerskVesselSchedule{},
			},
			schema.CMDU: {
				Name:          "CMA",
				BaseURL:       *e.CmaVVURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "cma vessel schedule",
				RequiresAuth:  false,
				BaseSchema:    &CMAVesselScheduleResponse{},
			},
			schema.APLU: {
				Name:          "APL",
				BaseURL:       *e.CmaVVURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "apl vessel schedule",
				RequiresAuth:  false,
				BaseSchema:    &CMAVesselScheduleResponse{},
			},
			schema.ANNU: {
				Name:          "ANL",
				BaseURL:       *e.CmaVVURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "anl vessel schedule",
				RequiresAuth:  false,
				BaseSchema:    &CMAVesselScheduleResponse{},
			},
			schema.CHNL: {
				Name:          "CHL",
				BaseURL:       *e.CmaVVURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "cnl vessel schedule",
				RequiresAuth:  false,
				BaseSchema:    &CMAVesselScheduleResponse{},
			},
			schema.HLCU: {
				Name:          "HAPAG",
				BaseURL:       *e.HapagVVURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "hapag vessel schedule",
				RequiresAuth:  false,
				BaseSchema:    &HapagVesselScheduleResponse{},
			},
			schema.ONEY: {
				Name:           "ONE",
				BaseURL:        *e.OneURL + "/" + "transportID",
				AuthURL:        *e.OneTURL,
				Method:         http.MethodGet,
				CacheDuration:  6 * time.Hour,
				CacheKey:       "one vessel schedule",
				RequiresAuth:   true,
				AuthExpiration: 55 * time.Minute,
				AuthSchema:     &OneVesselSchedule{},
				BaseSchema:     &OneVesselSchedule{},
			},

			// Add more carriers  here
		},
	}
}

func (f *VesselScheduleServiceFactory) CreateVesselScheduleService(carrier schema.CarrierCode) (interfaces.Schedule[*schema.MasterVesselSchedule, *schema.QueryParamsForVesselVoyage], error) {
	config, exists := f.configs[carrier]
	if !exists {
		log.Errorf("unsupported carrier: %s", carrier)
		return nil, fmt.Errorf("unsupported carrier: %s", carrier)
	}

	var auth *interfaces.OAuth2
	if config.RequiresAuth {
		auth = &interfaces.OAuth2{
			TokenUrl:    config.AuthURL,
			Method:      http.MethodPost,
			Secrets:     config.AuthSchema,
			TokenExpiry: config.AuthExpiration,
			Namespace:   fmt.Sprintf("%s token", config.Name),
		}
	}

	scheduleConfig := interfaces.ScheduleConfig{
		ScheduleURL:    config.BaseURL,
		Method:         config.Method,
		ScheduleExpiry: config.CacheDuration,
		Namespace:      config.CacheKey,
	}

	genericScheduleService := &interfaces.ScheduleService[*schema.MasterVesselSchedule, *schema.QueryParamsForVesselVoyage]{Token: auth, ScheduleConfig: scheduleConfig, ScheduleProvider: config.BaseSchema}
	return genericScheduleService, nil
}
