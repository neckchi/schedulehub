package controller

import (
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/config/domain"
	"github.com/neckchi/schedulehub/internal/exceptions"
	"net/http"
)

type Controller struct {
	Config *domain.Config
}

func (c *Controller) ReadConfig() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceName := r.PathValue("serviceName")
		config, err := c.Config.Get(serviceName)
		if err != nil {
			exceptions.InternalErrorHandler(w, err)
		}
		rsp, err := json.Marshal(&config)
		if err != nil {
			exceptions.InternalErrorHandler(w, err)
		}
		fmt.Fprintf(w, string(rsp))
	})
}
