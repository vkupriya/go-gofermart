package gophermart

import (
	"log"
	"net/http"

	"github.com/vkupriya/go-gophermart/internal/gophermart/config"
	"github.com/vkupriya/go-gophermart/internal/gophermart/handlers"
	"github.com/vkupriya/go-gophermart/internal/gophermart/service"
	//"go.uber.org/zap"
)

func Start() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}
	logger := cfg.Logger

	s, err := service.NewStore(cfg)
	if err != nil {
		logger.Sugar().Fatal(err)
	}

	svc := service.NewGophermartService(s, cfg)

	h := handlers.NewGophermartHandler(svc, cfg)

	s.OrdersAdd("vkupriya", "0123456")

	r := handlers.NewGophermartRouter(h)

	logger.Sugar().Infow(
		"Starting server",
		"addr", cfg.Address,
	)

	if err := http.ListenAndServe(cfg.Address, r); err != nil {
		logger.Sugar().Fatalw(err.Error(), "event", "start server")
	}

}
