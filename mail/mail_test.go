package mail

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
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

func generateLargeImage(width, height int, isPNG bool, transparent bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	r := rand.New(rand.NewSource(42))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if transparent && x < 10 && y < 10 {
				img.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 128})
			} else {
				img.Set(x, y, color.RGBA{
					R: uint8(r.Intn(256)),
					G: uint8(r.Intn(256)),
					B: uint8(r.Intn(256)),
					A: 255,
				})
			}
		}
	}
	var buf bytes.Buffer
	if isPNG {
		_ = png.Encode(&buf, img)
	} else {
		_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	}
	return buf.Bytes()
}

func TestImageOptimization(t *testing.T) {
	// 1. Opaque JPEG > 100KB, width > 600px
	opaqueData := generateLargeImage(800, 600, false, false)
	if len(opaqueData) <= 100*1024 {
		t.Fatalf("test setup error: generated opaque JPEG must be > 100KB, got %d bytes", len(opaqueData))
	}

	optData, _, optType := optimizeImage(opaqueData, "photo.jpg", "image/jpeg")
	if len(optData) >= len(opaqueData) {
		t.Errorf("expected optimization to reduce size, got original %d vs optimized %d", len(opaqueData), len(optData))
	}
	if optType != "image/jpeg" {
		t.Errorf("expected content type image/jpeg, got %s", optType)
	}

	// Verify it was resized
	decodedImg, _, err := image.Decode(bytes.NewReader(optData))
	if err != nil {
		t.Fatalf("failed to decode optimized image: %v", err)
	}
	if decodedImg.Bounds().Dx() != 600 {
		t.Errorf("expected resized width 600, got %d", decodedImg.Bounds().Dx())
	}

	// 2. Transparent PNG > 100KB, width > 600px
	transData := generateLargeImage(800, 600, true, true)
	if len(transData) <= 100*1024 {
		t.Fatalf("test setup error: generated transparent PNG must be > 100KB, got %d bytes", len(transData))
	}

	optDataPNG, _, optTypePNG := optimizeImage(transData, "logo.png", "image/png")
	if len(optDataPNG) >= len(transData) {
		t.Errorf("expected optimization to reduce size for PNG, got original %d vs optimized %d", len(transData), len(optDataPNG))
	}
	if optTypePNG != "image/png" {
		t.Errorf("expected transparent PNG to keep image/png content type, got %s", optTypePNG)
	}

	// 3. Opaque PNG > 100KB, width > 600px -> should be converted to JPEG
	opaquePNGData := generateLargeImage(800, 600, true, false)
	if len(opaquePNGData) <= 100*1024 {
		t.Fatalf("test setup error: generated opaque PNG must be > 100KB, got %d bytes", len(opaquePNGData))
	}

	_, optNameOpaquePNG, optTypeOpaquePNG := optimizeImage(opaquePNGData, "chart.png", "image/png")
	if optTypeOpaquePNG != "image/jpeg" {
		t.Errorf("expected opaque PNG to be optimized to image/jpeg, got %s", optTypeOpaquePNG)
	}
	if !strings.HasSuffix(optNameOpaquePNG, ".jpg") {
		t.Errorf("expected filename to have .jpg suffix, got %s", optNameOpaquePNG)
	}
}

func TestAttachmentSizeLimits(t *testing.T) {
	// Test eager verification: individual attachment limit
	builder := NewMessage().
		WithMaxAttachmentSize(50).
		WithMaxTotalAttachmentsSize(150)

	// Adding a small attachment (10 bytes) - should pass
	builder.AttachBytes(make([]byte, 10), "small.txt", "text/plain")
	if builder.Error() != nil {
		t.Fatalf("unexpected error adding small attachment: %v", builder.Error())
	}

	// Adding a large attachment (60 bytes) - should trigger eager error
	builder.AttachBytes(make([]byte, 60), "large.txt", "text/plain")
	if builder.Error() == nil {
		t.Fatal("expected eager limit error for attachment exceeding 50 bytes limit")
	}
	if !strings.Contains(builder.Error().Error(), "exceeds the maximum allowed individual limit") {
		t.Errorf("unexpected error message: %v", builder.Error())
	}

	// Reset builder and test total attachments size limit
	builder = NewMessage().
		WithMaxAttachmentSize(100).
		WithMaxTotalAttachmentsSize(150)

	builder.AttachBytes(make([]byte, 80), "file1.txt", "text/plain")
	if builder.Error() != nil {
		t.Fatalf("unexpected error adding file1: %v", builder.Error())
	}

	// Adding file2 of size 80 would make total 160 > 150
	builder.AttachBytes(make([]byte, 80), "file2.txt", "text/plain")
	if builder.Error() == nil {
		t.Fatal("expected eager limit error for total attachments exceeding 150 bytes limit")
	}

	// Test late size limit validation in Send() after S3 resolution
	store := NewMockStore()
	smtpConfig := SMTPConfig{
		Host: "localhost",
		Port: 25,
	}

	builder = NewMessage().
		From("sender@example.com", "Sender").
		To("receiver@example.com").
		WithStore(store).
		WithMaxAttachmentSize(50)

	// Simulate a pre-existing 60-byte encrypted S3 attachment manually uploaded
	aesKey := []byte("aes_encryption_key_size_32_bytes") // 32 bytes
	plaintext := make([]byte, 60)
	encrypted, _ := EncryptAES(aesKey, plaintext)
	_ = store.UploadAttachment("s3key_large", encrypted)

	builder.AddQueuedAttachment("s3key_large", aesKey, "invoice.pdf", "application/pdf", false, "")
	if builder.Error() != nil {
		t.Fatalf("unexpected error from AddQueuedAttachment: %v", builder.Error())
	}

	// Send should fail due to decrypted size (60 bytes) > individual limit (50 bytes)
	err := builder.Send(smtpConfig)
	if err == nil {
		t.Fatal("expected Send to fail because downloaded attachment exceeds individual limit")
	}
	if !strings.Contains(err.Error(), "exceeds the maximum allowed individual limit") {
		t.Errorf("unexpected error message from Send: %v", err)
	}
}

func TestProviderPresets(t *testing.T) {
	// 1. Verify presets builder config
	m := NewMessage().WithPreset(YandexPreset)
	if m.maxAttachmentSize != YandexPreset.MaxAttachmentSize {
		t.Errorf("expected maxAttachmentSize to be Yandex value %d, got %d", YandexPreset.MaxAttachmentSize, m.maxAttachmentSize)
	}
	if m.maxTotalAttachmentsSize != YandexPreset.MaxTotalAttachmentsSize {
		t.Errorf("expected maxTotalAttachmentsSize to be Yandex value %d, got %d", YandexPreset.MaxTotalAttachmentsSize, m.maxTotalAttachmentsSize)
	}

	// 2. Verify config generation
	cfg := YandexPreset.SMTPConfig("user@yandex.ru", "my-password")
	if cfg.Host != "smtp.yandex.ru" || cfg.Port != 465 || cfg.UseSSL != true || cfg.Username != "user@yandex.ru" || cfg.Password != "my-password" {
		t.Errorf("incorrect Yandex SMTPConfig: %+v", cfg)
	}

	// 3. Verify Gmail preset config
	m2 := NewMessage().WithPreset(GmailPreset)
	if m2.maxAttachmentSize != GmailPreset.MaxAttachmentSize {
		t.Errorf("expected maxAttachmentSize to be Gmail value %d, got %d", GmailPreset.MaxAttachmentSize, m2.maxAttachmentSize)
	}

	cfg2 := GmailPreset.SMTPConfig("user@gmail.com", "pass")
	if cfg2.Host != "smtp.gmail.com" || cfg2.Port != 587 || cfg2.UseSSL != false || cfg2.Username != "user@gmail.com" || cfg2.Password != "pass" {
		t.Errorf("incorrect Gmail SMTPConfig: %+v", cfg2)
	}
}

func TestHTMLSizeLimits(t *testing.T) {
	// 1. Eager HTML limit validation
	builder := NewMessage().WithMaxHTMLSize(10)
	builder.HTML("1234567890") // 10 bytes (valid)
	if builder.Error() != nil {
		t.Fatalf("unexpected error setting valid HTML: %v", builder.Error())
	}

	builder = NewMessage().WithMaxHTMLSize(10)
	builder.HTML("12345678901") // 11 bytes (invalid)
	if builder.Error() == nil {
		t.Fatal("expected error for HTML size exceeding 10 bytes limit")
	}
	if !strings.Contains(builder.Error().Error(), "exceeds the maximum allowed limit") {
		t.Errorf("unexpected error message: %v", builder.Error())
	}

	// 2. Late HTML limit validation in Send()
	builder = NewMessage().
		From("sender@example.com", "Sender").
		To("receiver@example.com").
		WithMaxHTMLSize(10)

	// Circumvent eager checks (e.g. modify body directly for testing purposes)
	builder.body = "12345678901"
	builder.isHTML = true

	smtpConfig := SMTPConfig{
		Host: "localhost",
		Port: 25,
	}

	err := builder.Send(smtpConfig)
	if err == nil {
		t.Fatal("expected Send to fail because body exceeds HTML limit")
	}
	if !strings.Contains(err.Error(), "exceeds the maximum allowed limit") {
		t.Errorf("unexpected error message from Send: %v", err)
	}
}

func TestBase64ImageCompressionInHTML(t *testing.T) {
	opaqueData := generateLargeImage(800, 600, false, false)
	base64Opaque := base64.StdEncoding.EncodeToString(opaqueData)
	
	htmlInput := `<html><body><img src="data:image/jpeg;base64,` + base64Opaque + `" /></body></html>`
	
	// Set maxHTMLSize to a larger value so it doesn't fail eager validation
	builder := NewMessage().WithMaxHTMLSize(10 * 1024 * 1024)
	builder.HTML(htmlInput)
	
	if builder.Error() != nil {
		t.Fatalf("unexpected error: %v", builder.Error())
	}
	
	// Check that the body is HTML and the base64 string was compressed
	if !builder.isHTML {
		t.Error("expected body to be HTML")
	}
	
	// Extract the new base64 data and verify it is shorter
	matches := base64ImageRegexp.FindStringSubmatch(builder.body)
	if len(matches) < 3 {
		t.Fatalf("could not find base64 image in compressed HTML: %s", builder.body)
	}
	
	compressedBase64 := matches[2]
	if len(compressedBase64) >= len(base64Opaque) {
		t.Errorf("expected base64 length to decrease: original %d, compressed %d", len(base64Opaque), len(compressedBase64))
	}
}




