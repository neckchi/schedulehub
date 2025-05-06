package mvs_handler

import (
	"context"
	"fmt"
	"github.com/neckchi/schedulehub/external/carrier_vessel_schedule"
	"github.com/neckchi/schedulehub/internal/database"
	"github.com/neckchi/schedulehub/internal/exceptions"
	"github.com/neckchi/schedulehub/internal/http"
	"github.com/neckchi/schedulehub/internal/middleware"
	"github.com/neckchi/schedulehub/internal/schema"
	env "github.com/neckchi/schedulehub/internal/secret"
	"github.com/neckchi/schedulehub/internal/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type VoyageService struct {
	client *httpclient.HttpClient
	env    *env.Manager
	vs     *carrier_vessel_schedule.VesselScheduleServiceFactory
	oracle database.OracleRepository
	redis  database.RedisRepository
}

func NewVoyageService(
	client *httpclient.HttpClient,
	env *env.Manager,
	vs *carrier_vessel_schedule.VesselScheduleServiceFactory,
	oracle database.OracleRepository,
	redis database.RedisRepository,
) *VoyageService {
	return &VoyageService{client, env, vs, oracle, redis}
}

func VoyageHandler(s *VoyageService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fw := utils.NewFlushWriter(w)
		queryParams, _ := r.Context().Value(middleware.VVQueryParamsKey).(schema.QueryParamsForVesselVoyage)
		scacConfig, ok := r.Context().Value(middleware.ScheduleConfig).(map[string]interface{})["externalAPICarriers"].(map[string]interface{})
		if !ok {
			err := fmt.Errorf("invalid schedule configuration")
			log.Error(err)
			exceptions.RequestErrorHandler(w, err)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()         // Ensure cancellation when function exits
		done := make(chan int) // this is going to ensure that our goroutine are shut down in the event that we call done from the P2PScheduleHandler function
		defer close(done)
		mvsService := NewMastervVesselVoyageService(ctx, s.oracle, done, s.client, s.env, s.vs, &queryParams, scacConfig)
		fanoutMVSChannels := mvsService.FanOutMVSChannels()
		fannedInStream := mvsService.FanInMasterVesselSchedule(fanoutMVSChannels...)
		mvsService.StreamMasterVesselSchedule(fw, fannedInStream)
		go func() {
			err := s.redis.Set(r.URL.String())
			if err != nil {
				exceptions.InternalErrorHandler(w, err)
			}
		}()
	})
}
