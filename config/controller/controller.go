package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"schedulehub/config/domain"
	"schedulehub/internal/exceptions"
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
