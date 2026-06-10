package mail

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestMessageBuilder(t *testing.T) {
	builder := NewMessage().
		From("sender@example.com", "Sender Name").
		To("receiver@example.com").
		Subject("Test Subject").
		Body("Hello World")

	if builder.Error() != nil {
		t.Fatalf("unexpected builder error: %v", builder.Error())
	}

	if builder.fromEmail != "sender@example.com" {
		t.Errorf("expected sender email sender@example.com, got %s", builder.fromEmail)
	}

	if builder.fromName != "Sender Name" {
		t.Errorf("expected sender name 'Sender Name', got %s", builder.fromName)
	}

	if len(builder.to) != 1 || builder.to[0] != "receiver@example.com" {
		t.Errorf("expected receiver receiver@example.com, got %v", builder.to)
	}

	if builder.subject != "Test Subject" {
		t.Errorf("expected subject 'Test Subject', got %s", builder.subject)
	}

	if builder.body != "Hello World" {
		t.Errorf("expected body 'Hello World', got %s", builder.body)
	}
}

func TestMessageBuilderErrors(t *testing.T) {
	builder := NewMessage().
		From("", "Sender").
		To("receiver@example.com")

	if builder.Error() == nil {
		t.Fatal("expected error for empty from email")
	}

	builder = NewMessage().
		From("sender@example.com", "Sender").
		To()

	if builder.Error() == nil {
		t.Fatal("expected error for empty recipients list")
	}
}

func TestSMTPDeliveryMock(t *testing.T) {
	// Start a mock SMTP server locally
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock smtp server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	parts := strings.Split(addr, ":")
	host := parts[0]
	var port int
	_, _ = fmt.Sscanf(parts[1], "%d", &port)

	smtpConfig := SMTPConfig{
		Host:     host,
		Port:     port,
		Username: "",
		Password: "",
		From:     "sender@example.com",
		FromName: "Sender Name",
		UseSSL:   false,
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)

		// 1. Send greeting
		writer.WriteString("220 mock.smtp.server.local\r\n")
		writer.Flush()

		// 2. Read EHLO/HELO
		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(line, "EHLO") && !strings.HasPrefix(line, "HELO") {
			return
		}
		writer.WriteString("250-mock.smtp.server.local\r\n250 HELP\r\n")
		writer.Flush()

		// 3. Read MAIL FROM
		line, _ = reader.ReadString('\n')
		if !strings.HasPrefix(line, "MAIL FROM") {
			return
		}
		writer.WriteString("250 2.1.0 OK\r\n")
		writer.Flush()

		// 4. Read RCPT TO
		line, _ = reader.ReadString('\n')
		if !strings.HasPrefix(line, "RCPT TO") {
			return
		}
		writer.WriteString("250 2.1.5 OK\r\n")
		writer.Flush()

		// 5. Read DATA
		line, _ = reader.ReadString('\n')
		if !strings.HasPrefix(line, "DATA") {
			return
		}
		writer.WriteString("354 Start mail input; end with <CR><LF>.<CR><LF>\r\n")
		writer.Flush()

		// 6. Read mail body until "."
		for {
			line, _ = reader.ReadString('\n')
			if line == ".\r\n" {
				break
			}
		}
		writer.WriteString("250 2.0.0 OK: queued\r\n")
		writer.Flush()

		// 7. Read QUIT
		line, _ = reader.ReadString('\n')
		if strings.HasPrefix(line, "QUIT") {
			writer.WriteString("221 2.0.0 Bye\r\n")
			writer.Flush()
		}
	}()

	err = NewMessage().
		From("sender@example.com", "Sender Name").
		To("receiver@example.com").
		Subject("Mock Test").
		Body("Body content").
		Send(smtpConfig)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}
