package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

type (
	responseData struct {
		status int
		size   int
	}

	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}

	MiddlewareLogger struct {
		config *models.Config
	}
)

func NewMiddlewareLogger(c *models.Config) *MiddlewareLogger {
	return &MiddlewareLogger{
		config: c,
	}
}

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	if err != nil {
		return 0, fmt.Errorf("failed to write into http.ResponseWriter: %w", err)
	}
	r.responseData.size += size
	return size, nil
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

func (m *MiddlewareLogger) Logging(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		logger := m.config.Logger

		start := time.Now()

		responseData := &responseData{
			status: http.StatusOK,
			size:   0,
		}

		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}

		uri := r.RequestURI
		method := r.Method

		h.ServeHTTP(&lw, r)

		duration := time.Since(start)
		logger.Sugar().Infoln(
			"uri", uri,
			"method", method,
			"status", responseData.status,
			"duration", duration,
			"size", responseData.size,
		)
	}
	return http.HandlerFunc(logFn)
}
