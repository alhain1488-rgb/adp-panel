package auth

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey int

const adminIDKey ctxKey = iota

// RequireAuth is middleware that rejects requests without a valid access token
// and puts the admin id into the request context.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			unauthorized(w)
			return
		}
		id, err := s.ParseAccess(token)
		if err != nil {
			unauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), adminIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminIDFrom extracts the authenticated admin id from the context.
func AdminIDFrom(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(adminIDKey).(int64)
	return id, ok
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"authentication required"}`))
}
