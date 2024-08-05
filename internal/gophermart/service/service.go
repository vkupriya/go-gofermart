package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"go.uber.org/zap"

	"github.com/go-resty/resty/v2"

	"github.com/vkupriya/go-gophermart/internal/gophermart/helpers"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	"github.com/vkupriya/go-gophermart/internal/gophermart/storage"

	"golang.org/x/crypto/bcrypt"
)

type Storage interface {
	UserAdd(c *models.Config, user models.User) error
	UserGet(c *models.Config, userid string) (models.User, error)
	OrderAdd(c *models.Config, userid string, oid string) error
	OrderGet(c *models.Config, oid string) (models.Order, error)
	OrdersGet(c *models.Config, userid string) (models.Orders, error)
	GetUnprocessedOrders(c *models.Config) (models.Orders, error)
	UpdateOrder(c *models.Config, order *models.Order) error
	UserAddAccrual(c *models.Config, order *models.Order) error
	AccrualWithdraw(c *models.Config, w models.Withdrawal) error
	WithdrawalsGet(c *models.Config, userid string) (models.Withdrawals, error)
	BalanceGet(c *models.Config, userid string) (models.Balance, error)
}

type GophermartService struct {
	store  Storage
	config *models.Config
}

func NewGophermartService(store *storage.PostgresDB, cfg *models.Config) *GophermartService {
	return &GophermartService{
		store:  store,
		config: cfg}
}

func (g *GophermartService) UserAdd(user models.User) error {
	logger := g.config.Logger
	password, err := helpers.HashPassword(user.Password)
	if err != nil {
		return fmt.Errorf("failed to register user %s: %w", user.UserID, err)
	}
	user.Password = password
	if err = g.store.UserAdd(g.config, user); err != nil {
		return fmt.Errorf("failed to register user %s: %w", user.UserID, err)
	}
	logger.Sugar().Debugw("user has been registered",
		"userID", user.UserID)
	return nil
}

func (g *GophermartService) UserGet(userid string) (models.User, error) {
	user, err := g.store.UserGet(g.config, userid)
	if err != nil {
		return user, fmt.Errorf("failed to get user %s: %w", userid, err)
	}
	return user, nil
}

func (g *GophermartService) UserLogin(userid string, passwd string) (string, error) {
	// logger := g.config.Logger
	user, err := g.store.UserGet(g.config, userid)
	if err != nil {
		return "", fmt.Errorf("failed to query user: %w", err)
	}
	ok := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(passwd))
	if ok != nil {
		return "", fmt.Errorf("incorrect password for user %s", userid)
	}

	tokenStr, err := helpers.CreateJWTString(g.config, userid)
	if err != nil {
		return "", fmt.Errorf("failed to create JWT token for user %s", userid)
	}
	return tokenStr, nil
}

func (g *GophermartService) OrderAdd(userid string, oid string) error {
	logger := g.config.Logger

	err := g.store.OrderAdd(g.config, userid, oid)
	if err != nil {
		return fmt.Errorf("failed to register order %s: %w", oid, err)
	}
	logger.Sugar().Debugw("order has been registered",
		"OrderID", oid)
	return nil
}

func (g *GophermartService) OrderGet(oid string) (models.Order, error) {
	order, err := g.store.OrderGet(g.config, oid)
	if err != nil {
		return models.Order{}, fmt.Errorf("failed to get order %s: %w", oid, err)
	}
	return order, nil
}

func (g *GophermartService) OrdersGet(userid string) (models.Orders, error) {
	orders, err := g.store.OrdersGet(g.config, userid)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders for user %s: %w", userid, err)
	}
	return orders, nil
}

func (g *GophermartService) AccrualWithdraw(w models.Withdrawal) error {
	err := g.store.AccrualWithdraw(g.config, w)
	if err != nil {
		return fmt.Errorf("failed to withdraw accrual for user %s", w.UserID)
	}
	return nil
}

func (g *GophermartService) WithdrawalsGet(userid string) (models.Withdrawals, error) {
	w, err := g.store.WithdrawalsGet(g.config, userid)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders for user %s: %w", userid, err)
	}
	return w, nil
}

func (g *GophermartService) BalanceGet(userid string) (models.Balance, error) {
	bal, err := g.store.BalanceGet(g.config, userid)
	if err != nil {
		return models.Balance{}, fmt.Errorf("failed to get orders for user %s: %w", userid, err)
	}
	return bal, nil
}

func (g *GophermartService) OrderDispatcher(ctx context.Context) error {
	var RetryFlag atomic.Bool
	// setting RetryFlag to false
	RetryFlag.Store(false)

	inputCh := make(chan models.Order, g.config.AccrualWorkers)
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := g.orderTicker(egCtx, inputCh); err != nil {
			return fmt.Errorf("order ticker failed: %w", err)
		}
		return nil
	})
	for w := 1; w <= int(g.config.AccrualWorkers); w++ {
		eg.Go(func() error {
			if err := g.getAccrualWorker(egCtx, inputCh, &RetryFlag); err != nil {
				return fmt.Errorf("accrual worker failed: %w", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to run collector/sender go routines: %w", err)
	}
	return nil
}

func (g *GophermartService) orderTicker(ctx context.Context, ch chan<- models.Order) error {
	ordersTicker := time.NewTicker(g.config.AccrualInterval)
	defer ordersTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ordersTicker.C:
			orders, err := g.store.GetUnprocessedOrders(g.config)
			if err != nil {
				return fmt.Errorf("failed to get unprocessed orders: %w", err)
			}
			for _, order := range orders {
				ch <- order
			}
		}
	}
}

func (g *GophermartService) getAccrualWorker(ctx context.Context, ch <-chan models.Order, rf *atomic.Bool) error {
	var (
		ar         models.AccrualResponse
		retryAfter time.Duration
	)

	logger := g.config.Logger
	h := g.config.AccrualAddress

	client := resty.New().
		SetTimeout(g.config.AccrualHTTPTimeout)

	for {
		select {
		case <-ctx.Done():
			return nil
		case order := <-ch:
			url := fmt.Sprintf("%s/api/orders/%s", h, order.Number)
			if rf.Load() {
				for {
					time.Sleep(g.config.AccrualWorkerRetry)
					if !rf.Load() {
						break
					}
				}
			}
			for {
				resp, err := client.R().
					SetHeader("Content-Type", "application/json").
					Get(url)

				if err != nil {
					logger.Sugar().Errorf("failed to connect to accrual service, retrying: %v\n", err)
					break
				}

				if resp.StatusCode() == http.StatusTooManyRequests {
					logger.Sugar().Error("request limit exceeded, retrying in 60 seconds")
					// checking if Retry-After is set in the Header otherwise use configured parameter
					r := resp.Header().Get("Retry-After")
					if r != "" {
						retryAfterInt, err := strconv.ParseInt(r, 10, 64)
						if err != nil {
							logger.Sugar().Errorf("failed to convert Retry-After into int64", zap.Error(err))
							break
						}
						retryAfter = time.Duration(retryAfterInt) * time.Second
					} else {
						retryAfter = g.config.AccrualRetryAfter
					}
					// setting RetryFlag to true
					rf.Store(true)
					time.Sleep(retryAfter)
					rf.Store(false)
					continue
				}

				if resp.StatusCode() == http.StatusOK {
					if err := json.Unmarshal(resp.Body(), &ar); err != nil {
						return fmt.Errorf("failed to unmarshal accrual response: %w", err)
					}
					order.Status = ar.Status
					order.Accrual = ar.Accrual
					if err := g.OrderUpdate(&order); err != nil {
						logger.Sugar().Error("failed to update order in DB", zap.Error(err))
					}
				}
				break
			}
		}
	}
}

func (g *GophermartService) OrderUpdate(order *models.Order) error {
	if err := g.store.UpdateOrder(g.config, order); err != nil {
		return fmt.Errorf("error updating order %s: %w", order.Number, err)
	}
	if order.Accrual != 0 {
		if err := g.store.UserAddAccrual(g.config, order); err != nil {
			return fmt.Errorf("failed to add accrual for user %s: %w", order.UserID, err)
		}
	}
	return nil
}
