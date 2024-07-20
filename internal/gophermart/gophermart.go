package gophermart

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/vkupriya/go-gophermart/internal/gophermart/config"
	"github.com/vkupriya/go-gophermart/internal/gophermart/server"
	"github.com/vkupriya/go-gophermart/internal/gophermart/server/handlers"
	"github.com/vkupriya/go-gophermart/internal/gophermart/service"
)

func Start() (err error) {
	const (
		timeoutServerShutdown = time.Second * 5
		timeoutShutdown       = time.Second * 10
	)

	rootCtx, cancelCtx := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelCtx()

	g, ctx := errgroup.WithContext(rootCtx)

	context.AfterFunc(ctx, func() {
		ctx, cancelCtx := context.WithTimeout(context.Background(), timeoutShutdown)
		defer cancelCtx()

		<-ctx.Done()
		log.Fatal("failed to gracefully shutdown the service")
	})

	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}
	logger := cfg.Logger

	s, err := service.NewStore(cfg)
	if err != nil {
		logger.Sugar().Fatal(err)
	}

	svc := service.NewGophermartService(s, cfg)

	h := handlers.NewGophermartHandler(svc, cfg)
	r := handlers.NewGophermartRouter(h)
	srv := server.NewServer(cfg, r)

	logger.Sugar().Infow(
		"Starting server",
		"addr", cfg.Address,
	)

	g.Go(func() (err error) {
		defer func() {
			errRec := recover()
			if errRec != nil {
				err = fmt.Errorf("a panic occurred: %v", errRec)
			}
		}()
		if err = srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return
			}
			return fmt.Errorf("listen and server has failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		defer log.Print("server has been shutdown")
		<-ctx.Done()

		shutdownTimeoutCtx, cancelShutdownTimeoutCtx := context.WithTimeout(context.Background(), timeoutServerShutdown)
		defer cancelShutdownTimeoutCtx()
		if err := srv.Shutdown(shutdownTimeoutCtx); err != nil {
			log.Printf("an error occurred during server shutdown: %v", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := svc.SvcOrderFetcher(ctx); err != nil {
			return fmt.Errorf("order fetcher has been terminate with error: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("go routines stopped with error: %w", err)
	}

	return nil
}
