package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/fragpit/env-cleaner/internal/config"
	"github.com/fragpit/env-cleaner/internal/model"
)

const (
	apiShutdownTimeout = 15 * time.Second
)

type EnvironmentService interface {
	GetEnvironments(ctx context.Context) ([]*model.Environment, error)
	AddEnvironment(ctx context.Context, env *model.Environment, ttl string) error
	GetEnvironmentForExtend(
		ctx context.Context,
		envID, token string,
	) (*model.Environment, error)
	ExtendEnvironment(
		ctx context.Context,
		envID, period, token string,
	) (*model.Environment, error)
}

type API struct {
	Config  config.ServerConfig
	service EnvironmentService
}

func New(
	cfg *config.ServerConfig,
	svc EnvironmentService,
) *API {
	return &API{
		Config:  *cfg,
		service: svc,
	}
}

func (a *API) Run(ctx context.Context) error {
	slog.Info("starting API")

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.CleanPath)

	envHandler := NewEnvironmentHandler(a.service)
	extendPage := NewExtendPageHandler(
		a.service,
		a.Config.StaleThreshold,
		a.Config.MaxExtendDuration,
	)

	r.Group(func(r chi.Router) {
		r.Get("/extend", extendPage.ServePage)
		r.Get("/extend/static/extend.css", extendPage.ServeCSS)
		r.Get("/extend/static/extend.js", extendPage.ServeJS)
	})

	r.Group(func(r chi.Router) {
		r.Use(a.authMiddleware)
		r.Get("/api/environments", envHandler.GetEnvironments)
		r.Post("/api/environments", envHandler.AddEnvironment)
	})

	r.Group(func(r chi.Router) {
		r.Post("/api/environments/{id}/extend", envHandler.ExtendEnvironment)
		r.Get("/api/openapi.yaml", serveOpenAPISpec)
	})

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start server", slog.Any("error", err))
			errChan <- fmt.Errorf("failed to start server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("received shutdown signal, shutting down API service")
		ctx, cancel := context.WithTimeout(ctx, apiShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error(
				"failed to shutdown API service gracefully",
				slog.Any("error", err),
			)
			return fmt.Errorf("failed to shutdown API service gracefully: %w", err)
		}

		slog.Info("API service shut down gracefully")
		return nil
	case err := <-errChan:
		if err != nil {
			slog.Error("API service encountered an error", slog.Any("error", err))
			return err
		}
	}

	return nil
}
