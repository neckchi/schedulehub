package routers

import (
	"github.com/neckchi/schedulehub/internal/handlers"
	"github.com/neckchi/schedulehub/internal/middleware"
	"net/http"
)

func HealthCheckRouter() http.Handler {
	middlewareStackForhc := middleware.CreateStack(middleware.Recovery, middleware.AddCorrelationID, middleware.Logging, middleware.AddHeaders)
	hc := middlewareStackForhc(handlers.HealthCheckHandler())
	healthCheckRouter := http.NewServeMux()
	healthCheckRouter.Handle("GET /health", hc)
	return healthCheckRouter
}
