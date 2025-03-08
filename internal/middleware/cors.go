package middleware

import (
	"net/http"
	"slices"
	"strings"
)

//Good article! https://eli.thegreenplace.net/2023/introduction-to-cors-for-go-programmers/

var originAllowlist = []string{"*"}

var methodAllowlist = []string{"GET", "OPTIONS"}

func isPreflight(r *http.Request) bool {
	return r.Method == "OPTIONS" &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}

func CheckCORS(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if isPreflight(r) {
			origin := r.Header.Get("Origin")
			method := r.Header.Get("Access-Control-Request-Method")
			if slices.Contains(originAllowlist, origin) && slices.Contains(methodAllowlist, method) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(methodAllowlist, ", "))
			}
		} else {
			origin := r.Header.Get("Origin")
			if slices.Contains(originAllowlist, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}
		w.Header().Add("Vary", "Origin")
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
