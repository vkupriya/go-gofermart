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

func (m *MemStorage) OrdersAdd(userid string, orderid string) error {
	m.orders = append(m.orders, models.Order{
		UserID:  userid,
		OrderID: orderid,
	})
	return nil
}

func (m *MemStorage) OrdersGet(userid string) (models.Orders, error) {
	var orders models.Orders
	for _, o := range m.orders {
		if o.UserID == userid {
			orders = append(orders, o)
		}
	}
	return orders, nil
}

func (m *MemStorage) UserAdd(userid string, passwd string) error {
	for _, u := range m.users {
		if u.User == userid {
			return fmt.Errorf("user '%s' already exists", userid)
		}
	}
	m.users = append(m.users, models.User{
		User:     userid,
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
