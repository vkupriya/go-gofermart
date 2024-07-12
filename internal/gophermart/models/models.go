package models

import (
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type Config struct {
	Logger      *zap.Logger
	Address     string
	PostgresDSN string
	KeyJWT      string
}

type Orders []Order

type Order struct {
	UserID  string `json:"user"`
	OrderID string `json:"order"`
}

type Users []User

type User struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Token struct {
	Token string `json:"token"`
}

type Claims struct {
	User string
	jwt.RegisteredClaims
}
