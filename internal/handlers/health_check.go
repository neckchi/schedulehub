package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"schedulehub/internal/exceptions"
)

func HealthCheckHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseBody := map[string]string{"message": "Health check successful"}
		responseJSON, err := json.Marshal(responseBody)
		if err != nil {
			failedCheck := fmt.Errorf("health check failed: %s", err)
			exceptions.InternalErrorHandler(w, failedCheck)
		}
		w.Write(responseJSON)
	})
}
