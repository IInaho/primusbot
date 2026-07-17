package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func WithBearerAuth(next http.Handler, token string) http.Handler {
	token = strings.TrimSpace(token)
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if authorized(r, token) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="nekocode"`)
		writeError(w, http.StatusUnauthorized, "unauthorized")
	})
}

func authorized(r *http.Request, token string) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		candidate := strings.TrimSpace(auth[len("bearer "):])
		return subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1
	}
	candidate := strings.TrimSpace(r.Header.Get("X-Nekocode-Token"))
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1
}
