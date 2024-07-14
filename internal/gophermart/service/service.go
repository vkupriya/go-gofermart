package service

import (
	"fmt"

	"github.com/vkupriya/go-gophermart/internal/gophermart/helpers"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	"github.com/vkupriya/go-gophermart/internal/gophermart/storage"
)

type Storage interface {
	// OrdersAdd(userid string, orderid string) error
	// OrdersGet(userid string) (models.Orders, error)
	UserAdd(c *models.Config, userid string, passwd string) error
	UserGet(c *models.Config, userid string) (models.User, error)
	OrderAdd(c *models.Config, userid string, oid string) error
	OrdersGet(c *models.Config, userid string) (models.Orders, error)
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
	ms, err := storage.NewPostgresDB(c)
	if err != nil {
		return ms, fmt.Errorf("failed to initialize MemStorage: %w", err)
	}
	return ms, nil
}

func (g *GophermartService) SvcUserAdd(userid string, passwd string) error {
	logger := g.config.Logger
	err := g.store.UserAdd(g.config, userid, passwd)
	if err != nil {
		return fmt.Errorf("failed to register user %s: %w", userid, err)
	}
	logger.Sugar().Debugf("user %s has been registered", userid)
	return nil
}

func (g *GophermartService) SvcUserLogin(userid string, passwd string) (string, error) {
	// logger := g.config.Logger
	user, err := g.store.UserGet(g.config, userid)
	if err != nil {
		return "", fmt.Errorf("failed to query user: %w", err)
	}

	if user.Password != passwd {
		return "", fmt.Errorf("incorrect password")
	}

	tokenStr, err := helpers.CreateJWTString(g.config, userid)
	if err != nil {
		return "", fmt.Errorf("failed to create JWT token for user %s", userid)
	}
	return tokenStr, nil
}

func (g *GophermartService) SvcOrderAdd(userid string, oid string) error {
	logger := g.config.Logger
	err := g.store.OrderAdd(g.config, userid, oid)
	if err != nil {
		return fmt.Errorf("failed to register order %s: %w", oid, err)
	}
	logger.Sugar().Debugf("order %s has been registered", oid)
	return nil
}

func (g *GophermartService) SvcOrdersGet(userid string) (models.Orders, error) {

	orders, err := g.store.OrdersGet(g.config, userid)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders for user %s: %w", userid, err)
	}
	return orders, nil
}
