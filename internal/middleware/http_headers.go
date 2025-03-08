package middleware

import (
	"net/http"
)

func AddHeaders(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Context().Value(correlationIDKey).(string)
		headers := map[string]string{
			"Connection":    "Keep-Alive",
			"Cache-Control": "max-age=7200,stale-while-revalidate=86400",
			"Content-Type":  "application/json",
			//"Content-Type": "text/event-stream",
			//"Context-Type":     "application/x-ndjson; charset=utf-8",
			"X-Correlation-ID": correlationID,
		}
		for key, value := range headers {
			w.Header().Set(key, value)
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
