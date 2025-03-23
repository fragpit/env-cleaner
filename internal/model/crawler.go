package model

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	crawlerOperationTimeout = 120 * time.Second
)

type Crawler struct {
	CrawlInterval string
	Connector     Connector
	Repository    Repository
}

func NewCrawler(crawlInt string, conn Connector, repo Repository) *Crawler {
	return &Crawler{
		CrawlInterval: crawlInt,
		Connector:     conn,
		Repository:    repo,
	}
}

func (c *Crawler) Run(ctx context.Context) {
	log.Infof(
		"Crawler service started, type=%s, interval=%s",
		c.Connector.GetConnectorType(),
		c.CrawlInterval,
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
		log.Fatalf("Error parsing duration: %v", err)
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
			log.Infof(
				"Crawler service shut down, type=%s",
				c.Connector.GetConnectorType(),
			)
			return
		}
	}
}

func startCrawler(ctx context.Context, c *Crawler) {
	log.Infof("Crawler task started, type=%s", c.Connector.GetConnectorType())

	ctx, cancel := context.WithTimeout(ctx, crawlerOperationTimeout)
	defer cancel()

	envs, err := c.Connector.GetEnvironments(ctx)
	if err != nil {
		log.Errorf("Error finding VMs: %v", err)
		return
	}

	if envs != nil {
		log.Info("Writing environments to database")
		if err := c.Repository.WriteEnvironments(ctx, envs); err != nil {
			log.Errorf("Error writing to DB: %v", err)
			return
		}
	}

	log.Infof("Crawler task finished, type=%s", c.Connector.GetConnectorType())
}
