package service

import (
	"fmt"

	"github.com/vkupriya/go-gophermart/internal/gophermart/helpers"
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

func (g *GophermartService) SvcOrdersAdd(user string, oid string) error {
	if err := g.store.OrdersAdd(user, oid); err != nil {
		return fmt.Errorf("failed to add order %s for user %s: %w", oid, user, err)
	}
	return nil
}

func (g *GophermartService) SvcOrdersGet(user string) (models.Orders, error) {
	logger := g.config.Logger
	res, err := g.store.OrdersGet(user)
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

func (g *GophermartService) SvcUserLogin(uname string, passwd string) (string, error) {
	logger := g.config.Logger
	user, err := g.store.UserGet(uname)
	if err != nil {
		return "", fmt.Errorf("failed to query user")
	}

	if user.Password != passwd {
		return "", fmt.Errorf("incorrect password")
	}

	tokenStr, err := helpers.CreateJWTString(g.config, uname)
	if err != nil {
		return "", fmt.Errorf("failed to create JWT token for user %s", uname)
	}

	logger.Sugar().Debugf("user %s logged in successfully", uname)
	return tokenStr, nil
}
