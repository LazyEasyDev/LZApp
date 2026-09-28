package email

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/LazyEasyDev/LZApp/config/email_config"
	mail "github.com/wneessen/go-mail"
)

type Sender struct {
	host          string
	fromEmail     string
	timeout       time.Duration
	clientOptions []mail.Option
}

func New(settings *email_config.EmailConfig) (*Sender, error) {
	if settings == nil {
		return nil, fmt.Errorf("email configuration is required")
	}
	if strings.TrimSpace(settings.Host) == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}
	if err := mail.NewMsg().From(settings.FromEmail); err != nil {
		return nil, fmt.Errorf("invalid email sender: %w", err)
	}
	if settings.Password == "" {
		return nil, fmt.Errorf("SMTP password is required")
	}
	password := settings.Password
	if strings.TrimSpace(settings.Username) == "" || password == "" {
		return nil, fmt.Errorf("SMTP username and password are required")
	}
	port := settings.Port
	if port == 0 {
		port = 587
	}
	timeoutSeconds := settings.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = 8
	}
	const maxTimeoutSeconds = int64((1<<63 - 1) / time.Second)
	if timeoutSeconds < 0 || int64(timeoutSeconds) > maxTimeoutSeconds {
		return nil, fmt.Errorf("SMTP timeout must be positive seconds within time.Duration range")
	}
	sender := &Sender{
		host:      settings.Host,
		fromEmail: settings.FromEmail,
		timeout:   time.Duration(timeoutSeconds) * time.Second,
		clientOptions: []mail.Option{
			mail.WithPort(port),
			mail.WithTimeout(time.Duration(timeoutSeconds) * time.Second),
			mail.WithTLSPolicy(mail.TLSMandatory),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(settings.Username),
			mail.WithPassword(password),
		},
	}
	if _, err := mail.NewClient(sender.host, sender.clientOptions...); err != nil {
		return nil, fmt.Errorf("configure SMTP client: %w", err)
	}
	return sender, nil
}

func (sender *Sender) Send(ctx context.Context, toAddress, subject, body string) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if sender == nil || len(sender.clientOptions) == 0 {
		return fmt.Errorf("email sender is not configured")
	}
	message := mail.NewMsg()
	if err := message.From(sender.fromEmail); err != nil {
		return fmt.Errorf("invalid email sender: %w", err)
	}
	if err := message.To(toAddress); err != nil {
		return fmt.Errorf("invalid email recipient: %w", err)
	}
	message.Subject(subject)
	message.SetBodyString(mail.TypeTextPlain, body)
	sendContext, cancel := context.WithTimeout(ctx, sender.timeout)
	defer cancel()
	var connection net.Conn
	var stopCancellation func() bool
	defer func() {
		if stopCancellation != nil {
			stopCancellation()
		}
		if connection != nil {
			_ = connection.Close()
		}
	}()
	options := append([]mail.Option(nil), sender.clientOptions...)
	options = append(options, mail.WithDialContextFunc(func(dialContext context.Context, network, address string) (net.Conn, error) {
		dialedConnection, err := (&net.Dialer{}).DialContext(dialContext, network, address)
		if err != nil {
			return nil, err
		}
		connection = dialedConnection
		stopCancellation = context.AfterFunc(sendContext, func() {
			_ = dialedConnection.Close()
		})
		return dialedConnection, nil
	}))
	client, err := mail.NewClient(sender.host, options...)
	if err != nil {
		return fmt.Errorf("configure SMTP client: %w", err)
	}
	if err := client.DialAndSendWithContext(sendContext, message); err != nil {
		if contextErr := sendContext.Err(); contextErr != nil {
			return fmt.Errorf("send email: %w", contextErr)
		}
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
