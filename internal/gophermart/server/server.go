package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

func NewServer(c *models.Config, gr chi.Router) *http.Server {
	return &http.Server{
		Addr:    c.Address,
		Handler: gr,
	}
}
