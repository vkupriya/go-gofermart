package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/vkupriya/go-gophermart/internal/gophermart/helpers"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	mw "github.com/vkupriya/go-gophermart/internal/gophermart/server/middleware"
	"go.uber.org/zap"
)

type Service interface {
	SvcUserAdd(user models.User) error
	SvcUserGet(uid string) (models.User, error)
	SvcUserLogin(uid string, passwd string) (string, error)
	SvcOrderAdd(uid string, oid string) error
	SvcOrdersGet(uid string) (models.Orders, error)
	SvcOrderGet(oid string) (models.Order, error)
	SvcAccrualWithdraw(w models.Withdrawal) error
	SvcWithdrawalsGet(uid string) (models.Withdrawals, error)
	SvcBalanceGet(uid string) (models.Balance, error)
}

type GophermartHandler struct {
	service Service
	config  *models.Config
}

const (
	ErrorNoContextUser        string = "failed to get user from context value"
	ErrorIncorrectOrderNumber string = "incorrect order number "
)

func NewGophermartHandler(service Service, cfg *models.Config) *GophermartHandler {
	return &GophermartHandler{
		service: service,
		config:  cfg,
	}
}

func NewGophermartRouter(gr *GophermartHandler) chi.Router {
	r := chi.NewRouter()

	ma := mw.NewMiddlewareAuth(gr.config)
	ml := mw.NewMiddlewareLogger(gr.config)
	r.Use(ml.Logging)
	r.Post("/api/user/register", gr.UserAdd)
	r.Post("/api/user/login", gr.UserLogin)

	r.Group(func(r chi.Router) {
		r.Use(ma.Auth)
		r.Post("/api/user/orders", gr.OrderAdd)
		r.Get("/api/user/orders", gr.OrdersGet)
		r.Post("/api/user/balance/withdraw", gr.AccrualWithdraw)
		r.Get("/api/user/withdrawals", gr.WithdrawalsGet)
		r.Get("/api/user/balance", gr.BalanceGet)
	})
	return r
}

func (gr *GophermartHandler) OrdersGet(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger
	v := r.Context().Value(mw.CtxKey{})
	ctxUname, ok := v.(string)
	if !ok {
		logger.Sugar().Error(ErrorNoContextUser)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err := gr.service.SvcOrdersGet(ctxUname)
	if err != nil {
		fmt.Println(err)
	}

	body, err := json.Marshal(resp)
	if err != nil {
		logger.Sugar().Error("failed to marshal orders list", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	if _, err := rw.Write(body); err != nil {
		logger.Sugar().Error("failed to write orders list", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (gr *GophermartHandler) UserAdd(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger

	var user models.User

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&user); err != nil {
		logger.Sugar().Error("cannot decode request JSON body")
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := gr.service.SvcUserAdd(user); err != nil {
		logger.Sugar().Error(zap.Error(err))
		rw.WriteHeader(http.StatusConflict)
		return
	}

	token, err := gr.service.SvcUserLogin(user.UserID, user.Password)
	if err != nil || token == "" {
		fmt.Println(err)
		logger.Sugar().Errorf("user %s failed to authenticate", user.UserID)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	rw.Header().Set("Authorization", "Bearer "+token)
}

func (gr *GophermartHandler) UserLogin(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger

	var user models.User

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&user); err != nil {
		logger.Sugar().Error("cannot decode request JSON body")
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	token, err := gr.service.SvcUserLogin(user.UserID, user.Password)
	if err != nil || token == "" {
		fmt.Println(err)
		logger.Sugar().Errorf("user %s failed to authenticate", user.UserID)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	rw.Header().Set("Authorization", "Bearer "+token)
}

func (gr *GophermartHandler) OrderAdd(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger
	v := r.Context().Value(mw.CtxKey{})
	ctxUname, ok := v.(string)
	if !ok {
		logger.Sugar().Error(ErrorNoContextUser)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Sugar().Error("failed to read request body.", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
	}
	oid := string(b)
	orderNum, err := strconv.ParseInt(oid, 10, 64)
	if err != nil || !helpers.ValidOrder(orderNum) {
		logger.Sugar().Errorf(ErrorIncorrectOrderNumber, oid)
		rw.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	order, err := gr.service.SvcOrderGet(oid)
	if err != nil {
		logger.Sugar().Error("failed to get order from DB", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	if order.UserID != "" {
		if order.UserID == ctxUname {
			logger.Sugar().Infof("order %s already registered", oid)
			return
		} else {
			logger.Sugar().Errorf("order %s already registered by another user", oid)
			rw.WriteHeader(http.StatusConflict)
			return
		}
	}
	if err := gr.service.SvcOrderAdd(ctxUname, oid); err != nil {
		logger.Sugar().Error(zap.Error(err))
		rw.WriteHeader(http.StatusConflict)
		return
	}
	logger.Sugar().Infof("order %s has been registered", oid)
	rw.WriteHeader(http.StatusAccepted)
}

func (gr *GophermartHandler) AccrualWithdraw(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger
	var w models.Withdrawal
	v := r.Context().Value(mw.CtxKey{})
	ctxUname, ok := v.(string)
	if !ok {
		logger.Sugar().Error(ErrorNoContextUser)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&w); err != nil {
		logger.Sugar().Error("cannot decode request JSON body")
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	w.UserID = ctxUname
	orderNum, err := strconv.ParseInt(w.Number, 10, 64)
	if err != nil || !helpers.ValidOrder(orderNum) {
		logger.Sugar().Errorf(ErrorIncorrectOrderNumber, w.Number)
		rw.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	user, err := gr.service.SvcUserGet(ctxUname)
	if err != nil {
		logger.Sugar().Error("failed to get user from DB", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	if user.Accrual == 0 || w.Sum > user.Accrual {
		logger.Sugar().Error("not enough accrual points to withdraw")
		rw.WriteHeader(http.StatusPaymentRequired)
		return
	}
	if err := gr.service.SvcAccrualWithdraw(w); err != nil {
		logger.Sugar().Error(zap.Error(err))
		rw.WriteHeader(http.StatusConflict)
		return
	}
}

func (gr *GophermartHandler) WithdrawalsGet(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger
	v := r.Context().Value(mw.CtxKey{})
	ctxUname, ok := v.(string)
	if !ok {
		logger.Sugar().Error(ErrorNoContextUser)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	w, err := gr.service.SvcWithdrawalsGet(ctxUname)
	if err != nil {
		logger.Sugar().Error("failed to get withdrawals", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	b, err := json.Marshal(w)
	if err != nil {
		logger.Sugar().Error("failed to marshal withdrawals list", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := rw.Write(b); err != nil {
		logger.Sugar().Error("failed to write withdrawals list", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (gr *GophermartHandler) BalanceGet(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger
	v := r.Context().Value(mw.CtxKey{})
	ctxUname, ok := v.(string)
	if !ok {
		logger.Sugar().Error(ErrorNoContextUser)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	bal, err := gr.service.SvcBalanceGet(ctxUname)
	if err != nil {
		logger.Sugar().Error("failed to get user balance", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, err := json.Marshal(bal)
	if err != nil {
		logger.Sugar().Error("failed to marshal balance", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	if _, err := rw.Write(body); err != nil {
		logger.Sugar().Error("failed to write balance", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}
