package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/fragpit/env-cleaner/internal/model"
)

const (
	deleterOperationTimeout = 120 * time.Second
)

type DeleterConfig struct {
	DeleteInterval string
	StaleThreshold string
	DryRun         bool
}

type Deleter struct {
	config      DeleterConfig
	Factory     ConnectorFactory
	Repository  model.Repository
	Notificator model.Notificator
}

func NewDeleter(
	cfg DeleterConfig,
	factory ConnectorFactory,
	repo model.Repository,
	nt model.Notificator,
) *Deleter {
	return &Deleter{
		config:      cfg,
		Factory:     factory,
		Repository:  repo,
		Notificator: nt,
	}
}

func (d *Deleter) Run(ctx context.Context) {
	slog.Info("deleter service started",
		slog.String("interval", d.config.DeleteInterval),
	)
	runDeleterPeriodically(ctx, startDeleter, d)
}

func runDeleterPeriodically(
	ctx context.Context,
	f func(ctx context.Context, d *Deleter),
	d *Deleter,
) {
	interval, err := time.ParseDuration(
		d.config.DeleteInterval,
	)
	if err != nil {
		slog.Error("error parsing duration", slog.Any("error", err))
		os.Exit(1)
	}

	f(ctx, d)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if ctx.Err() != nil {
				continue
			}
			f(ctx, d)
		case <-ctx.Done():
			slog.Info("deleter service shut down")
			return
		}
	}
}

func startDeleter(ctx context.Context, d *Deleter) {
	slog.Info("deleter task started")

	ctx, cancel := context.WithTimeout(
		ctx, deleterOperationTimeout,
	)
	defer cancel()

	envs, err := d.GetOutdatedEnvironments(ctx)
	if err != nil {
		slog.Error("error getting outdated environments", slog.Any("error", err))
		return
	}

	for _, env := range envs {
		connector, err := d.Factory.GetConnector(env.Type)
		if err != nil {
			slog.Error("error getting connector", slog.Any("error", err))
			continue
		}

		if err := connector.CheckEnvironment(
			ctx, env,
		); err != nil {
			slog.Error("error checking environment", slog.Any("error", err))
			continue
		}

		if !d.config.DryRun {
			if err := connector.DeleteEnvironment(
				ctx, env,
			); err != nil {
				slog.Error("error deleting environment", slog.Any("error", err))
				continue
			}

			if err := d.Repository.DeleteEnvironment(
				ctx, env.EnvID,
			); err != nil {
				slog.Error("error deleting environment from DB", slog.Any("error", err))
				continue
			}
		}

		if err := d.Notificator.SendDeleteMessage(
			env,
		); err != nil {
			slog.Error("error sending delete message", slog.Any("error", err))
			continue
		}
	}

	slog.Info("deleter task finished")
}

func (d *Deleter) GetStaleEnvironments(
	ctx context.Context,
) ([]*model.Environment, error) {
	staleThreshold, err := time.ParseDuration(
		d.config.StaleThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error getting stale environments: %w", err,
		)
	}

	staleThresholdSeconds := int64(staleThreshold.Seconds())
	env, err := d.Repository.GetStaleEnvironments(
		ctx, staleThresholdSeconds,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error getting stale environments: %w", err,
		)
	}

	return env, nil
}

func (d *Deleter) GetOutdatedEnvironments(
	ctx context.Context,
) ([]*model.Environment, error) {
	env, err := d.Repository.GetOutdatedEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"error getting outdated environment: %w", err,
		)
	}

	return env, nil
}
