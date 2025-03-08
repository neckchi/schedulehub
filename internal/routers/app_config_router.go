package routers

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"schedulehub/config/controller"
	"schedulehub/config/domain"
	"schedulehub/config/service"
	"schedulehub/internal/middleware"
	"time"
)

func AppConfigRouter() http.Handler {
	config := domain.Config{}
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to setup config: %v", err)
	}
	configService := service.ConfigService{
		Config:   &config,
		Location: filepath.Join(currentDir, "config.yaml"),
	}
	go configService.Watch(time.Second * 3)
	c := controller.Controller{
		Config: &config,
	}
	//c := SetupConfig()
	middlewareStackForrc := middleware.CreateStack(middleware.CheckCORS, middleware.AddCorrelationID, middleware.AddHeaders, middleware.Logging)
	appConfigRouter := http.NewServeMux()
	rc := middlewareStackForrc(c.ReadConfig())
	appConfigRouter.Handle("GET /read/{serviceName}", rc)
	return appConfigRouter
}
