package helpers

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	"golang.org/x/crypto/bcrypt"
)

func CreateJWTString(c *models.Config, userid string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, models.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(c.JWTTokenTTL)),
		},
		UserID: userid,
	})

	// создаём строку токена
	tokenString, err := token.SignedString([]byte(c.JWTKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// возвращаем строку токена
	return tokenString, nil
}

func ValidateJWT(c *models.Config, tokenString string) (*models.Claims, error) {
	claims := &models.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(c.JWTKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}
	return claims, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("failed to hash password")
	}
	return string(bytes), nil
}
