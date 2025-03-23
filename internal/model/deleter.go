package model

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
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
	Repository  Repository
	Notificator Notificator
}

func NewDeleter(
	cfg DeleterConfig,
	factory ConnectorFactory,
	repo Repository,
	nt Notificator,
) *Deleter {
	return &Deleter{
		config:      cfg,
		Factory:     factory,
		Repository:  repo,
		Notificator: nt,
	}
}

func (d *Deleter) Run(ctx context.Context) {
	log.Infof("Deleter service started, interval=%s", d.config.DeleteInterval)
	runDeleterPeriodically(ctx, startDeleter, d)
}

func runDeleterPeriodically(
	ctx context.Context,
	f func(ctx context.Context, d *Deleter),
	d *Deleter,
) {
	interval, err := time.ParseDuration(d.config.DeleteInterval)
	if err != nil {
		log.Fatalf("Error parsing duration: %v", err)
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
			log.Info("Deleter service shut down")
			return
		}
	}
}

func startDeleter(ctx context.Context, d *Deleter) {
	log.Info("Deleter task started")

	ctx, cancel := context.WithTimeout(ctx, deleterOperationTimeout)
	defer cancel()

	envs, err := d.GetOutdatedEnvironments(ctx)
	if err != nil {
		log.Errorf("Error getting outdated environments: %v", err)
		return
	}

	for _, env := range envs {
		connector, err := d.Factory.GetConnector(env.Type)
		if err != nil {
			log.Errorf("Error getting connector: %v", err)
			continue
		}

		if err := connector.CheckEnvironment(ctx, env); err != nil {
			log.Errorf("Error checking environment: %v", err)
			continue
		}

		if !d.config.DryRun {
			if err := connector.DeleteEnvironment(ctx, env); err != nil {
				log.Errorf("Error deleting environment: %v", err)
				continue
			}

			if err := d.Repository.DeleteEnvironment(ctx, env.EnvID); err != nil {
				log.Errorf("Error deleting environment from DB: %v", err)
				continue
			}
		}

		if err := d.Notificator.SendDeleteMessage(env); err != nil {
			log.Errorf("Error sending delete message: %v", err)
			continue
		}
	}

	log.Info("Deleter task finished")
}

func (d *Deleter) GetStaleEnvironments(
	ctx context.Context,
) ([]*Environment, error) {
	staleThreshold, err := time.ParseDuration(d.config.StaleThreshold)
	if err != nil {
		return nil, fmt.Errorf("error getting stale environments: %w", err)
	}

	staleThresholdSeconds := int64(staleThreshold.Seconds())
	env, err := d.Repository.GetStaleEnvironments(ctx, staleThresholdSeconds)
	if err != nil {
		return nil, fmt.Errorf("error getting stale environments: %w", err)
	}

	return env, nil
}

func (d *Deleter) GetOutdatedEnvironments(
	ctx context.Context,
) ([]*Environment, error) {
	env, err := d.Repository.GetOutdatedEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting outdated environment: %w", err)
	}

	return env, nil
}
