package middleware

import (
	"context"
	"github.com/google/uuid"
	"net/http"
)

type correlateContextKey string

const correlationIDKey correlateContextKey = "X-Correlation-ID"

func AddCorrelationID(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var correlationID string
		cidFromRequest := r.Header.Get(string(correlationIDKey))
		if cidFromRequest == "" {
			correlationID = uuid.New().String()
		} else {
			correlationID = cidFromRequest
		}
		ctx := context.WithValue(r.Context(), correlationIDKey, correlationID)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
