package mail

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// AttachmentStore is a self-hosted S3/MinIO/Local abstraction for temporary attachment storage.
type AttachmentStore interface {
	UploadAttachment(key string, data []byte) error
	DownloadAttachment(key string) ([]byte, error)
	DeleteAttachment(key string) error
}

// Attachment represents a file embedded as an inline CID or standard attachment in the HTML email.
type Attachment struct {
	Filename      string
	ContentType   string
	ContentID     string
	Data          []byte
	IsInline      bool
	S3Key         string // Stored S3 reference key
	DecryptionKey []byte // Symmetric decryption key (AES-256-GCM)
}

// MessageBuilder is a fluent builder for email messages.
type MessageBuilder struct {
	fromEmail               string
	fromName                string
	to                      []string
	subject                 string
	body                    string
	isHTML                  bool
	attachments             []Attachment
	store                   AttachmentStore // S3 store for deferred loading and auto-cleanup
	maxAttachmentSize      int64           // Limit for single attachment (default: 10MB)
	maxTotalAttachmentsSize int64           // Limit for all attachments combined (default: 25MB)
	err                     error           // Sticky error
}

// NewMessage creates a new fluent MessageBuilder.
func NewMessage() *MessageBuilder {
	return &MessageBuilder{
		maxAttachmentSize:      10 * 1024 * 1024, // 10MB
		maxTotalAttachmentsSize: 25 * 1024 * 1024, // 25MB
	}
}

// WithMaxAttachmentSize overrides the default 10MB limit for a single attachment.
// A value of 0 or less disables the limit check for individual attachments.
func (m *MessageBuilder) WithMaxAttachmentSize(size int64) *MessageBuilder {
	if m.err != nil {
		return m
	}
	m.maxAttachmentSize = size
	return m
}

// WithMaxTotalAttachmentsSize overrides the default 25MB limit for the combined size of all attachments.
// A value of 0 or less disables the limit check for total attachments.
func (m *MessageBuilder) WithMaxTotalAttachmentsSize(size int64) *MessageBuilder {
	if m.err != nil {
		return m
	}
	m.maxTotalAttachmentsSize = size
	return m
}

// validateLimits checks if a new attachment of the given size would exceed configured individual or total limits.
func (m *MessageBuilder) validateLimits(newSize int64) error {
	if m.maxAttachmentSize > 0 && newSize > m.maxAttachmentSize {
		return fmt.Errorf("attachment size %d bytes exceeds the maximum allowed individual limit of %d bytes", newSize, m.maxAttachmentSize)
	}

	var currentTotal int64
	for _, att := range m.attachments {
		currentTotal += int64(len(att.Data))
	}

	if m.maxTotalAttachmentsSize > 0 && (currentTotal+newSize) > m.maxTotalAttachmentsSize {
		return fmt.Errorf("total attachments size %d bytes exceeds the maximum allowed combined limit of %d bytes", currentTotal+newSize, m.maxTotalAttachmentsSize)
	}

	return nil
}

// WithStore sets the AttachmentStore for S3 queueing support.
func (m *MessageBuilder) WithStore(store AttachmentStore) *MessageBuilder {
	if m.err != nil {
		return m
	}
	m.store = store
	return m
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
	m.isHTML = false
	return m
}

// HTML sets the HTML formatted mail body payload.
func (m *MessageBuilder) HTML(htmlContent string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	m.body = htmlContent
	m.isHTML = true
	return m
}

// EmbedFile reads a local file and registers it as an inline CID attachment.
func (m *MessageBuilder) EmbedFile(filePath string, cidName string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	if filePath == "" {
		m.err = fmt.Errorf("file path cannot be empty")
		return m
	}
	if cidName == "" {
		m.err = fmt.Errorf("content ID cannot be empty")
		return m
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		m.err = fmt.Errorf("failed to read embed file: %w", err)
		return m
	}

	contentType := http.DetectContentType(data)
	filename := filepath.Base(filePath)

	return m.EmbedBytes(data, filename, contentType, cidName)
}

// EmbedBytes registers raw binary data as an inline CID attachment.
func (m *MessageBuilder) EmbedBytes(data []byte, filename string, contentType string, cidName string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	if len(data) == 0 {
		m.err = fmt.Errorf("attachment data cannot be empty")
		return m
	}
	if cidName == "" {
		m.err = fmt.Errorf("content ID cannot be empty")
		return m
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	optData, optFilename, optContentType := optimizeImage(data, filename, contentType)

	if err := m.validateLimits(int64(len(optData))); err != nil {
		m.err = err
		return m
	}

	m.attachments = append(m.attachments, Attachment{
		Filename:    optFilename,
		ContentType: optContentType,
		ContentID:   cidName,
		Data:        optData,
		IsInline:    true,
	})
	return m
}

// AttachFile reads a local file and registers it as a standard email attachment.
func (m *MessageBuilder) AttachFile(filePath string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	if filePath == "" {
		m.err = fmt.Errorf("file path cannot be empty")
		return m
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		m.err = fmt.Errorf("failed to read attachment file: %w", err)
		return m
	}

	contentType := http.DetectContentType(data)
	filename := filepath.Base(filePath)

	return m.AttachBytes(data, filename, contentType)
}

// AttachBytes registers raw binary data as a standard email attachment.
func (m *MessageBuilder) AttachBytes(data []byte, filename string, contentType string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	if len(data) == 0 {
		m.err = fmt.Errorf("attachment data cannot be empty")
		return m
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	optData, optFilename, optContentType := optimizeImage(data, filename, contentType)

	if err := m.validateLimits(int64(len(optData))); err != nil {
		m.err = err
		return m
	}

	m.attachments = append(m.attachments, Attachment{
		Filename:    optFilename,
		ContentType: optContentType,
		Data:        optData,
		IsInline:    false,
	})
	return m
}

// QueueAttachment encrypts the file data using AES-256 GCM, uploads it to S3/AttachmentStore,
// and registers the deferred S3 Key in the builder automatically.
func (m *MessageBuilder) QueueAttachment(store AttachmentStore, aesKey []byte, filename string, contentType string, data []byte, isInline bool, cid string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if store == nil {
		return "", fmt.Errorf("attachment store cannot be nil")
	}
	if len(aesKey) != 32 {
		return "", fmt.Errorf("aes key must be exactly 32 bytes for AES-256")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("data cannot be empty")
	}

	optData, optFilename, optContentType := optimizeImage(data, filename, contentType)

	if err := m.validateLimits(int64(len(optData))); err != nil {
		m.err = err
		return "", err
	}

	encrypted, err := EncryptAES(aesKey, optData)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	s3Key := fmt.Sprintf("mail/attachments/%d_%s", time.Now().UnixNano(), optFilename)
	err = store.UploadAttachment(s3Key, encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to upload encrypted attachment: %w", err)
	}

	m.AddQueuedAttachment(s3Key, aesKey, optFilename, optContentType, isInline, cid)
	return s3Key, nil
}

// AddQueuedAttachment registers a deferred encrypted S3/AttachmentStore file reference.
func (m *MessageBuilder) AddQueuedAttachment(s3Key string, aesKey []byte, filename string, contentType string, isInline bool, cid string) *MessageBuilder {
	if m.err != nil {
		return m
	}
	if s3Key == "" {
		m.err = fmt.Errorf("s3 key cannot be empty")
		return m
	}
	if len(aesKey) != 32 {
		m.err = fmt.Errorf("aes key must be exactly 32 bytes for AES-256")
		return m
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	m.attachments = append(m.attachments, Attachment{
		Filename:      filename,
		ContentType:   contentType,
		ContentID:     cid,
		IsInline:      isInline,
		S3Key:         s3Key,
		DecryptionKey: aesKey,
	})
	return m
}

// Error returns the accumulated error if any, supporting sticky error check.
func (m *MessageBuilder) Error() error {
	return m.err
}

// EncryptAES encrypts plaintext bytes using AES-256-GCM.
func EncryptAES(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptAES decrypts ciphertext bytes using AES-256-GCM.
func DecryptAES(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// base64Wrap wraps base64 encoded strings to 76 characters per line as per RFC 2045.
func base64Wrap(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var buf strings.Builder
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		buf.WriteString(encoded[i:end])
		buf.WriteString("\r\n")
	}
	return buf.String()
}

// Send delivers the message using the provided SMTP server configuration.
func (m *MessageBuilder) Send(config SMTPConfig) error {
	if m.err != nil {
		return m.err
	}
	if len(m.to) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	// 1. Resolve deferred S3/AttachmentStore attachments
	for i, att := range m.attachments {
		if att.S3Key != "" {
			if m.store == nil {
				return fmt.Errorf("cannot send mail: S3 queued attachment is present but no AttachmentStore was set")
			}
			encrypted, err := m.store.DownloadAttachment(att.S3Key)
			if err != nil {
				return fmt.Errorf("failed to download attachment from S3: %w", err)
			}
			decrypted, err := DecryptAES(att.DecryptionKey, encrypted)
			if err != nil {
				return fmt.Errorf("failed to decrypt attachment: %w", err)
			}
			m.attachments[i].Data = decrypted
		}
	}

	// 1b. Validate attachment size limits (both individual and cumulative) after downloading S3 attachments
	var totalSize int64
	for _, att := range m.attachments {
		size := int64(len(att.Data))
		if m.maxAttachmentSize > 0 && size > m.maxAttachmentSize {
			return fmt.Errorf("attachment %q size %d bytes exceeds the maximum allowed individual limit of %d bytes", att.Filename, size, m.maxAttachmentSize)
		}
		totalSize += size
	}
	if m.maxTotalAttachmentsSize > 0 && totalSize > m.maxTotalAttachmentsSize {
		return fmt.Errorf("total attachments size %d bytes exceeds the maximum allowed combined limit of %d bytes", totalSize, m.maxTotalAttachmentsSize)
	}

	fromEmail := config.From
	if m.fromEmail != "" {
		fromEmail = m.fromEmail
	}
	fromName := config.FromName
	if m.fromName != "" {
		fromName = m.fromName
	}

	// Prepare RFC 822 formatted message headers
	var headers []string
	if fromName != "" {
		headers = append(headers, fmt.Sprintf("From: %s <%s>", fromName, fromEmail))
	} else {
		headers = append(headers, fmt.Sprintf("From: %s", fromEmail))
	}
	headers = append(headers, fmt.Sprintf("To: %s", strings.Join(m.to, ", ")))
	headers = append(headers, fmt.Sprintf("Subject: %s", m.subject))
	headers = append(headers, "MIME-Version: 1.0")

	var msg []byte

	if m.isHTML || len(m.attachments) > 0 {
		// Use multipart related encoding for HTML + Attachments
		boundary := fmt.Sprintf("nativebpm_boundary_%d", time.Now().UnixNano())
		headers = append(headers, fmt.Sprintf("Content-Type: multipart/related; boundary=\"%s\"", boundary))
		headers = append(headers, "") // End of main headers

		var bodyBuf strings.Builder
		bodyBuf.WriteString(strings.Join(headers, "\r\n"))
		bodyBuf.WriteString("\r\n")

		// 1. Text/HTML part
		bodyBuf.WriteString("--" + boundary + "\r\n")
		if m.isHTML {
			bodyBuf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
		} else {
			bodyBuf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		}
		bodyBuf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		bodyBuf.WriteString(m.body)
		bodyBuf.WriteString("\r\n")

		// 2. Attachment parts (both inline embeds and standard attachments)
		for _, att := range m.attachments {
			bodyBuf.WriteString("--" + boundary + "\r\n")
			bodyBuf.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", att.ContentType, att.Filename))
			bodyBuf.WriteString("Content-Transfer-Encoding: base64\r\n")
			if att.IsInline {
				bodyBuf.WriteString(fmt.Sprintf("Content-ID: <%s>\r\n", att.ContentID))
				bodyBuf.WriteString(fmt.Sprintf("Content-Disposition: inline; filename=\"%s\"\r\n\r\n", att.Filename))
			} else {
				bodyBuf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", att.Filename))
			}
			bodyBuf.WriteString(base64Wrap(att.Data))
			bodyBuf.WriteString("\r\n")
		}

		// 3. End boundary
		bodyBuf.WriteString("--" + boundary + "--\r\n")
		msg = []byte(bodyBuf.String())
	} else {
		// Simple plain text message
		headers = append(headers, "Content-Type: text/plain; charset=\"utf-8\"")
		headers = append(headers, "")
		headers = append(headers, m.body)
		msg = []byte(strings.Join(headers, "\r\n"))
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	var auth smtp.Auth
	if config.Username != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	sendErr := func() error {
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
	}()

	if sendErr != nil {
		return sendErr
	}

	// 2. Clear out temp files in S3 immediately upon successful transmission
	if m.store != nil {
		for _, att := range m.attachments {
			if att.S3Key != "" {
				_ = m.store.DeleteAttachment(att.S3Key)
			}
		}
	}

	return nil
}

