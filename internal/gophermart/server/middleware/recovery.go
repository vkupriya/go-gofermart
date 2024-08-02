package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type MiddlewareRecovery struct {
	logger *zap.Logger
}

func NewMiddlewareRecovery(zl *zap.Logger) *MiddlewareRecovery {
	return &MiddlewareRecovery{
		logger: zl,
	}
}

func (m *MiddlewareRecovery) Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := m.logger
		defer func() {
			errRec := recover()
			if errRec != nil {
				switch x := errRec.(type) {
				case string:
					err := errors.New(x)
					logger.Sugar().Error("a panic occured ", zap.Error(err))
				case error:
					err := fmt.Errorf("a panic occurred: %w", x)
					logger.Sugar().Error(zap.Error(err))
				default:
					err := errors.New("unknown panic")
					logger.Sugar().Error(zap.Error(err))
				}
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
