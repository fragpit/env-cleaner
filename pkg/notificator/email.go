package notificator

import (
	"fmt"
)

type EmailNotificator struct {
	smtpServerAddress string
	smtpServerPort    int
	username          string
	password          string
}

func NewEmailNotificator(
	smtpServerAddress string,
	smtpServerPort int,
	username string,
	password string,
) *EmailNotificator {
	return &EmailNotificator{
		smtpServerAddress: smtpServerAddress,
		smtpServerPort:    smtpServerPort,
		username:          username,
		password:          password,
	}
}

type EmailMessage struct {
	From    string
	To      string
	Subject string
	Body    string
}

//nolint:revive
func NewEmailMessage(senderName, channel, text string) (*EmailMessage, error) {
	// TODO: Implement email message creation logic here

	return nil, fmt.Errorf("not implemented")
}

//nolint:revive
func (nt *EmailNotificator) Send(msg *EmailMessage) error {
	// TODO: Implement email sending logic here

	return fmt.Errorf("not implemented")
}
