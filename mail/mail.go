package mail

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPConfig represents the SMTP server settings.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	FromName string `json:"from_name"`
	UseSSL   bool   `json:"use_ssl"`
}

// MessageBuilder is a fluent builder for email messages.
type MessageBuilder struct {
	fromEmail string
	fromName  string
	to        []string
	subject   string
	body      string
	err       error // Sticky error
}

// NewMessage creates a new fluent MessageBuilder.
func NewMessage() *MessageBuilder {
	return &MessageBuilder{}
}

// From configures the sender details.
func (m *MessageBuilder) From(email, name string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	if email == "" {
		m.err = fmt.Errorf("from email cannot be empty")
		return m
	}
	m.fromEmail = email
	m.fromName = name
	return m
}

// To appends target recipient emails.
func (m *MessageBuilder) To(emails ...string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	var valid []string
	for _, email := range emails {
		trimmed := strings.TrimSpace(email)
		if trimmed != "" {
			valid = append(valid, trimmed)
		}
	}
	if len(valid) == 0 {
		m.err = fmt.Errorf("at least one recipient email is required")
		return m
	}
	m.to = append(m.to, valid...)
	return m
}

// Subject sets the mail subject header.
func (m *MessageBuilder) Subject(subject string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	m.subject = subject
	return m
}

// Body sets the plain-text mail body payload.
func (m *MessageBuilder) Body(body string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	m.body = body
	return m
}

// Error returns the accumulated error if any, supporting sticky error check.
func (m *MessageBuilder) Error() error {
	return m.err
}

// Send delivers the message using the provided SMTP server configuration.
func (m *MessageBuilder) Send(config SMTPConfig) error {
	if m.err != nil {
		return m.err
	}
	if len(m.to) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	fromEmail := config.From
	if m.fromEmail != "" {
		fromEmail = m.fromEmail
	}
	fromName := config.FromName
	if m.fromName != "" {
		fromName = m.fromName
	}

	// Prepare RFC 822 formatted message headers and payload
	var headers []string
	if fromName != "" {
		headers = append(headers, fmt.Sprintf("From: %s <%s>", fromName, fromEmail))
	} else {
		headers = append(headers, fmt.Sprintf("From: %s", fromEmail))
	}
	headers = append(headers, fmt.Sprintf("To: %s", strings.Join(m.to, ", ")))
	headers = append(headers, fmt.Sprintf("Subject: %s", m.subject))
	headers = append(headers, "MIME-Version: 1.0")
	headers = append(headers, "Content-Type: text/plain; charset=\"utf-8\"")
	headers = append(headers, "")
	headers = append(headers, m.body)

	msg := []byte(strings.Join(headers, "\r\n"))
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	var auth smtp.Auth
	if config.Username != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	if config.UseSSL {
		// Secure SSL/TLS dial (commonly port 465)
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // Allow self-signed certs in local/test settings
			ServerName:         config.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to dial SMTP via SSL/TLS: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, config.Host)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err = client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP SSL/TLS auth failed: %w", err)
			}
		}

		if err = client.Mail(fromEmail); err != nil {
			return fmt.Errorf("SMTP MAIL command failed: %w", err)
		}
		for _, recipient := range m.to {
			if err = client.Rcpt(recipient); err != nil {
				return fmt.Errorf("SMTP RCPT command failed for %s: %w", recipient, err)
			}
		}

		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("SMTP DATA command failed: %w", err)
		}
		_, err = w.Write(msg)
		if err != nil {
			return fmt.Errorf("failed to write message body: %w", err)
		}
		err = w.Close()
		if err != nil {
			return fmt.Errorf("failed to close SMTP data writer: %w", err)
		}

		return client.Quit()
	}

	// Normal connection with STARTTLS upgrading (typically port 587)
	err := smtp.SendMail(addr, auth, fromEmail, m.to, msg)
	if err != nil {
		return fmt.Errorf("failed to send SMTP mail: %w", err)
	}

	return nil
}
