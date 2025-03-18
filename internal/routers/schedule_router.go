package routers

import (
	"github.com/neckchi/schedulehub/external"
	"github.com/neckchi/schedulehub/internal/database"
	"github.com/neckchi/schedulehub/internal/handlers"
	"github.com/neckchi/schedulehub/internal/http"
	"github.com/neckchi/schedulehub/internal/middleware"
	"github.com/neckchi/schedulehub/internal/secret"
	"net/http"
	"time"
)

func ScheduleRouter() http.Handler {
	envManager, err := env.NewManager()
	if err != nil {
		panic(err)
	}
	externalApiConfig := external.NewScheduleServiceFactory(envManager)
	redisSettings := database.RedisSettings{
		DB:         envManager.RedisDb,
		DBUser:     envManager.RedisUser,
		DBPassword: envManager.RedisPw,
		Host:       envManager.RedisHost,
		Port:       envManager.RedisPort,
		Protocol:   envManager.RedisPrtl,
	}
	redis, err := database.NewRedisConnection(redisSettings)
	if err != nil {
		panic(err)
	}
	//We cant change any connection pool config without restarting the server so we have to change them by request if necessary.
	httpClient := httpclient.CreateHttpClientInstance(redis, httpclient.WithCtxTimeout(7*time.Second),
		httpclient.WithMaxRetries(2), httpclient.WithRetryDelay(2*time.Second),
		httpclient.WithMaxIdleConns(200), httpclient.WithMaxConnsPerHost(200), httpclient.WithMaxIdleConnsPerHost(200),
		httpclient.WithIdleConnTimeout(90), httpclient.WithDisableKeepAlives(false))
	middlewareStackForp2p := middleware.CreateStack(middleware.Recovery, middleware.CheckCORS,
		middleware.AddCorrelationID, middleware.AddHeaders, middleware.GetAppConfig, middleware.P2PQueryValidation, middleware.Logging)
	middlewareStackForhc := middleware.CreateStack(middleware.Recovery, middleware.AddCorrelationID, middleware.AddHeaders, middleware.Logging)
	router := http.NewServeMux()
	//HealthCheck
	hc := middlewareStackForhc(handlers.HealthCheckHandler())
	//P2P schedule
	sh := middlewareStackForp2p(handlers.P2PScheduleHandler(httpClient, envManager, externalApiConfig, redis))
	router.Handle("GET /schedules/p2p", sh)
	router.Handle("GET /health", hc)
	return router
}
