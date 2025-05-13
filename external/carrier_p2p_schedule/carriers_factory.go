package carrier_p2p_schedule

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
	Name             string
	BaseURL          string
	AuthURL          string
	LocURL           string
	Method           string
	LocationDuration time.Duration
	LocationKey      string
	RequiresLocation bool `default:"false"`
	CacheDuration    time.Duration
	CacheKey         string
	RequiresAuth     bool
	AuthExpiration   time.Duration
	AuthSchema       interfaces.TokenProvider
	LocSchema        interfaces.LocationProvider
	BaseSchema       interfaces.ScheduleProvider[[]*schema.P2PSchedule, *schema.QueryParams]
}

// Factory for creating schedule services
type P2PScheduleServiceFactory struct {
	configs map[schema.CarrierCode]CarrierConfig
}

func NewP2PScheduleServiceFactory(e *env.Manager) *P2PScheduleServiceFactory {
	return &P2PScheduleServiceFactory{
		configs: map[schema.CarrierCode]CarrierConfig{
			schema.ZIMU: {
				Name:           "ZIM",
				BaseURL:        *e.ZimURL,
				AuthURL:        *e.ZimTURL,
				Method:         http.MethodGet,
				CacheDuration:  6 * time.Hour,
				CacheKey:       "zim schedule",
				RequiresAuth:   true,
				AuthExpiration: 55 * time.Minute,
				AuthSchema:     &ZimScheduleResponse{},
				BaseSchema:     &ZimScheduleResponse{},
			},
			schema.ONEY: {
				Name:           "ONE DCSA",
				BaseURL:        *e.OneDCSAURL,
				AuthURL:        *e.OneTURL,
				Method:         http.MethodGet,
				CacheDuration:  6 * time.Hour,
				CacheKey:       "one dcsa schedule",
				RequiresAuth:   true,
				AuthExpiration: 55 * time.Minute,
				AuthSchema:     &OneDCSAScheduleResponse{},
				BaseSchema:     &OneDCSAScheduleResponse{},
			},
			schema.MSCU: {
				Name:           "MSC",
				BaseURL:        *e.MscURL,
				AuthURL:        *e.MscOauth,
				Method:         http.MethodGet,
				CacheDuration:  6 * time.Hour,
				CacheKey:       "msc schedule",
				RequiresAuth:   true,
				AuthExpiration: 55 * time.Minute,
				AuthSchema:     &MscScheduleResponse{},
				BaseSchema:     &MscScheduleResponse{},
			},
			schema.CMDU: {
				Name:             "CMA",
				BaseURL:          *e.CmaURL,
				Method:           http.MethodGet,
				CacheDuration:    6 * time.Hour,
				CacheKey:         "cma schedule",
				RequiresAuth:     false,
				RequiresLocation: false,
				BaseSchema:       &CmaScheduleResponse{},
			},
			schema.APLU: {
				Name:          "APL",
				BaseURL:       *e.CmaURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "apl schedule",
				RequiresAuth:  false,
				BaseSchema:    &CmaScheduleResponse{},
			},
			schema.ANNU: {
				Name:          "ANL",
				BaseURL:       *e.CmaURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "anl schedule",
				RequiresAuth:  false,
				BaseSchema:    &CmaScheduleResponse{},
			},
			schema.CHNL: {
				Name:          "CHL",
				BaseURL:       *e.CmaURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "cnl schedule",
				RequiresAuth:  false,
				BaseSchema:    &CmaScheduleResponse{},
			},
			schema.HLCU: {
				Name:          "HAPAG",
				BaseURL:       *e.HapagURL,
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "hapag schedule",
				RequiresAuth:  false,
				BaseSchema:    &HapagScheduleResponse{},
			},
			schema.COSU: {
				Name:          "Cosco",
				BaseURL:       *e.IqaxURL + "/" + string(schema.COSU),
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "cosco schedule",
				RequiresAuth:  false,
				BaseSchema:    &IqaxScheduleResponse{},
			},
			schema.OOLU: {
				Name:          "OOCL",
				BaseURL:       *e.IqaxURL + "/" + string(schema.OOLU),
				Method:        http.MethodGet,
				CacheDuration: 6 * time.Hour,
				CacheKey:      "oocl schedule",
				RequiresAuth:  false,
				BaseSchema:    &IqaxScheduleResponse{},
			},
			schema.MAEU: {
				Name:             "MAEU",
				BaseURL:          *e.MaerskP2PURL,
				LocURL:           *e.MaerskLocURL,
				Method:           http.MethodGet,
				CacheDuration:    6 * time.Hour,
				CacheKey:         "maersk a/s schedule",
				LocationDuration: 8000 * time.Hour,
				LocationKey:      "maersk location",
				RequiresLocation: true,
				RequiresAuth:     false,
				BaseSchema:       &MaerskScheduleResponse{},
				LocSchema:        &MaerskScheduleResponse{},
			},
			schema.MAEI: {
				Name:             "MAEI",
				BaseURL:          *e.MaerskP2PURL,
				LocURL:           *e.MaerskLocURL,
				Method:           http.MethodGet,
				CacheDuration:    6 * time.Hour,
				CacheKey:         "maersk line schedule",
				LocationDuration: 8000 * time.Hour,
				LocationKey:      "maersk location",
				RequiresLocation: true,
				RequiresAuth:     false,
				BaseSchema:       &MaerskScheduleResponse{},
				LocSchema:        &MaerskScheduleResponse{},
			},

			// Add more carriers  here
		},
	}
}

func (f *P2PScheduleServiceFactory) CreateScheduleService(carrier schema.CarrierCode) (interfaces.Schedule[[]*schema.P2PSchedule, *schema.QueryParams], error) {
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

	var loc *interfaces.LocationService
	if config.RequiresLocation {
		loc = &interfaces.LocationService{
			LocationUrl:    config.LocURL,
			Method:         config.Method,
			Secrets:        config.LocSchema,
			LocationExpiry: config.LocationDuration,
			Namespace:      config.LocationKey,
		}
	}

	scheduleConfig := interfaces.ScheduleConfig{
		ScheduleURL:    config.BaseURL,
		Method:         config.Method,
		ScheduleExpiry: config.CacheDuration,
		Namespace:      config.CacheKey,
	}

	genericScheduleService := &interfaces.ScheduleService[[]*schema.P2PSchedule, *schema.QueryParams]{Token: auth, Location: loc, ScheduleConfig: scheduleConfig, ScheduleProvider: config.BaseSchema}
	return genericScheduleService, nil
}
