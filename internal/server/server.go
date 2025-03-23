package server

import (
	"context"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/fragpit/env-cleaner/internal/api"
	"github.com/fragpit/env-cleaner/internal/config"
	"github.com/fragpit/env-cleaner/internal/connectors/helm"
	"github.com/fragpit/env-cleaner/internal/connectors/vsphere"
	"github.com/fragpit/env-cleaner/internal/model"
	"github.com/fragpit/env-cleaner/internal/notifications"
	"github.com/fragpit/env-cleaner/internal/storage/postgresql"
	"github.com/fragpit/env-cleaner/internal/storage/sqlite"
)

func Run() error {
	var err error
	var wg sync.WaitGroup

	cfg, err := config.NewServerConfig()
	if err != nil {
		log.Errorf("Error reading configuration: %v", err)
		return err
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer cancel()

	var st model.Repository
	if cfg.SQLite.DatabaseFolder != "" {
		st, err = sqlite.New(cfg.SQLite.DatabaseFolder)
		if err == nil {
			log.Infof(
				"Successfully connected to SQLite database, folder: %s",
				cfg.SQLite.DatabaseFolder,
			)
		}
	} else if cfg.Postgresql.Host != "" {
		st, err = postgresql.New(
			cfg.Postgresql.Host,
			cfg.Postgresql.Port,
			cfg.Postgresql.Username,
			cfg.Postgresql.Password,
			cfg.Postgresql.Database,
		)
		if err == nil {
			log.Infof(
				"Successfully connected to PostgreSQL database, server: %s:%d, database: %s",
				cfg.Postgresql.Host,
				cfg.Postgresql.Port,
				cfg.Postgresql.Database,
			)
		}
	}
	if err != nil {
		log.Errorf("Error creating storage: %v", err)
		return err
	}

	if st == nil {
		log.Errorf("Check storage configuration settings")
		return err
	}

	defer func() {
		if err := st.Close(); err != nil {
			log.Errorf("Error closing storage: %v", err)
		}
	}()

	nt := notifications.New(
		cfg.Notifications.AdminOnly,
		cfg.APIURL,
		cfg.StaleThreshold,
		cfg.MaxExtendDuration,
		&notifications.SlackConfig{
			Enabled:      cfg.Notifications.Slack.Enabled,
			WebhookURL:   cfg.Notifications.Slack.WebhookURL,
			SenderName:   cfg.Notifications.Slack.SenderName,
			AdminChannel: cfg.Notifications.Slack.AdminChannel,
		},
		&notifications.EmailConfig{
			Enabled:           cfg.Notifications.Email.Enabled,
			SMTPServerAddress: cfg.Notifications.Email.SMTPServerAddress,
			SMTPServerPort:    cfg.Notifications.Email.SMTPServerPort,
			Username:          cfg.Notifications.Email.Username,
			Password:          cfg.Notifications.Email.Password,
			SenderEmail:       cfg.Notifications.Email.SenderEmail,
			AdminEmail:        cfg.Notifications.Email.AdminEmail,
		},
	)

	if cfg.DryRun {
		log.Warn("Dry run mode is enabled")
	}

	enabledConnectors := make(map[string]model.Connector)

	if !cfg.Environments.VSphereVM.Enabled && !cfg.Environments.Helm.Enabled {
		log.Errorf(
			"Check environments configuration settings: no connectors enabled",
		)
		return err
	}

	if cfg.Environments.VSphereVM.Enabled {
		vsConfig := vsphere.Config{
			EnvCfg:  cfg.Environments.VSphereVM,
			ConnCfg: cfg.Connectors.VSphere,
		}

		vsConn, err := vsphere.New(ctx, &vsConfig, nt)
		if err != nil {
			log.Errorf("Error creating vSphere connector: %v", err)
			return err
		}

		vsCr := model.NewCrawler(cfg.CrawlInterval, vsConn, st)
		wg.Add(1)
		go func() {
			defer wg.Done()
			vsCr.Run(ctx)
		}()

		enabledConnectors["vsphere_vm"] = vsConn
	}

	if cfg.Environments.Helm.Enabled {
		helmConfig := helm.Config{
			EnvCfg:  cfg.Environments.Helm,
			ConnCfg: cfg.Connectors.K8s,
		}

		helmConn, err := helm.New(&helmConfig, nt)
		if err != nil {
			log.Errorf("Error creating Helm connector: %v", err)
			return err
		}

		helmCr := model.NewCrawler(cfg.CrawlInterval, helmConn, st)
		wg.Add(1)
		go func() {
			defer wg.Done()
			helmCr.Run(ctx)
		}()

		enabledConnectors["helm"] = helmConn
	}

	factory := &model.ConnectorList{Connectors: enabledConnectors}
	deleter := model.NewDeleter(
		model.DeleterConfig{
			DeleteInterval: cfg.DeleteInterval,
			StaleThreshold: cfg.StaleThreshold,
			DryRun:         cfg.DryRun,
		},
		factory,
		st,
		nt,
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		deleter.Run(ctx)
	}()

	a := api.New(cfg, factory, st)
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.Run(ctx)
	}()

	wg.Wait()

	log.Info("Env-cleaner shut down gracefully")
	return nil
}
