package notificator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"log/slog"
)

type SlackNotificator struct {
	WebhookURL string
}

func NewSlackNotificator(webhookURL string) *SlackNotificator {
	return &SlackNotificator{
		WebhookURL: webhookURL,
	}
}

type SlackMessage struct {
	Username string `json:"username"`
	Channel  string `json:"channel"`
	Text     string `json:"text"`
}

func NewSlackMessage(senderName, channel, text string) (*SlackMessage, error) {
	if senderName == "" || channel == "" {
		return nil, fmt.Errorf("senderName and channel cannot be empty")
	}
	return &SlackMessage{
		Username: senderName,
		Channel:  "@" + channel,
		Text:     text,
	}, nil
}

func (nt *SlackNotificator) Send(msg *SlackMessage) error {
	slog.Debug("sending slack notification", slog.String("channel", msg.Channel))

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error marshaling slack message: %w", err)
	}

	resp, err := http.Post(nt.WebhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error sending slack notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}
