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
	AccrualAddress string
	JWTTokenTTL    int64
	ContextTimeout int64
}

type Orders []Order

type Order struct {
	UserID   string    `json:"-" db:"userid"`
	Uploaded time.Time `json:"uploaded_at" db:"uploaded_at"`
	Number   string    `json:"number" db:"number"`
	Status   string    `json:"status" db:"status"`
	Accrual  float32   `json:"accrual,omitempty" db:"accrual"`
}

type Users []User

type User struct {
	UserID   string  `json:"login"`
	Password string  `json:"password"`
	Accrual  float32 `json:"-"`
}

type Claims struct {
	UserID string
	jwt.RegisteredClaims
}

type AccrualResponse struct {
	Status  string  `json:"status"`
	Number  string  `json:"order"`
	Accrual float32 `json:"accrual"`
}

type Withdrawals []Withdrawal

type Withdrawal struct {
	Processed time.Time `json:"processed_at" db:"processed_at"`
	UserID    string    `json:"-" db:"userid"`
	Number    string    `json:"order" db:"number"`
	Sum       float32   `json:"sum" db:"sum"`
}

type Balance struct {
	Current   float32 `json:"current"`
	Withdrawn float32 `json:"withdrawn"`
}
