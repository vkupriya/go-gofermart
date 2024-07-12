package storage

import (
	"fmt"

	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

type MemStorage struct {
	orders models.Orders
	users  models.Users
}

func NewMemStorage(c *models.Config) (*MemStorage, error) {
	return &MemStorage{
		orders: models.Orders{},
		users:  models.Users{},
	}, nil
}

func (m *MemStorage) OrdersAdd(user string, orderid string) error {
	m.orders = append(m.orders, models.Order{
		UserID:  user,
		OrderID: orderid,
	})
	return nil
}

func (m *MemStorage) OrdersGet(user string) (models.Orders, error) {
	var orders models.Orders
	for _, o := range m.orders {
		if o.UserID == user {
			orders = append(orders, o)
		}
	}
	return orders, nil
}

func (m *MemStorage) UserAdd(user string, passwd string) error {
	for _, u := range m.users {
		if u.User == user {
			return fmt.Errorf("user '%s' already exists", user)
		}
	}
	m.users = append(m.users, models.User{
		User:     user,
		Password: passwd,
	})

	return nil
}

func (m *MemStorage) UserGet(userid string) (models.User, error) {

	for _, u := range m.users {
		if u.User == userid {
			return u, nil
		}
	}
	return models.User{}, nil
}
