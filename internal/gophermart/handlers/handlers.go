package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	mw "github.com/vkupriya/go-gophermart/internal/gophermart/middleware"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	"go.uber.org/zap"
)

type Service interface {
	SvcOrdersAdd(uid string, oid string) error
	SvcOrdersGet(uid string) (models.Orders, error)
	SvcUserAdd(uid string, passwd string) error
	SvcUserLogin(uid string, passwd string) (string, error)
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
		r.Get("/api/orders/{userID}", gr.GetOrders)
	})
	return r
}

func (gr *GophermartHandler) GetOrders(rw http.ResponseWriter, r *http.Request) {
	logger := gr.config.Logger
	v := r.Context().Value(mw.CtxKey{})
	ctxUname, ok := v.(string)
	if !ok {
		logger.Sugar().Error("failed to get user from context value")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Println("ctxUser: ", ctxUname)

	user := chi.URLParam(r, "userID")

	resp, err := gr.service.SvcOrdersGet(user)
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

	if err := gr.service.SvcUserAdd(user.Login, user.Password); err != nil {
		logger.Sugar().Error("login already taken")
		rw.WriteHeader(http.StatusConflict)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	if err := enc.Encode(&user); err != nil {
		logger.Sugar().Debug("error encoding JSON response", zap.Error(err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
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

	token, err := gr.service.SvcUserLogin(user.Login, user.Password)
	if err != nil || token == "" {
		fmt.Println(err)
		logger.Sugar().Errorf("user %s failed to authenticate", user.Login)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	rw.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(rw, token)
}
