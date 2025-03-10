package routers

import (
	"github.com/neckchi/schedulehub/internal/database"
	"github.com/neckchi/schedulehub/internal/handlers"
	"github.com/neckchi/schedulehub/internal/middleware"
	env "github.com/neckchi/schedulehub/internal/secret"
	"net/http"
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
	middlewareStackForvv := middleware.CreateStack(middleware.Recovery, middleware.CheckCORS, middleware.AddCorrelationID, middleware.AddHeaders, middleware.VVQueryValidation, middleware.Logging)
	voyageRouter := http.NewServeMux()
	//Master Vessel Voyage
	vv := middlewareStackForvv(handlers.VoyageHandler(oracle))
	voyageRouter.Handle("GET /schedules/vv", vv)
	return voyageRouter
}
