package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type Config struct {
	Logger         *zap.Logger
	Address        string
	PostgresDSN    string
	KeyJWT         string
	ContextTimeout int64
}

type Orders []Order

type Order struct {
	UserID   string    `json:"-" db:"userid"`
	Uploaded time.Time `json:"uploaded_at" db:"uploaded_at"`
	Number   string    `json:"number" db:"number"`
	Status   string    `json:"status" db:"status"`
	Accrual  int64     `json:"accrual,omitempty" db:"accrual"`
}

type Users []User

type User struct {
	UserID   string `json:"login"`
	Password string `json:"password"`
}

type Claims struct {
	UserID string
	jwt.RegisteredClaims
}
