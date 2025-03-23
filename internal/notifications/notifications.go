package notifications

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/xhit/go-str2duration/v2"

	"github.com/fragpit/env-cleaner/internal/model"
	"github.com/fragpit/env-cleaner/pkg/notificator"
)

type Notificator struct {
	adminOnly         bool
	apiURL            string
	staleThreshold    string
	maxExtendDuration string
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
	maxExtendDuration string,
	slackCfg *SlackConfig,
	emailCfg *EmailConfig,
) *Notificator {
	if slackCfg.Enabled {
		slackCfg.SlackNotificator = notificator.NewSlackNotificator(slackCfg.WebhookURL)
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
		adminOnly:         adminOnly,
		apiURL:            apiURL,
		staleThreshold:    staleThreshold,
		maxExtendDuration: maxExtendDuration,
		SlackConfig:       slackCfg,
		EmailConfig:       emailCfg,
	}
}

var orphanMessage = `
**Environment: %s, type: %s, is orphaned**
`

var staleMessage = `
**Environment %s, type: %s, is stale and will be deleted in %s**
Use one of the following links to extend your environment:
- [Extend %[7]s](%[4]s/extend?env_id=%[5]s&period=%[7]s&token=%[6]s)
- [Extend %[8]s](%[4]s/extend?env_id=%[5]s&period=%[8]s&token=%[6]s)
- [Extend %[9]s](%[4]s/extend?env_id=%[5]s&period=%[9]s&token=%[6]s)
`

var deleteMessage = `
**Environment: %s, type: %s, is outdated and has been deleted**
`

func (nt *Notificator) SendOrphanMessage(env *model.Environment) error {
	name := setNamespacedName(env)
	log.Infof("Sending orphaned message for environment %s, type: %s", name, env.Type)

	if nt.SlackConfig.Enabled {
		slackChannel := nt.AdminChannel
		msg, err := notificator.NewSlackMessage(
			nt.SlackConfig.SenderName,
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

func (nt *Notificator) SendStaleMessage(env *model.Environment, tk *model.Token) error {
	name := setNamespacedName(env)
	log.Infof(
		"Sending stale message for environment %s, type: %s, id: %s",
		name, env.Type, env.EnvID,
	)

	if nt.SlackConfig.Enabled {
		slackChannel := env.Owner
		if nt.adminOnly {
			slackChannel = nt.AdminChannel
		}

		extendPeriods, err := setExtendPeriods(nt.staleThreshold, nt.maxExtendDuration)
		if err != nil {
			return fmt.Errorf("error setting stale periods: %w", err)
		}
		msg, err := notificator.NewSlackMessage(
			nt.SlackConfig.SenderName,
			slackChannel,
			fmt.Sprintf(staleMessage,
				name,
				env.Type,
				nt.staleThreshold,
				nt.apiURL,
				env.EnvID,
				tk.Token,
				extendPeriods["min"],
				extendPeriods["mid"],
				extendPeriods["max"],
			))

		if err != nil {
			return fmt.Errorf(
				"error creating slack message for environment %s, type: %s, id: %s: %w",
				name, env.Type, env.EnvID, err)
		}

		if err := nt.SlackNotificator.Send(msg); err != nil {
			return fmt.Errorf(
				"error sending slack notification for environment %s, type: %s, id: %s: %w",
				name, env.Type, env.EnvID, err,
			)
		}
	}

	return nil
}

func (nt *Notificator) SendDeleteMessage(env *model.Environment) error {
	name := setNamespacedName(env)
	log.Infof(
		"Sending delete message for environment %s, type: %s, id: %s",
		name, env.Type, env.EnvID,
	)

	if nt.SlackConfig.Enabled {
		slackChannel := env.Owner
		if nt.adminOnly {
			slackChannel = nt.AdminChannel
		}

		msg, err := notificator.NewSlackMessage(
			nt.SlackConfig.SenderName,
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
				name, env.Type, env.EnvID, err,
			)
		}
	}

	return nil
}

func setNamespacedName(env *model.Environment) string {
	name := env.Name
	if env.Namespace != "" {
		name = fmt.Sprintf("%s (namespace: %s)", env.Name, env.Namespace)
	}

	return name
}

func setExtendPeriods(staleThreshold, maxExtendDuration string) (map[string]string, error) {
	maxDuration, err := str2duration.ParseDuration(maxExtendDuration)
	if err != nil {
		return nil, fmt.Errorf("error parsing max extend duration: %w", err)
	}

	midDuration := maxDuration / 2

	return map[string]string{
		"min": staleThreshold,
		"mid": str2duration.String(midDuration),
		"max": maxExtendDuration,
	}, nil
}
