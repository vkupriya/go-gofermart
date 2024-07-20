package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/go-resty/resty/v2"

	"github.com/vkupriya/go-gophermart/internal/gophermart/helpers"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	"github.com/vkupriya/go-gophermart/internal/gophermart/storage"
)

type Storage interface {
	UserAdd(c *models.Config, user models.User) error
	UserGet(c *models.Config, userid string) (models.User, error)
	OrderAdd(c *models.Config, userid string, oid string) error
	OrderGet(c *models.Config, oid string) (models.Order, error)
	OrdersGet(c *models.Config, userid string) (models.Orders, error)
	GetAllNewOrders(c *models.Config) (models.Orders, error)
	UpdateOrder(c *models.Config, order models.Order) error
	UserAddAccrual(c *models.Config, order models.Order) error
	AccrualWithdraw(c *models.Config, w models.Withdrawal) error
	WithdrawalsGet(c *models.Config, userid string) (models.Withdrawals, error)
	BalanceGet(c *models.Config, userid string) (models.Balance, error)
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

func (g *GophermartService) SvcUserAdd(user models.User) error {
	logger := g.config.Logger
	err := g.store.UserAdd(g.config, user)
	if err != nil {
		return fmt.Errorf("failed to register user %s: %w", user.UserID, err)
	}
	logger.Sugar().Debugf("user %s has been registered", user.UserID)
	return nil
}

func (g *GophermartService) SvcUserGet(userid string) (models.User, error) {
	user, err := g.store.UserGet(g.config, userid)
	if err != nil {
		return user, fmt.Errorf("failed to get user %s: %w", userid, err)
	}
	return user, nil
}

func (g *GophermartService) SvcUserLogin(userid string, passwd string) (string, error) {
	// logger := g.config.Logger
	user, err := g.store.UserGet(g.config, userid)
	if err != nil {
		return "", fmt.Errorf("failed to query user: %w", err)
	}

	if user.Password != passwd {
		return "", fmt.Errorf("incorrect password for user %s", userid)
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

func (g *GophermartService) SvcOrderGet(oid string) (models.Order, error) {
	order, err := g.store.OrderGet(g.config, oid)
	if err != nil {
		return models.Order{}, fmt.Errorf("failed to get order %s: %w", oid, err)
	}
	return order, nil
}

func (g *GophermartService) SvcOrdersGet(userid string) (models.Orders, error) {
	orders, err := g.store.OrdersGet(g.config, userid)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders for user %s: %w", userid, err)
	}
	return orders, nil
}

func (g *GophermartService) SvcAccrualWithdraw(w models.Withdrawal) error {
	err := g.store.AccrualWithdraw(g.config, w)
	if err != nil {
		return fmt.Errorf("failed to withdraw accrual for user %s", w.UserID)
	}
	return nil
}

func (g *GophermartService) SvcWithdrawalsGet(userid string) (models.Withdrawals, error) {
	w, err := g.store.WithdrawalsGet(g.config, userid)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders for user %s: %w", userid, err)
	}
	return w, nil
}

func (g *GophermartService) SvcBalanceGet(userid string) (models.Balance, error) {
	bal, err := g.store.BalanceGet(g.config, userid)
	if err != nil {
		return models.Balance{}, fmt.Errorf("failed to get orders for user %s: %w", userid, err)
	}
	return bal, nil
}

func (g *GophermartService) SvcOrderFetcher(ctx context.Context) error {
	logger := g.config.Logger
	const (
		tickerPeriod int64 = 10
	)
	var failure bool
	ordersTicker := time.NewTicker(time.Duration(tickerPeriod) * time.Second)
	defer ordersTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ordersTicker.C:
			orders, err := g.store.GetAllNewOrders(g.config)
			if err != nil {
				return fmt.Errorf("failed to get unprocessed orders: %w", err)
			}
			failure = false
			for _, order := range orders {
				if failure {
					order.Status = "NEW"
					if err := g.SvcOrderUpdate(&order); err != nil {
						logger.Sugar().Error("failed to update order in DB", zap.Error(err))
					}
					continue
				}
				accrualOrder, err := g.SvcOrderGetAccrual(&order)
				if err != nil {
					// revert status of order to NEW
					order.Status = "NEW"
					if err := g.SvcOrderUpdate(&order); err != nil {
						logger.Sugar().Error("failed to update order in DB", zap.Error(err))
					}
					failure = true
					continue
				}
				// Update order in DB
				if err := g.SvcOrderUpdate(&accrualOrder); err != nil {
					logger.Sugar().Error("failed to update order in DB", zap.Error(err))
					failure = true
				}
			}
			// exiting Ticker
			if failure {
				return errors.New("failure to communicate to accrual service and/or DB, exiting")
			}
		}
	}
}

func (g *GophermartService) SvcOrderGetAccrual(order *models.Order) (models.Order, error) {
	const (
		httpTimeout       int   = 30
		retries           int   = 3
		retryDelay        int   = 2
		rateExceededRetry int64 = 60
	)

	var (
		ar    models.AccrualResponse
		retry int
	)

	logger := g.config.Logger
	h := g.config.AccrualAddress

	client := resty.New()
	client.SetTimeout(time.Duration(httpTimeout) * time.Second)

	url := fmt.Sprintf("%s/api/orders/%s", h, order.Number)
	retry = 0
	for retry <= retries {
		if retry == retries {
			return models.Order{}, fmt.Errorf("failed to send metrics after %d", retries)
		}

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			Get(url)

		if err != nil {
			logger.Sugar().Errorf("failed to connect to accrual service, retrying: %v\n", err)
			time.Sleep(time.Duration(1+(retry*retryDelay)) * time.Second)
		} else {
			if resp.StatusCode() == http.StatusTooManyRequests {
				logger.Sugar().Error("request limit exceeded, retrying in 60 seconds")
				time.Sleep(time.Duration(rateExceededRetry) * time.Second)
			}

			if resp.StatusCode() == http.StatusOK {
				if err := json.Unmarshal(resp.Body(), &ar); err != nil {
					return *order, fmt.Errorf("failed to unmarshal accrual response: %w", err)
				}
				order.Status = ar.Status
				order.Accrual = ar.Accrual
				return *order, nil
			} else {
				break
			}
		}
		retry++
	}
	return models.Order{}, nil
}

func (g *GophermartService) SvcOrderUpdate(order *models.Order) error {
	if err := g.store.UpdateOrder(g.config, *order); err != nil {
		return fmt.Errorf("error updating order %s: %w", order.Number, err)
	}
	if order.Accrual != 0 {
		if err := g.store.UserAddAccrual(g.config, *order); err != nil {
			return fmt.Errorf("failed to add accrual for user %s: %w", order.UserID, err)
		}
	}
	return nil
}
