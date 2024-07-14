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
)

func NewConfig() (*models.Config, error) {
	a := flag.String("a", "localhost:8080", "Gophermart server host address and port.")
	d := flag.String("d", "postgres://postgres:postgres@localhost:5432/gophermart?sslmode=disable", "PostgreSQL DSN")
	j := flag.String("j", "secret-key", "JWT key")

	flag.Parse()

	if envAddr, ok := os.LookupEnv("ADDRESS"); ok {
		a = &envAddr
	}

	if envDSN, ok := os.LookupEnv("DATABASE_DSN"); ok {
		d = &envDSN
	}

	if envJWT, ok := os.LookupEnv("JWT"); ok {
		j = &envJWT
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
	}, nil
}
