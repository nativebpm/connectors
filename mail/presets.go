package mail

// ProviderPreset represents the SMTP configurations and size limits for popular mail providers.
type ProviderPreset struct {
	Host                    string
	Port                    int
	UseSSL                  bool
	MaxAttachmentSize       int64
	MaxTotalAttachmentsSize int64
	MaxHTMLSize             int64 // Maximum HTML body size to prevent Gmail email clipping (Gmail clips at 102KB)
}

// SMTPConfig generates an SMTPConfig using the preset details and credentials.
func (p ProviderPreset) SMTPConfig(username, password string) SMTPConfig {
	return SMTPConfig{
		Host:     p.Host,
		Port:     p.Port,
		UseSSL:   p.UseSSL,
		Username: username,
		Password: password,
		From:     username,
	}
}

var (
	// YandexPreset defines SMTP configuration and size limits for Yandex Mail.
	// Limits: Max message size is 30MB (including MIME encoding). Safe total attachments size is 22MB.
	// Yandex does not strictly clip HTML body at 102KB, but keeping it under 102KB is recommended.
	YandexPreset = ProviderPreset{
		Host:                    "smtp.yandex.ru",
		Port:                    465,
		UseSSL:                  true,
		MaxAttachmentSize:       22 * 1024 * 1024,
		MaxTotalAttachmentsSize: 22 * 1024 * 1024,
		MaxHTMLSize:             102 * 1024, // 102KB standard recommendation
	}

	// GmailPreset defines SMTP configuration and size limits for Google Gmail.
	// Limits: Max message size is 25MB. Safe total attachments size is 18MB.
	// Gmail strictly clips HTML body exceeding 102KB.
	GmailPreset = ProviderPreset{
		Host:                    "smtp.gmail.com",
		Port:                    587,
		UseSSL:                  false,
		MaxAttachmentSize:       18 * 1024 * 1024,
		MaxTotalAttachmentsSize: 18 * 1024 * 1024,
		MaxHTMLSize:             102 * 1024, // 102KB limit
	}

	// MailRuPreset defines SMTP configuration and size limits for Mail.ru.
	// Limits: Max message size is 25MB. Safe total attachments size is 18MB.
	MailRuPreset = ProviderPreset{
		Host:                    "smtp.mail.ru",
		Port:                    465,
		UseSSL:                  true,
		MaxAttachmentSize:       18 * 1024 * 1024,
		MaxTotalAttachmentsSize: 18 * 1024 * 1024,
		MaxHTMLSize:             102 * 1024, // 102KB standard recommendation
	}

	// OutlookPreset defines SMTP configuration and size limits for Outlook/Office365.
	// Limits: Max message size is 25MB. Safe total attachments size is 18MB.
	OutlookPreset = ProviderPreset{
		Host:                    "smtp.office365.com",
		Port:                    587,
		UseSSL:                  false,
		MaxAttachmentSize:       18 * 1024 * 1024,
		MaxTotalAttachmentsSize: 18 * 1024 * 1024,
		MaxHTMLSize:             102 * 1024, // 102KB standard recommendation
	}
)

// WithPreset applies the provider's standard attachment and total size limits to the builder.
func (m *MessageBuilder) WithPreset(preset ProviderPreset) *MessageBuilder {
	if m.err != nil {
		return m
	}
	m.maxAttachmentSize = preset.MaxAttachmentSize
	m.maxTotalAttachmentsSize = preset.MaxTotalAttachmentsSize
	m.maxHTMLSize = preset.MaxHTMLSize
	return m
}
