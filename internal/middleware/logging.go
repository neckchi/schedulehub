package middleware

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"sync"
	"time"
)

type CustomLogFormatter struct {
	log.TextFormatter
	correlationID string
}

// override the TextFormatter.Format method as we need the customFormat in logging.
func (f *CustomLogFormatter) Format(entry *log.Entry) ([]byte, error) {
	var fields string
	timestamp := entry.Time.Format("2006-01-02T15:04:05.000-07:00")

	msg := entry.Message
	if len(entry.Data) > 0 {
		fieldStr := make([]string, 0, len(entry.Data))
		for k, v := range entry.Data {
			fieldStr = append(fieldStr, fmt.Sprintf("%v=%v", k, v))
		}
		fields = " " + strings.Join(fieldStr, " ")
	}

	logMessage := fmt.Sprintf("%s %s %s %s %s\n",
		timestamp,
		strings.ToUpper(entry.Level.String()),
		f.correlationID,
		" "+msg,
		fields,
	)

	return []byte(logMessage), nil
}

type extendWriter struct {
	http.ResponseWriter
	statusCode int
}

func (e *extendWriter) WriteHeader(statusCode int) {
	e.ResponseWriter.WriteHeader(statusCode)
	e.statusCode = statusCode
}

var (
	customizeOnce   sync.Once
	customFormatter *CustomLogFormatter
)

func Logging(next http.Handler) http.Handler {
	customizeOnce.Do(func() {
		customFormatter = &CustomLogFormatter{}
		log.SetFormatter(customFormatter)
	})
	fn := func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		cid := r.Context().Value(correlationIDKey).(string)
		customFormatter.correlationID = cid
		extendedWriter := &extendWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(extendedWriter, r)
		log.Info(r.Method, r.URL, extendedWriter.statusCode, time.Since(startTime))
	}
	return http.HandlerFunc(fn)
}
