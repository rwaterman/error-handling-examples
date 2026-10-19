package app

import (
	"context"
	"net/http"
	"regexp"
	"uuid"
)

const requestIDHeader = "X-Request-Id"

var trustedRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_=-]{1,255}$`)

type requestIDKey struct{}

// withRequestID accepts the caller's request ID only when it matches
// trustedRequestIDPattern, and otherwise generates one. It sets the response
// header before calling next, so every response carries it.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := trustedRequestID(r.Header.Get(requestIDHeader))
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func trustedRequestID(candidate string) string {
	if trustedRequestIDPattern.MatchString(candidate) {
		return candidate
	}
	return uuid.New().String()
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
