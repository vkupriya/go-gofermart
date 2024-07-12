package middleware

import (
	"context"
	"net/http"

	"github.com/vkupriya/go-gophermart/internal/gophermart/helpers"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

type CtxKey struct{}

type MiddlewareAuth struct {
	config *models.Config
}

func NewMiddlewareAuth(c *models.Config) *MiddlewareAuth {
	return &MiddlewareAuth{
		config: c,
	}
}

func (m *MiddlewareAuth) Auth(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		// logger := m.config.Logger
		tokenStr := r.Header.Get("Authorization")

		if tokenStr == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		tokenStr = tokenStr[len("Bearer "):]

		claims, err := helpers.ValidateJWT(m.config, tokenStr)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), CtxKey{}, claims.User)
		h.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(logFn)
}
