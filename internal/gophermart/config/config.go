package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	models "github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

const (
	defaultContextTimeout        time.Duration = 3 * time.Second
	defaultJWTTokenTTL           time.Duration = 3600 * time.Second
	defaultAddress               string        = "localhost:8080"
	defaultAccrualURL            string        = "http://localhost:8082"
	defaultAccrualHTTPTimeout    time.Duration = 10 * time.Second
	defaultAccrualRetryAfter     time.Duration = 60 * time.Second
	defaultAccrualInterval       time.Duration = 10 * time.Second
	defaultAccrualWorkers        int64         = 3
	defaultTimeoutServerShutdown time.Duration = 5 * time.Second
	defaultTimeoutShutdown       time.Duration = 10 * time.Second
	defaultAccrualWorkerRetry    time.Duration = 15 * time.Second
)

func NewConfig() (*models.Config, error) {
	logConfig := zap.NewDevelopmentConfig()
	logger, err := logConfig.Build()
	if err != nil {
		return &models.Config{}, fmt.Errorf("failed to initialize Logger: %w", err)
	}

	// Opening config file if present
	viper.AddConfigPath("./")
	viper.AddConfigPath("$HOME/")
	viper.SetConfigName(".env")
	viper.SetConfigType("yaml")
	err = viper.ReadInConfig()
	if err != nil {
		fmt.Println("Error:", err)
		// if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		// 	logger.Sugar().Infof("configuration .env file not found")
		// } else {
		// 	logger.Sugar().Info("Failed to open config file", zap.Error(err))
		// }
	}

	vJWTKey := viper.GetString("server.JWTKey")
	vJWTTokenTTL := viper.GetInt64("server.JWTTokenTTL")
	vAddress := viper.GetString("server.Address")
	vTimeoutServerShutdown := viper.GetInt64("server.TimeoutServerShutdown")
	vTimeoutShutdown := viper.GetInt64("server.TimeoutShutdown")
	vAccrualAddress := viper.GetString("accrual.Address")
	vHTTPTimeout := viper.GetInt64("accrual.HTTPTimeout")
	vInterval := viper.GetInt64("accrual.Interval")
	vWorkers := viper.GetInt64("accrual.Workers")
	vWorkerRetry := viper.GetInt64("accrual.WorkerRetry")

	a := flag.String("a", defaultAddress, "Gophermart server host address and port.")
	r := flag.String("r", defaultAccrualURL, "Accrual server address and port")
	w := flag.Int64("w", defaultAccrualWorkers, "Number of Accrual processing workers")
	d := flag.String("d", "", "PostgreSQL DSN")
	j := flag.String("j", "", "JWT key")

	flag.Parse()

	if *a == defaultAddress {
		if envAddr, ok := os.LookupEnv("RUN_ADDRESS"); ok {
			a = &envAddr
		} else if vAddress != "" {
			a = &vAddress
		}
	}

	if *r == defaultAccrualURL {
		if envAccrualAddr, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); ok {
			r = &envAccrualAddr
		} else if vAccrualAddress != "" {
			r = &vAccrualAddress
		}
	}

	if *w == defaultAccrualWorkers {
		if envAccrualWorkers, ok := os.LookupEnv("ACCRUAL_WORKERS"); ok {
			envAccrualWorkers, err := strconv.ParseInt(envAccrualWorkers, 10, 64)
			if err != nil {
				return nil, errors.New("failed to convert env var ACCRUAL_WORKERS to integer")
			}
			w = &envAccrualWorkers
		} else if vWorkers != 0 {
			*w = vWorkers
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
			if vJWTKey != "" {
				j = &vJWTKey
			} else {
				return &models.Config{}, errors.New("jwt secret key is missing")
			}
		}
	}

	var JWTTokenTTL time.Duration
	if vJWTTokenTTL != 0 {
		JWTTokenTTL = time.Duration(vJWTTokenTTL) * time.Second
	} else {
		JWTTokenTTL = defaultJWTTokenTTL
	}

	var TimeoutServerShutdown time.Duration
	if vTimeoutServerShutdown != 0 {
		TimeoutServerShutdown = time.Duration(vTimeoutServerShutdown) * time.Second
	} else {
		TimeoutServerShutdown = defaultTimeoutServerShutdown
	}

	var TimeoutShutdown time.Duration
	if vTimeoutShutdown != 0 {
		TimeoutShutdown = time.Duration(vTimeoutShutdown) * time.Second
	} else {
		TimeoutShutdown = defaultTimeoutShutdown
	}

	var AccrualHTTPTimeout time.Duration
	if vHTTPTimeout != 0 {
		AccrualHTTPTimeout = time.Duration(vHTTPTimeout) * time.Second
	} else {
		AccrualHTTPTimeout = defaultAccrualHTTPTimeout
	}

	var AccrualInterval time.Duration
	if vInterval != 0 {
		AccrualInterval = time.Duration(vInterval) * time.Second
	} else {
		AccrualInterval = defaultAccrualInterval
	}

	var AccrualWorkerRetry time.Duration
	if vWorkerRetry != 0 {
		AccrualWorkerRetry = time.Duration(vWorkerRetry) * time.Second
	} else {
		AccrualWorkerRetry = defaultAccrualWorkerRetry
	}
	return &models.Config{
		Address:               *a,
		Logger:                logger,
		PostgresDSN:           *d,
		ContextTimeout:        defaultContextTimeout,
		JWTKey:                *j,
		JWTTokenTTL:           JWTTokenTTL,
		AccrualAddress:        *r,
		AccrualHTTPTimeout:    AccrualHTTPTimeout,
		AccrualRetryAfter:     defaultAccrualRetryAfter,
		AccrualInterval:       AccrualInterval,
		AccrualWorkers:        *w,
		TimeoutServerShutdown: TimeoutServerShutdown,
		TimeoutShutdown:       TimeoutShutdown,
		AccrualWorkerRetry:    AccrualWorkerRetry,
	}, nil
}
