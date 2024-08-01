package gophermart

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"golang.org/x/sync/errgroup"

	"github.com/vkupriya/go-gophermart/internal/gophermart/config"
	"github.com/vkupriya/go-gophermart/internal/gophermart/server"
	"github.com/vkupriya/go-gophermart/internal/gophermart/server/handlers"
	"github.com/vkupriya/go-gophermart/internal/gophermart/service"
	"github.com/vkupriya/go-gophermart/internal/gophermart/storage"
)

func Start() (err error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	logger := cfg.Logger
	rootCtx, cancelCtx := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelCtx()

	g, ctx := errgroup.WithContext(rootCtx)

	_ = context.AfterFunc(ctx, func() {
		ctx, cancelCtx := context.WithTimeout(context.Background(), cfg.TimeoutShutdown)
		defer cancelCtx()

		<-ctx.Done()
		logger.Sugar().Error("failed to gracefully shutdown the service")
	})

	s, err := storage.NewPostgresDB(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize PostgresDB: %w", err)
	}

	svc := service.NewGophermartService(s, cfg)

	h := handlers.NewGophermartHandler(svc, cfg.Logger)
	r := handlers.NewGophermartRouter(cfg, h)
	srv := server.NewServer(cfg, r)

	logger.Sugar().Infow(
		"Starting server",
		"addr", cfg.Address,
	)

	g.Go(func() error {
		defer logger.Sugar().Info("closed DB")

		<-ctx.Done()

		s.Close()
		return nil
	})

	g.Go(func() (err error) {
		defer func() {
			errRec := recover()
			if errRec != nil {
				switch x := errRec.(type) {
				case string:
					err = errors.New(x)
					logger.Sugar().Error("a panic occured", zap.Error(err))
				case error:
					err = fmt.Errorf("a panic occurred: %w", x)
					logger.Sugar().Error(zap.Error(err))
				default:
					err = errors.New("unknown panic")
					logger.Sugar().Error(zap.Error(err))
				}
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
		defer logger.Sugar().Info("server has been shutdown")
		<-ctx.Done()

		shutdownTimeoutCtx, cancelShutdownTimeoutCtx := context.WithTimeout(context.Background(), cfg.TimeoutServerShutdown)
		defer cancelShutdownTimeoutCtx()
		if err := srv.Shutdown(shutdownTimeoutCtx); err != nil {
			return fmt.Errorf("an error occurred during server shutdown: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := svc.OrderDispatcher(ctx); err != nil {
			return fmt.Errorf("order fetcher has been terminated with error: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("go routines stopped with error: %w", err)
	}
	return nil
}
