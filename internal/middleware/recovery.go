package middleware

import (
	"fmt"
	"github.com/neckchi/schedulehub/internal/exceptions"
	log "github.com/sirupsen/logrus"
	"net/http"
	"runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("Caught Panic : %v ,Stack Trace: %s", err, string(debug.Stack()))
				caughtPanic := fmt.Errorf("Caught Panic : %v ,Stack Trace: %s", err, string(debug.Stack()))
				exceptions.InternalErrorHandler(w, caughtPanic)
				return
			}
		}()
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
