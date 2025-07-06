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
	"github.com/fragpit/env-cleaner/pkg/utils"
)

const (
	apiShutdownTimeout = 15 * time.Second
)

type API struct {
	Config           config.ServerConfig
	ConnectorFactory model.ConnectorFactory
	Repository       model.Repository
}

func New(
	cfg *config.ServerConfig,
	factory model.ConnectorFactory,
	repo model.Repository,
) *API {
	return &API{
		Config:           *cfg,
		ConnectorFactory: factory,
		Repository:       repo,
	}
}

func (a *API) Run(ctx context.Context) error {
	log.Info("Starting API")

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.CleanPath)
	r.Use(a.authMiddleware)

	r.Get("/api/environments", a.getEnvironments)
	r.Post("/api/environments", a.addEnvironment)

	r.Get("/extend", a.extendFunc)

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

func (a *API) extendFunc(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queryParams := r.URL.Query()
	envID := queryParams.Get("env_id")
	period := queryParams.Get("period")
	token := queryParams.Get("token")

	tk, err := a.Repository.GetToken(ctx, envID)
	if err != nil || tk.Token != token {
		log.Errorf("Error getting token for environment id: %s, %v", envID, err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	if err := utils.PeriodValidate(period, a.Config.MaxExtendDuration); err != nil {
		log.Errorf(
			"Error validating period for environment id: %s, period: %s %v",
			envID,
			period,
			err,
		)
		http.Error(w, "Invalid period", http.StatusBadRequest)
		return
	}

	env, err := a.Repository.GetEnvByID(ctx, envID)
	if err != nil {
		log.Errorf("Error getting env by ID: %s, %v", envID, err)
		http.Error(w, "Environment not found", http.StatusNotFound)
		return
	}

	if err := a.Repository.ExtendEnvironment(ctx, envID, period); err != nil {
		log.Errorf("Error extending environment id: %s %v", envID, err)
		http.Error(
			w,
			"Failed to extend environment",
			http.StatusInternalServerError,
		)
		return
	}

	if err := a.Repository.DeleteToken(ctx, env.EnvID); err != nil {
		log.Errorf(
			"Error deleting token for environment id: %s, %v",
			env.EnvID,
			err,
		)
	}

	log.Infof(
		"Extended environment: %s, type: %s, id: %s, period: %s, token: %s",
		setName(env), env.Type, env.EnvID, period, token,
	)

	msg := fmt.Sprintf("Extended environment: %s, type: %s, period: %s",
		setName(env), env.Type, period)
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, msg); err != nil {
		log.Errorf("Error writing response, environment id: %s, %v", envID, err)
	}
}

func setName(env *model.Environment) string {
	name := env.Name
	if env.Namespace != "" {
		name = fmt.Sprintf("%s (namespace: %s)", env.Name, env.Namespace)
	}

	return name
}
