package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	models "github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

const (
	defaultContextTimeout        time.Duration = 3 * time.Second
	defaultJWTTokenTTL           int64         = 3600
	defaultAddress               string        = "localhost:8080"
	defaultAccrualURL            string        = "http://localhost:8082"
	defaultAccrualHTTPTimeout    time.Duration = 10 * time.Second
	defaultAccrualRateLimitWait  time.Duration = 60 * time.Second
	defaultAccrualInterval       time.Duration = 10 * time.Second
	defaultAccrualWorkers        int64         = 3
	defaultTimeoutServerShutdown time.Duration = 5 * time.Second
	defaultTimeoutShutdown       time.Duration = 10 * time.Second
	defaultAccrualWorkerRetry    time.Duration = 15 * time.Second
)

func NewConfig() (*models.Config, error) {
	a := flag.String("a", defaultAddress, "Gophermart server host address and port.")
	r := flag.String("r", defaultAccrualURL, "Accrual server address and port")
	w := flag.Int64("w", defaultAccrualWorkers, "Number of Accrual processing workers")
	d := flag.String("d", "", "PostgreSQL DSN")
	j := flag.String("j", "", "JWT key")

	flag.Parse()

	if *a == defaultAddress {
		if envAddr, ok := os.LookupEnv("RUN_ADDRESS"); ok {
			a = &envAddr
		}
	}

	if *r == defaultAccrualURL {
		if envAccrualAddr, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); ok {
			r = &envAccrualAddr
		}
	}

	if *w == defaultAccrualWorkers {
		if envAccrualWorkers, ok := os.LookupEnv("ACCRUAL_WORKERS"); ok {
			envAccrualWorkers, err := strconv.ParseInt(envAccrualWorkers, 10, 64)
			if err != nil {
				return nil, errors.New("failed to convert env var ACCRUAL_WORKERS to integer")
			}
			w = &envAccrualWorkers
		}
	}

	if !strings.Contains(*r, "://") {
		*r = "http://" + *r
	}

	if *d == "" {
		if envDSN, ok := os.LookupEnv("DATABASE_URI"); ok {
			d = &envDSN
		} else {
			return &models.Config{}, errors.New("postgreSQL DSN is missing")
		}
	}

	if *j == "" {
		if envJWT, ok := os.LookupEnv("JWT"); ok {
			j = &envJWT
		} else {
			return &models.Config{}, errors.New("jwt secret key is missing")
		}
	}

	logConfig := zap.NewDevelopmentConfig()
	logger, err := logConfig.Build()
	if err != nil {
		return &models.Config{}, fmt.Errorf("failed to initialize Logger: %w", err)
	}

	return &models.Config{
		Address:               *a,
		Logger:                logger,
		PostgresDSN:           *d,
		ContextTimeout:        defaultContextTimeout,
		KeyJWT:                *j,
		JWTTokenTTL:           defaultJWTTokenTTL,
		AccrualAddress:        *r,
		AccrualHTTPTimeout:    defaultAccrualHTTPTimeout,
		AccrualRateLimitWait:  defaultAccrualRateLimitWait,
		AccrualInterval:       defaultAccrualInterval,
		AccrualWorkers:        *w,
		TimeoutServerShutdown: defaultTimeoutServerShutdown,
		TimeoutShutdown:       defaultTimeoutShutdown,
		AccrualWorkerRetry:    defaultAccrualWorkerRetry,
	}, nil
}
