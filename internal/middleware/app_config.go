package middleware

import (
	"context"
	"github.com/neckchi/schedulehub/configs/controller"
	"github.com/neckchi/schedulehub/configs/domain"
	"github.com/neckchi/schedulehub/internal/exceptions"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
)

func GetAppConfig(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		config := domain.Config{}
		currentDir, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to setup config: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(currentDir, "config.yaml"))
		if err != nil {
			exceptions.InternalErrorHandler(w, err)
			return
		}
		err = config.SetFromBytes(data)
		if err != nil {
			exceptions.InternalErrorHandler(w, err)
			return
		}
		c := controller.Controller{
			Config: &config,
		}
		result, _ := c.Config.Get("service.registry.schedule")
		if err != nil {
			exceptions.InternalErrorHandler(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), "appConfig", result)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
