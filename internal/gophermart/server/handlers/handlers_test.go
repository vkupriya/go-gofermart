package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	mock_handlers "github.com/vkupriya/go-gophermart/internal/gophermart/server/handlers/mocks"
	mw "github.com/vkupriya/go-gophermart/internal/gophermart/server/middleware"
	"go.uber.org/zap"
)

//nolint:dupl // handlers unit tests following same pattern
func TestOrdersGet(t *testing.T) {
	logConfig := zap.NewDevelopmentConfig()
	logger, err := logConfig.Build()
	if err != nil {
		t.Error("failed to initialize Logger: %w", err)
	}

	tTime, _ := time.Parse(time.RFC3339, "2024-07-21T16:00:11.336546+01:00")

	orders := []models.Order{
		{
			UserID:   "user01",
			Number:   "2377225624",
			Accrual:  0,
			Status:   "NEW",
			Uploaded: tTime,
		},
	}

	testCases := []struct {
		mockSvc      func(*gomock.Controller) *mock_handlers.MockService
		name         string
		user         string
		method       string
		path         string
		expectedCode int
		expectedBody string
	}{
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				s.EXPECT().OrdersGet(gomock.Any()).Return(orders, nil).AnyTimes()
				return s
			},
			name:         "#get_orders_OK",
			user:         "user01",
			method:       http.MethodPost,
			path:         "/api/user/orders",
			expectedCode: http.StatusOK,
			expectedBody: `[{"uploaded_at":"2024-07-21T16:00:11.336546+01:00","number":"2377225624","status":"NEW"}]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := tc.mockSvc(ctrl)

			h := NewGophermartHandler(svc, logger)

			r := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			ctx := r.Context()
			ctx = context.WithValue(ctx, mw.CtxKey{}, tc.user)
			r = r.WithContext(ctx)

			h.OrdersGet(w, r)
			res := w.Result()
			defer func() {
				if err := res.Body.Close(); err != nil {
					assert.Error(t, err)
				}
			}()

			b, _ := io.ReadAll(res.Body)
			assert.NoError(t, err, "error making HTTP request")

			assert.Equal(t, tc.expectedCode, res.StatusCode, "Response code didn't match expected")
			if tc.expectedBody != "" {
				assert.Equal(t, tc.expectedBody, string(b))
			}
		})
	}
}

//nolint:dupl // handlers unit tests following same pattern
func TestOrderAdd(t *testing.T) {
	logConfig := zap.NewDevelopmentConfig()
	logger, err := logConfig.Build()
	if err != nil {
		t.Error("failed to initialize Logger: %w", err)
	}

	tTime, _ := time.Parse(time.RFC3339, "2024-07-21T16:00:11.336546+01:00")

	order := models.Order{
		UserID:   "user01",
		Number:   "2377225624",
		Status:   "NEW",
		Accrual:  0,
		Uploaded: tTime,
	}

	testCases := []struct {
		mockSvc      func(*gomock.Controller) *mock_handlers.MockService
		name         string
		user         string
		method       string
		path         string
		body         string
		expectedCode int
		expectedBody string
	}{
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				s.EXPECT().OrderAdd(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				s.EXPECT().OrderGet(gomock.Any()).Return(models.Order{}, nil).AnyTimes()
				return s
			},
			name:         "#add_order_OK",
			user:         "user01",
			method:       http.MethodPost,
			path:         "/api/user/orders",
			body:         "2377225624",
			expectedCode: http.StatusAccepted,
			expectedBody: "",
		},
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				s.EXPECT().OrderAdd(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				s.EXPECT().OrderGet(gomock.Any()).Return(order, nil).AnyTimes()
				return s
			},
			name:         "#add_order_same_user_OK",
			user:         "user01",
			method:       http.MethodPost,
			path:         "/api/user/orders",
			body:         "2377225624",
			expectedCode: http.StatusOK,
			expectedBody: "",
		},
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				s.EXPECT().OrderAdd(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				s.EXPECT().OrderGet(gomock.Any()).Return(order, nil).AnyTimes()
				return s
			},
			name:         "#add_order_exists_differentuser_FAIL",
			user:         "testuser",
			method:       http.MethodPost,
			path:         "/api/user/orders",
			body:         "2377225624",
			expectedCode: http.StatusConflict,
			expectedBody: "",
		},
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				s.EXPECT().OrderAdd(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				s.EXPECT().OrderGet(gomock.Any()).Return(order, nil).AnyTimes()
				return s
			},
			name:         "#add_order_incorrect_number_FAIL",
			user:         "user01",
			method:       http.MethodPost,
			path:         "/api/user/orders",
			body:         "2377225625",
			expectedCode: http.StatusUnprocessableEntity,
			expectedBody: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := tc.mockSvc(ctrl)

			h := NewGophermartHandler(svc, logger)

			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			ctx := r.Context()
			ctx = context.WithValue(ctx, mw.CtxKey{}, tc.user)
			r = r.WithContext(ctx)

			h.OrderAdd(w, r)
			res := w.Result()
			defer func() {
				if err := res.Body.Close(); err != nil {
					assert.Error(t, err)
				}
			}()

			b, _ := io.ReadAll(res.Body)
			assert.NoError(t, err, "error making HTTP request")

			assert.Equal(t, tc.expectedCode, res.StatusCode, "Response code didn't match expected")
			if tc.expectedBody != "" {
				assert.Equal(t, tc.expectedBody, string(b))
			}
		})
	}
}

//nolint:dupl // handlers unit tests following same pattern
func TestBalanceGet(t *testing.T) {
	logConfig := zap.NewDevelopmentConfig()
	logger, err := logConfig.Build()
	if err != nil {
		t.Error("failed to initialize Logger: %w", err)
	}

	balance := models.Balance{
		Current:   600.5,
		Withdrawn: 386.5,
	}

	testCases := []struct {
		mockSvc      func(*gomock.Controller) *mock_handlers.MockService
		name         string
		user         string
		method       string
		path         string
		expectedCode int
		expectedBody string
	}{
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				s.EXPECT().BalanceGet(gomock.Any()).Return(balance, nil).AnyTimes()
				return s
			},
			name:         "#balance_get_OK",
			user:         "user01",
			method:       http.MethodPost,
			path:         "/api/user/balance",
			expectedCode: http.StatusOK,
			expectedBody: `{"current":600.5,"withdrawn":386.5}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := tc.mockSvc(ctrl)

			h := NewGophermartHandler(svc, logger)

			r := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			ctx := r.Context()
			ctx = context.WithValue(ctx, mw.CtxKey{}, tc.user)
			r = r.WithContext(ctx)

			h.BalanceGet(w, r)
			res := w.Result()
			defer func() {
				if err := res.Body.Close(); err != nil {
					assert.Error(t, err)
				}
			}()

			b, _ := io.ReadAll(res.Body)
			assert.NoError(t, err, "error making HTTP request")

			assert.Equal(t, tc.expectedCode, res.StatusCode, "Response code didn't match expected")
			if tc.expectedBody != "" {
				assert.Equal(t, tc.expectedBody, string(b))
			}
		})
	}
}

//nolint:dupl // handlers unit tests following same pattern
func TestAccrualWithdraw(t *testing.T) {
	logConfig := zap.NewDevelopmentConfig()
	logger, err := logConfig.Build()
	if err != nil {
		t.Error("failed to initialize Logger: %w", err)
	}

	testCases := []struct {
		mockSvc      func(*gomock.Controller) *mock_handlers.MockService
		name         string
		user         string
		method       string
		path         string
		body         string
		expectedCode int
		expectedBody string
	}{
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				return s
			},
			name:         "#accrual_withdraw_badrequest_FAIL",
			user:         "user01",
			method:       http.MethodPost,
			path:         "/api/user/balance/withdraw",
			body:         "",
			expectedCode: http.StatusBadRequest,
			expectedBody: "",
		},
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				s.EXPECT().UserGet(gomock.Any()).Return(models.User{UserID: "user01", Accrual: 0, Password: ""}, nil)
				return s
			},
			name:         "#accrual_withdraw_paymentneeded_FAIL",
			user:         "user01",
			method:       http.MethodPost,
			path:         "/api/user/balance/withdraw",
			body:         `{"order":"12345678903","sum":250}`,
			expectedCode: http.StatusPaymentRequired,
			expectedBody: "",
		},
		{
			mockSvc: func(c *gomock.Controller) *mock_handlers.MockService {
				s := mock_handlers.NewMockService(c)
				s.EXPECT().UserGet(gomock.Any()).Return(models.User{UserID: "user01", Accrual: 500, Password: ""}, nil)
				s.EXPECT().AccrualWithdraw(gomock.Any()).Return(nil)
				return s
			},
			name:         "#accrual_withdraw_OK",
			user:         "user01",
			method:       http.MethodPost,
			path:         "/api/user/balance/withdraw",
			body:         `{"order":"12345678903","sum":250}`,
			expectedCode: http.StatusOK,
			expectedBody: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := tc.mockSvc(ctrl)

			h := NewGophermartHandler(svc, logger)

			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			ctx := r.Context()
			ctx = context.WithValue(ctx, mw.CtxKey{}, tc.user)
			r = r.WithContext(ctx)

			h.AccrualWithdraw(w, r)
			res := w.Result()
			defer func() {
				if err := res.Body.Close(); err != nil {
					assert.Error(t, err)
				}
			}()

			b, _ := io.ReadAll(res.Body)
			assert.NoError(t, err, "error making HTTP request")

			assert.Equal(t, tc.expectedCode, res.StatusCode, "Response code didn't match expected")
			if tc.expectedBody != "" {
				assert.Equal(t, tc.expectedBody, string(b))
			}
		})
	}
}
