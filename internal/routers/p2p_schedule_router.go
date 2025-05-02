package routers

import (
	"github.com/neckchi/schedulehub/internal/dependencies"
	"github.com/neckchi/schedulehub/internal/handlers/p2p_schedule_handler"
	"github.com/neckchi/schedulehub/internal/middleware"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func ScheduleRouter() http.Handler {
	deps, err := dependencies.NewDependencies()
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize dependencies")
		return nil
	}
	middlewareStackForp2p := middleware.CreateStack(
		middleware.Recovery,
		middleware.CheckCORS,
		middleware.AddCorrelationID,
		middleware.AddHeaders,
		middleware.GetAppConfig("service.registry.p2p"),
		middleware.Logging,
		middleware.P2PQueryValidation,
	)
	p2pService := p2p_schedule_handler.NewP2PScheduleService(
		deps.HTTPClient,
		deps.EnvManager,
		deps.P2PSvc,
		deps.RedisDB,
	)

	p2pScheduleRouter := http.NewServeMux()
	sh := middlewareStackForp2p(p2p_schedule_handler.P2PScheduleHandler(p2pService))
	p2pScheduleRouter.Handle("GET /schedules/p2p", sh)
	//HealthCheck

	return p2pScheduleRouter
}
