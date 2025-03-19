package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/neckchi/schedulehub/internal/exceptions"
	"net/http"
)

func HealthCheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseBody := map[string]string{"message": "Health check successful"}
		responseJSON, err := json.Marshal(responseBody)
		if err != nil {
			failedCheck := fmt.Errorf("health check failed in json marshal %s", err)
			exceptions.InternalErrorHandler(w, failedCheck)
		}
		_, _ = w.Write(responseJSON)
	})
}
