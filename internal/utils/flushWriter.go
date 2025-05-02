package utils

import "net/http"

type FlushWriter struct {
	http.ResponseWriter
}

func (fw FlushWriter) Flush() {
	if flusher, ok := fw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func NewFlushWriter(w http.ResponseWriter) FlushWriter {
	return FlushWriter{ResponseWriter: w}
}
