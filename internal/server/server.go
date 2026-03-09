package server

import (
	"context"
	"log/slog"
	"os/signal"
	"sync"
	"syscall"

	"github.com/fragpit/env-cleaner/internal/api"
	"github.com/fragpit/env-cleaner/internal/config"
	"github.com/fragpit/env-cleaner/internal/connectors/helm"
	"github.com/fragpit/env-cleaner/internal/connectors/vsphere"
	"github.com/fragpit/env-cleaner/internal/model"
	"github.com/fragpit/env-cleaner/internal/notifications"
	"github.com/fragpit/env-cleaner/internal/service"
	"github.com/fragpit/env-cleaner/internal/storage/postgresql"
	"github.com/fragpit/env-cleaner/internal/storage/sqlite"
)

func Run() error {
	var err error
	var wg sync.WaitGroup

	cfg, err := config.NewServerConfig()
	if err != nil {
		slog.Error("error reading configuration", slog.Any("error", err))
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
			slog.Info("successfully connected to SQLite database",
				slog.String("folder", cfg.SQLite.DatabaseFolder),
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
			slog.Info("successfully connected to PostgreSQL database",
				slog.String("host", cfg.Postgresql.Host),
				slog.Int("port", cfg.Postgresql.Port),
				slog.String("database", cfg.Postgresql.Database),
			)
		}
	}
	if err != nil {
		slog.Error("error creating storage", slog.Any("error", err))
		return err
	}

	if st == nil {
		slog.Error("check storage configuration settings")
		return err
	}

	defer func() {
		if err := st.Close(); err != nil {
			slog.Error("error closing storage", slog.Any("error", err))
		}
	}()

	nt := notifications.New(
		cfg.Notifications.AdminOnly,
		cfg.APIURL,
		cfg.StaleThreshold,
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
		slog.Warn("dry run mode is enabled")
	}

	enabledConnectors := make(map[string]model.Connector)

	if !cfg.Environments.VSphereVM.Enabled && !cfg.Environments.Helm.Enabled {
		slog.Error(
			"check environments configuration settings: no connectors enabled",
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
			slog.Error("error creating vSphere connector", slog.Any("error", err))
			return err
		}

		vsCr := service.NewCrawler(cfg.CrawlInterval, vsConn, st)
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
			slog.Error("error creating Helm connector", slog.Any("error", err))
			return err
		}

		helmCr := service.NewCrawler(cfg.CrawlInterval, helmConn, st)
		wg.Add(1)
		go func() {
			defer wg.Done()
			helmCr.Run(ctx)
		}()

		enabledConnectors["helm"] = helmConn
	}

	factory := &service.ConnectorList{Connectors: enabledConnectors}
	deleter := service.NewDeleter(
		service.DeleterConfig{
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

	svc := service.NewEnvironmentService(st, factory, cfg.MaxExtendDuration)
	a := api.New(cfg, svc)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := a.Run(ctx); err != nil {
			slog.Error(
				"shutdown env-cleaner, error running API",
				slog.Any("error", err),
			)
			cancel()
		}
	}()

	wg.Wait()

	slog.Info("env-cleaner shut down gracefully")
	return nil
}
