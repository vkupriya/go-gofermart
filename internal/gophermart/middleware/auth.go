package middleware

import (
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v4"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

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
		//logger := m.config.Logger
		fmt.Println("Key: ", m.config.KeyJWT)
		claims := &models.Claims{}
		tokenStr := r.Header.Get("Authorization")
		fmt.Println("Token: ", tokenStr)
		if tokenStr == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		tokenStr = tokenStr[len("Bearer "):]
		fmt.Println("token string: ", tokenStr)

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(m.config.KeyJWT), nil
		})
		fmt.Println("Error: ", err)
		fmt.Println("Username: ", claims.Username)
		fmt.Println("Token valid? :", token.Valid)
		if err != nil || !token.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	}
	return http.HandlerFunc(logFn)
}
