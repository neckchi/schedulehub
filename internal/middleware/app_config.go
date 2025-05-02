package middleware

import (
	"context"
	"github.com/neckchi/schedulehub/configs/controller"
	"github.com/neckchi/schedulehub/configs/domain"
	"github.com/neckchi/schedulehub/configs/service"
	"github.com/neckchi/schedulehub/internal/exceptions"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	configOnce sync.Once
	config     domain.Config
)

type appConfig string

const ScheduleConfig appConfig = "scheduleConfig"

func GetAppConfig(path string) func(http.Handler) http.Handler {
	configOnce.Do(func() {
		currentDir, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to setup config: %v", err)
		}
		configService := service.ConfigService{
			Config:   &config,
			Location: filepath.Join(currentDir, "config.yaml"),
		}
		go configService.Watch(time.Minute * 5)
	})
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			// this will be called once. it wont spawn as many goroutine as the incoming request
			// And the gorouinte will keep reading the config yaml every 5 mins.
			//We can consider link all app secret and  config to AWS PS standard tier with the sync.once func
			c := controller.Controller{
				Config: &config,
			}
			result, _ := c.Config.Get(path)
			if err != nil {
				exceptions.InternalErrorHandler(w, err)
				return
			}
			ctx := context.WithValue(r.Context(), ScheduleConfig, result)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)

		})
	}
}
