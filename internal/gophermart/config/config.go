package config

import (
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap"

	models "github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

const (
	defaultContextTimeout int64 = 3
	defaultJWTTokenTTL    int64 = 3600
)

func NewConfig() (*models.Config, error) {
	a := flag.String("a", "localhost:8080", "Gophermart server host address and port.")
	d := flag.String("d", "postgres://postgres:postgres@localhost:5432/gophermart?sslmode=disable", "PostgreSQL DSN")
	j := flag.String("j", "secret-key", "JWT key")
	r := flag.String("r", "localhost:8082", "Accrual server address and port")

	flag.Parse()

	if envAddr, ok := os.LookupEnv("RUN_ADDRESS"); ok {
		a = &envAddr
	}

	if envDSN, ok := os.LookupEnv("DATABASE_URI"); ok {
		d = &envDSN
	}

	if envJWT, ok := os.LookupEnv("JWT"); ok {
		j = &envJWT
	}

	if envAccrualAddr, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); ok {
		r = &envAccrualAddr
	}

	logConfig := zap.NewDevelopmentConfig()
	logger, err := logConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Logger: %w", err)
	}

	return &models.Config{
		Address:        *a,
		Logger:         logger,
		PostgresDSN:    *d,
		ContextTimeout: defaultContextTimeout,
		KeyJWT:         *j,
		JWTTokenTTL:    defaultJWTTokenTTL,
		AccrualAddress: *r,
	}, nil
}
