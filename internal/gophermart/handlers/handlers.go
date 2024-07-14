package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	mw "github.com/vkupriya/go-gophermart/internal/gophermart/middleware"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	"go.uber.org/zap"
)

type Service interface {
	// SvcOrdersAdd(uid string, oid string) error
	// SvcOrdersGet(uid string) (models.Orders, error)
	SvcUserAdd(uid string, passwd string) error
	SvcUserLogin(uid string, passwd string) (string, error)
	SvcOrderAdd(uid string, oid string) error
	SvcOrdersGet(uid string) (models.Orders, error)
}

type GophermartHandler struct {
	service Service
	config  *models.Config
}

func NewGophermartHandler(service Service, cfg *models.Config) *GophermartHandler {
	return &GophermartHandler{
		service: service,
		config:  cfg,
	}
}

func NewGophermartRouter(gr *GophermartHandler) chi.Router {
	r := chi.NewRouter()

	ma := mw.NewMiddlewareAuth(gr.config)
	r.Post("/api/user/register", gr.UserAdd)
	r.Post("/api/user/login", gr.UserLogin)

	r.Group(func(r chi.Router) {
		r.Use(ma.Auth)
		r.Post("/api/user/orders", gr.OrderAdd)
		r.Get("/api/user/orders", gr.OrdersGet)
	})
	return r
}

func (gr *GophermartHandler) OrdersGet(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger
	v := r.Context().Value(mw.CtxKey{})
	ctxUname, ok := v.(string)
	if !ok {
		logger.Sugar().Error("failed to get user from context value")
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

	if err := gr.service.SvcUserAdd(user.UserID, user.Password); err != nil {
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
	rw.Header().Set("Authorization", "Bearer: "+token)
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
	rw.Header().Set("Authorization", "Bearer: "+token)
}

func (gr *GophermartHandler) OrderAdd(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger
	v := r.Context().Value(mw.CtxKey{})
	ctxUname, ok := v.(string)
	if !ok {
		logger.Sugar().Error("failed to get user from context value")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	oid, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Sugar().Error("failed to read request body.", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
	}

	if err := gr.service.SvcOrderAdd(ctxUname, string(oid)); err != nil {
		logger.Sugar().Error(zap.Error(err))
		rw.WriteHeader(http.StatusConflict)
		return
	}
}
