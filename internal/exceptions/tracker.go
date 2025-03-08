package exceptions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical"
	SeverityError    SeverityLevel = "error"
	SeverityWarning  SeverityLevel = "warning"
	SeverityInfo     SeverityLevel = "info"
)

type ErrorTracker struct {
	mu    sync.Mutex
	count map[string]int
}

var errorTracker = ErrorTracker{count: make(map[string]int)}

type ErrorDetail struct {
	Message   string        `json:"message"`
	Count     int           `json:"count"`
	Severity  SeverityLevel `json:"severity"`
	Timestamp string        `json:"timestamp"`
}

type ErrorResponse struct {
	Errors []ErrorDetail `json:"errors"`
}

func trackError(err error, severity SeverityLevel) ErrorDetail {
	errorTracker.mu.Lock()
	errorTracker.count[err.Error()]++
	count := errorTracker.count[err.Error()]
	errorTracker.mu.Unlock()

	return ErrorDetail{
		Message:   err.Error(),
		Count:     count,
		Severity:  severity,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func parseValidationErrors(err error) []error {
	var validationErrors []error
	errorLines := strings.Split(err.Error(), "\n")
	for _, line := range errorLines {
		parts := strings.SplitN(line, " Error:", 2)
		if len(parts) == 2 {
			field := strings.TrimPrefix(parts[0], "Key: ")
			message := strings.TrimSpace(parts[1])
			err := fmt.Errorf("%s %s", field, message) // Create a formatted error message
			validationErrors = append(validationErrors, err)
		}
	}
	return validationErrors
}

func writeError(w http.ResponseWriter, errs []error, severity SeverityLevel, code int) {
	var errorsOccurred = make([]ErrorDetail, 0, len(errs))
	for _, err := range errs {
		errorsOccurred = append(errorsOccurred, trackError(err, severity))
	}

	for _, err := range errorsOccurred {
		if err.Count > 5 && err.Severity == "critical" {
			log.Fatalf("ALERT: High occurrence of critical error - %s (Count: %d)\n", err.Message, err.Count)
		}
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Errors: errorsOccurred})
}

var (
	RequestErrorHandler = func(w http.ResponseWriter, err error) {
		log.Error(err)
		writeError(w, []error{err}, SeverityError, http.StatusBadRequest)
	}
	InternalErrorHandler = func(w http.ResponseWriter, err error) {
		log.Error(err)
		writeError(w, []error{err}, SeverityError, http.StatusInternalServerError)

	}
	ValidationErrorHandler = func(w http.ResponseWriter, err error) {
		validationErrors := parseValidationErrors(err)
		log.Error(err)
		writeError(w, validationErrors, SeverityCritical, http.StatusUnprocessableEntity)

	}
)
