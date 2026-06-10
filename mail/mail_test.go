package mail

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/nativebpm/connectors/compress"
)

// MockStore is an in-memory implementation of AttachmentStore for testing.
type MockStore struct {
	mu    sync.Mutex
	store map[string][]byte
}

func NewMockStore() *MockStore {
	return &MockStore{
		store: make(map[string][]byte),
	}
}

func (s *MockStore) UploadAttachment(key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[key] = data
	return nil
}

func (s *MockStore) DownloadAttachment(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, exists := s.store[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return data, nil
}

func (s *MockStore) DeleteAttachment(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, key)
	return nil
}

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

	if builder.isHTML {
		t.Error("expected isHTML to be false for Body call")
	}
}

func TestMessageBuilderHTMLAndAttachments(t *testing.T) {
	builder := NewMessage().
		From("sender@example.com", "Sender").
		To("receiver@example.com").
		Subject("HTML Test").
		HTML("<h1>Hello HTML</h1>").
		EmbedBytes([]byte("dummy-png-data"), "banner.png", "image/png", "main_banner").
		AttachBytes([]byte("dummy-pdf-data"), "invoice.pdf", "application/pdf")

	if builder.Error() != nil {
		t.Fatalf("unexpected builder error: %v", builder.Error())
	}

	if !builder.isHTML {
		t.Error("expected isHTML to be true after HTML call")
	}

	if builder.body != "<h1>Hello HTML</h1>" {
		t.Errorf("expected body '<h1>Hello HTML</h1>', got %s", builder.body)
	}

	if len(builder.attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(builder.attachments))
	}

	att1 := builder.attachments[0]
	if att1.Filename != "banner.png" || att1.ContentType != "image/png" || att1.ContentID != "main_banner" || !att1.IsInline {
		t.Errorf("incorrect inline attachment configuration: %+v", att1)
	}

	att2 := builder.attachments[1]
	if att2.Filename != "invoice.pdf" || att2.ContentType != "application/pdf" || att2.ContentID != "" || att2.IsInline {
		t.Errorf("incorrect standard attachment configuration: %+v", att2)
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

func TestAESEncryptionDecryption(t *testing.T) {
	key := []byte("a_very_secret_32_bytes_key_12345") // 32 bytes
	plaintext := []byte("Hello, this is a secure BPMN transaction payload!")

	ciphertext, err := EncryptAES(key, plaintext)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("expected ciphertext to be different from plaintext")
	}

	decrypted, err := DecryptAES(key, ciphertext)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("expected decrypted text %s, got %s", string(plaintext), string(decrypted))
	}
}

func TestGzipCompressionDecompression(t *testing.T) {
	plaintext := []byte("Hello, this is a very long text to compress using gzip. Go packages handle this natively.")
	compressed, err := compress.GzipCompress(plaintext)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Fatal("expected compressed bytes to be non-empty")
	}

	decompressed, err := compress.GzipDecompress(compressed)
	if err != nil {
		t.Fatalf("decompression failed: %v", err)
	}

	if !bytes.Equal(plaintext, decompressed) {
		t.Errorf("expected decompressed %s, got %s", string(plaintext), string(decompressed))
	}
}

func TestS3EncryptedQueueDelivery(t *testing.T) {
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

	var capturedBody []string
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)

		writer.WriteString("220 mock.smtp.server.local\r\n")
		writer.Flush()

		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(line, "EHLO") && !strings.HasPrefix(line, "HELO") {
			return
		}
		writer.WriteString("250-mock.smtp.server.local\r\n250 HELP\r\n")
		writer.Flush()

		line, _ = reader.ReadString('\n')
		writer.WriteString("250 OK\r\n")
		writer.Flush()

		line, _ = reader.ReadString('\n')
		writer.WriteString("250 OK\r\n")
		writer.Flush()

		line, _ = reader.ReadString('\n')
		writer.WriteString("354 Start mail input\r\n")
		writer.Flush()

		for {
			line, _ = reader.ReadString('\n')
			if line == ".\r\n" {
				break
			}
			capturedBody = append(capturedBody, strings.TrimRight(line, "\r\n"))
		}
		writer.WriteString("250 OK: queued\r\n")
		writer.Flush()

		line, _ = reader.ReadString('\n')
		writer.WriteString("221 Bye\r\n")
		writer.Flush()
	}()

	store := NewMockStore()
	aesKey := []byte("aes_encryption_key_size_32_bytes") // 32 bytes
	payload := []byte("important_pdf_invoice_report_data")

	m := NewMessage().
		From("sender@example.com", "Sender Name").
		To("receiver@example.com").
		Subject("S3 Attachment Queue Test").
		WithStore(store).
		HTML("<h1>Open attached report</h1>")

	// 1. Queue attachment (encrypts and uploads)
	s3Key, err := m.QueueAttachment(store, aesKey, "invoice.pdf", "application/pdf", payload, false, "")
	if err != nil {
		t.Fatalf("failed to queue attachment: %v", err)
	}

	if s3Key == "" {
		t.Fatal("expected returned s3Key to be non-empty")
	}

	// 2. Check that store holds the encrypted data, not plain text
	storedData, err := store.DownloadAttachment(s3Key)
	if err != nil {
		t.Fatalf("failed to download from mock store: %v", err)
	}
	if bytes.Equal(storedData, payload) {
		t.Fatal("expected S3 stored payload to be encrypted, got raw plaintext")
	}

	// 3. Send the message (downloads, decrypts, sends, and deletes)
	err = m.Send(smtpConfig)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// 4. Verify SMTP content
	rawMail := strings.Join(capturedBody, "\n")
	if !strings.Contains(rawMail, "Content-Disposition: attachment; filename=\"invoice.pdf\"") {
		t.Error("expected SMTP body to contain the attachment filename headers")
	}

	// 5. Verify the temporary S3 key has been auto-deleted upon successful delivery
	_, err = store.DownloadAttachment(s3Key)
	if err == nil {
		t.Error("expected S3 key to be deleted automatically after Send call, but it still exists")
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

func TestSMTPDeliveryMockMultipart(t *testing.T) {
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

	var capturedBody []string
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)

		// 1. Greeting
		writer.WriteString("220 mock.smtp.server.local\r\n")
		writer.Flush()

		// 2. EHLO
		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(line, "EHLO") && !strings.HasPrefix(line, "HELO") {
			return
		}
		writer.WriteString("250-mock.smtp.server.local\r\n250 HELP\r\n")
		writer.Flush()

		// 3. MAIL FROM
		line, _ = reader.ReadString('\n')
		writer.WriteString("250 OK\r\n")
		writer.Flush()

		// 4. RCPT TO
		line, _ = reader.ReadString('\n')
		writer.WriteString("250 OK\r\n")
		writer.Flush()

		// 5. DATA
		line, _ = reader.ReadString('\n')
		writer.WriteString("354 Start mail input\r\n")
		writer.Flush()

		// 6. Read raw email MIME lines
		for {
			line, _ = reader.ReadString('\n')
			if line == ".\r\n" {
				break
			}
			capturedBody = append(capturedBody, strings.TrimRight(line, "\r\n"))
		}
		writer.WriteString("250 OK: queued\r\n")
		writer.Flush()

		// 7. QUIT
		line, _ = reader.ReadString('\n')
		writer.WriteString("221 Bye\r\n")
		writer.Flush()
	}()

	err = NewMessage().
		From("sender@example.com", "Sender Name").
		To("receiver@example.com").
		Subject("HTML Multipart Mock Test").
		HTML("<h1>Mock HTML Body</h1>").
		EmbedBytes([]byte("inline-banner-data"), "banner.png", "image/png", "my_banner").
		AttachBytes([]byte("standard-attach-data"), "docs.pdf", "application/pdf").
		Send(smtpConfig)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	rawMail := strings.Join(capturedBody, "\n")

	// Validate SMTP message structure
	if !strings.Contains(rawMail, "Content-Type: multipart/related;") {
		t.Error("expected Content-Type to be multipart/related")
	}

	if !strings.Contains(rawMail, "Content-Type: text/html; charset=\"utf-8\"") {
		t.Error("expected HTML content-type part")
	}

	if !strings.Contains(rawMail, "<h1>Mock HTML Body</h1>") {
		t.Error("expected HTML body inside captured email content")
	}

	if !strings.Contains(rawMail, "Content-ID: <my_banner>") {
		t.Error("expected Content-ID header for embedded banner")
	}

	if !strings.Contains(rawMail, "Content-Disposition: inline; filename=\"banner.png\"") {
		t.Error("expected Content-Disposition inline for embedded banner")
	}

	if !strings.Contains(rawMail, "Content-Disposition: attachment; filename=\"docs.pdf\"") {
		t.Error("expected Content-Disposition attachment for docs.pdf")
	}
}
