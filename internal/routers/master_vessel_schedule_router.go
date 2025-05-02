package routers

import (
	"github.com/neckchi/schedulehub/internal/dependencies"
	"github.com/neckchi/schedulehub/internal/handlers/mvs_handler"
	"github.com/neckchi/schedulehub/internal/middleware"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func VoyageRouter() http.Handler {
	deps, err := dependencies.NewDependencies()
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize dependencies")
		return nil
	}
	middlewareStackForMVS := middleware.CreateStack(
		middleware.Recovery,
		middleware.CheckCORS,
		middleware.AddCorrelationID,
		middleware.AddHeaders,
		middleware.GetAppConfig("service.registry.mvs"),
		middleware.Logging,
		middleware.VVQueryValidation,
	)

	voyageService := mvs_handler.NewVoyageService(
		deps.HTTPClient,
		deps.EnvManager,
		deps.VesselSvc,
		deps.OracleDB,
		deps.RedisDB,
	)

	voyageRouter := http.NewServeMux()

	vv := middlewareStackForMVS(mvs_handler.VoyageHandler(voyageService))
	voyageRouter.Handle("GET /schedules/mastervoyage", vv)
	return voyageRouter
}
