package routers

import (
	"net/http"
	"schedulehub/internal/database"
	"schedulehub/internal/handlers"
	"schedulehub/internal/middleware"
	env "schedulehub/internal/secret"
)

func VoyageRouter() http.Handler {
	envManager, _ := env.NewManager()
	oracleSetting := database.OracleSettings{
		DBUser:      envManager.DbUser,
		DBPassword:  envManager.DbPw,
		Host:        envManager.Host,
		Port:        envManager.Port,
		ServiceName: envManager.ServiceName,
	}
	oracle, _ := database.NewOracleDBConnectionPool(oracleSetting, 20, 3)
	middlewareStackForvv := middleware.CreateStack(middleware.CheckCORS, middleware.AddCorrelationID, middleware.AddHeaders, middleware.VVQueryValidation, middleware.Logging)
	voyageRouter := http.NewServeMux()
	//Master Vessel Voyage
	vv := middlewareStackForvv(handlers.VoyageHandler(oracle))
	voyageRouter.Handle("GET /schedules/vv", vv)
	return voyageRouter
}
