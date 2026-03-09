package notifications

import (
	"fmt"
	"log/slog"

	"github.com/fragpit/env-cleaner/internal/model"
	"github.com/fragpit/env-cleaner/pkg/notificator"
)

type Notificator struct {
	adminOnly      bool
	apiURL         string
	staleThreshold string
	*SlackConfig
	*EmailConfig
}

type SlackConfig struct {
	AdminChannel string
	Enabled      bool
	SenderName   string
	WebhookURL   string

	SlackNotificator *notificator.SlackNotificator
}

type EmailConfig struct {
	Enabled           bool
	SMTPServerAddress string
	SMTPServerPort    int
	Username          string
	Password          string
	SenderEmail       string
	AdminEmail        string

	EmailNotificator *notificator.EmailNotificator
}

func New(
	adminOnly bool,
	apiURL string,
	staleThreshold string,
	slackCfg *SlackConfig,
	emailCfg *EmailConfig,
) *Notificator {
	if slackCfg.Enabled {
		slackCfg.SlackNotificator = notificator.NewSlackNotificator(
			slackCfg.WebhookURL,
		)
	}

	if emailCfg.Enabled {
		emailCfg.EmailNotificator = notificator.NewEmailNotificator(
			emailCfg.SMTPServerAddress,
			emailCfg.SMTPServerPort,
			emailCfg.Username,
			emailCfg.Password,
		)
	}

	return &Notificator{
		adminOnly:      adminOnly,
		apiURL:         apiURL,
		staleThreshold: staleThreshold,
		SlackConfig:    slackCfg,
		EmailConfig:    emailCfg,
	}
}

var orphanMessage = `
**Environment: %s, type: %s, is orphaned**
`

var staleMessage = `
**Environment %s, type: %s, is stale and will be deleted in %s**
[Extend your environment](%[4]s/extend?env_id=%[5]s&token=%[6]s)
`

var deleteMessage = `
**Environment: %s, type: %s, is outdated and has been deleted**
`

func (nt *Notificator) SendOrphanMessage(env *model.Environment) error {
	name := env.DisplayName()
	slog.Info("sending orphaned message",
		slog.String("environment", name),
		slog.String("type", env.Type),
	)

	if nt.SlackConfig.Enabled {
		slackChannel := nt.AdminChannel
		msg, err := notificator.NewSlackMessage(
			nt.SenderName,
			slackChannel,
			fmt.Sprintf(
				orphanMessage,
				name,
				env.Type,
			))

		if err != nil {
			return fmt.Errorf("error creating slack message: %w", err)
		}

		if err := nt.SlackNotificator.Send(msg); err != nil {
			return fmt.Errorf(
				"error sending slack notification, for environment %s: %w",
				name, err,
			)
		}
	}

	return nil
}

func (nt *Notificator) SendStaleMessage(
	env *model.Environment,
	tk *model.Token,
) error {
	name := env.DisplayName()
	slog.Info("sending stale message",
		slog.String("environment", name),
		slog.String("type", env.Type),
		slog.String("id", env.EnvID),
	)

	if nt.SlackConfig.Enabled {
		slackChannel := env.Owner
		if nt.adminOnly {
			slackChannel = nt.AdminChannel
		}

		msg, err := notificator.NewSlackMessage(
			nt.SenderName,
			slackChannel,
			fmt.Sprintf(staleMessage,
				name,
				env.Type,
				nt.staleThreshold,
				nt.apiURL,
				env.EnvID,
				tk.Token,
			))

		if err != nil {
			return fmt.Errorf(
				"error creating slack message for environment %s, type: %s, id: %s: %w",
				name, env.Type, env.EnvID, err)
		}

		if err := nt.SlackNotificator.Send(msg); err != nil {
			return fmt.Errorf(
				"error sending slack notification for environment %s, type: %s, id: %s: %w",
				name,
				env.Type,
				env.EnvID,
				err,
			)
		}
	}

	return nil
}

func (nt *Notificator) SendDeleteMessage(env *model.Environment) error {
	name := env.DisplayName()
	slog.Info("sending delete message",
		slog.String("environment", name),
		slog.String("type", env.Type),
		slog.String("id", env.EnvID),
	)

	if nt.SlackConfig.Enabled {
		slackChannel := env.Owner
		if nt.adminOnly {
			slackChannel = nt.AdminChannel
		}

		msg, err := notificator.NewSlackMessage(
			nt.SenderName,
			slackChannel,
			fmt.Sprintf(
				deleteMessage,
				name,
				env.Type,
			))

		if err != nil {
			return fmt.Errorf(
				"error creating slack message for environment %s, type: %s, id: %s: %w",
				name, env.Type, env.EnvID, err)
		}

		if err := nt.SlackNotificator.Send(msg); err != nil {
			return fmt.Errorf(
				"error sending slack notification for environment %s, type: %s, id: %s: %w",
				name,
				env.Type,
				env.EnvID,
				err,
			)
		}
	}

	return nil
}
