package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/fragpit/env-cleaner/internal/config"
	"github.com/fragpit/env-cleaner/internal/model"
)

const (
	apiShutdownTimeout = 15 * time.Second
)

type EnvironmentService interface {
	GetEnvironments(ctx context.Context) ([]*model.Environment, error)
	AddEnvironment(ctx context.Context, env *model.Environment, ttl string) error
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
	log.Info("Starting API")

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.CleanPath)
	r.Use(a.authMiddleware)

	envHandler := NewEnvironmentHandler(a.service)
	r.Get("/api/environments", envHandler.GetEnvironments)
	r.Post("/api/environments", envHandler.AddEnvironment)
	r.Get("/extend", envHandler.ExtendEnvironment)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("failed to start server: %v", err)
			errChan <- fmt.Errorf("failed to start server: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("Received shutdown signal, shutting down API service")
		ctx, cancel := context.WithTimeout(ctx, apiShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Errorf("Failed to shutdown API service gracefully: %v", err)
			return fmt.Errorf("failed to shutdown API service gracefully: %w", err)
		}

		log.Info("API service shut down")
		return nil
	case err := <-errChan:
		if err != nil {
			log.Errorf("API service encountered an error: %v", err)
			return err
		}
	}

	return nil
}

