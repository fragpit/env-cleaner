package service

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/fragpit/env-cleaner/internal/model"
)

const (
	crawlerOperationTimeout = 120 * time.Second
)

type Crawler struct {
	CrawlInterval string
	Connector     model.Connector
	Repository    model.Repository
}

func NewCrawler(
	crawlInt string,
	conn model.Connector,
	repo model.Repository,
) *Crawler {
	return &Crawler{
		CrawlInterval: crawlInt,
		Connector:     conn,
		Repository:    repo,
	}
}

func (c *Crawler) Run(ctx context.Context) {
	slog.Info("crawler service started",
		slog.String("type", c.Connector.GetConnectorType()),
		slog.String("interval", c.CrawlInterval),
	)
	runPeriodically(ctx, startCrawler, c)
}

func runPeriodically(
	ctx context.Context,
	f func(context.Context, *Crawler),
	c *Crawler,
) {
	interval, err := time.ParseDuration(c.CrawlInterval)
	if err != nil {
		slog.Error("error parsing duration", slog.Any("error", err))
		os.Exit(1)
	}

	f(ctx, c)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if ctx.Err() != nil {
				continue
			}
			f(ctx, c)
		case <-ctx.Done():
			slog.Info("crawler service shut down",
				slog.String("type", c.Connector.GetConnectorType()),
			)
			return
		}
	}
}

func startCrawler(ctx context.Context, c *Crawler) {
	slog.Info("crawler task started",
		slog.String("type", c.Connector.GetConnectorType()),
	)

	ctx, cancel := context.WithTimeout(
		ctx, crawlerOperationTimeout,
	)
	defer cancel()

	envs, err := c.Connector.GetEnvironments(ctx)
	if err != nil {
		slog.Error("error finding VMs", slog.Any("error", err))
		return
	}

	if envs != nil {
		slog.Info("writing environments to database")
		if err := c.Repository.WriteEnvironments(
			ctx, envs,
		); err != nil {
			slog.Error("error writing to DB", slog.Any("error", err))
			return
		}
	}

	slog.Info("crawler task finished",
		slog.String("type", c.Connector.GetConnectorType()),
	)
}
