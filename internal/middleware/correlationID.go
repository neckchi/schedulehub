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
		correlationID := r.Header.Get(string(correlationIDKey))
		if correlationID == "" {
			correlationID = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), correlationIDKey, correlationID)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
