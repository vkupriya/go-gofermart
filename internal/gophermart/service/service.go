package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	"github.com/vkupriya/go-gophermart/internal/gophermart/storage"
)

type Storage interface {
	OrdersAdd(userid string, orderid string) error
	OrdersGet(userid string) (models.Orders, error)
	UserAdd(userid string, passwd string) error
	UserGet(userid string) (models.User, error)
}

type GophermartService struct {
	store  Storage
	config *models.Config
}

func NewGophermartService(store Storage, cfg *models.Config) *GophermartService {
	return &GophermartService{
		store:  store,
		config: cfg}
}

func NewStore(c *models.Config) (Storage, error) {
	ms, err := storage.NewMemStorage(c)
	if err != nil {
		return ms, fmt.Errorf("failed to initialize MemStorage: %w", err)
	}
	return ms, nil
}

func (g *GophermartService) SvcOrdersAdd(uid string, oid string) error {
	if err := g.store.OrdersAdd(uid, oid); err != nil {
		return fmt.Errorf("failed to add order %s for user %s: %w", oid, uid, err)
	}
	return nil
}

func (g *GophermartService) SvcOrdersGet(uid string) (models.Orders, error) {
	logger := g.config.Logger
	res, err := g.store.OrdersGet(uid)
	if err != nil {
		logger.Sugar().Error(err)
	}
	return res, nil
}

func (g *GophermartService) SvcUserAdd(uid string, passwd string) error {
	logger := g.config.Logger
	err := g.store.UserAdd(uid, passwd)
	if err != nil {
		return fmt.Errorf("failed to register user %s: %w", uid, err)
	}
	logger.Sugar().Debugf("user %s has been registered", uid)
	return nil
}

func (g *GophermartService) SvcUserLogin(uid string, passwd string) (string, error) {
	logger := g.config.Logger
	key := g.config.KeyJWT
	user, err := g.store.UserGet(uid)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user")
	}

	if user.Password != passwd {
		return "", fmt.Errorf("incorrect password")
	}

	claims := models.Claims{
		Username: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 1)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenStr, err := token.SignedString([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w", err)
	}
	logger.Sugar().Debugf("user %s has been registered", uid)
	return tokenStr, nil
}
