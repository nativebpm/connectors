package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"

	"github.com/nativebpm/connectors/mail"
	"github.com/nativebpm/connectors/mail/mjml"
)

func main() {
	// Read SMTP configuration from environment variables
	smtpHost := getEnv("SMTP_HOST", "smtp.yandex.ru")
	smtpPort := getEnvInt("SMTP_PORT", 465)
	smtpUser := getEnv("SMTP_USER", "")
	smtpPass := getEnv("SMTP_PASSWORD", "")
	fromEmail := getEnv("SMTP_FROM", "")
	fromName := getEnv("SMTP_FROM_NAME", "PONEN")
	useSSL := getEnvBool("SMTP_USE_SSL", true)

	recipient := getEnv("SMTP_TO", "supabase@internet.ru")

	if smtpUser == "" || smtpPass == "" || fromEmail == "" {
		fmt.Println("Error: SMTP_USER, SMTP_PASSWORD, and SMTP_FROM environment variables must be configured to run this example.")
		fmt.Println("Please run:")
		fmt.Println("  export SMTP_USER=\"your-email@yandex.ru\"")
		fmt.Println("  export SMTP_PASSWORD=\"your-app-password\"")
		fmt.Println("  export SMTP_FROM=\"your-email@yandex.ru\"")
		fmt.Println("  export SMTP_TO=\"recipient@example.com\"")
		os.Exit(1)
	}

	config := mail.SMTPConfig{
		Host:     smtpHost,
		Port:     smtpPort,
		Username: smtpUser,
		Password: smtpPass,
		From:     fromEmail,
		FromName: fromName,
		UseSSL:   useSSL,
	}

	// Example 1: Standard SMTP plain-text email
	fmt.Printf("Sending plain-text email from %s to %s via Yandex SMTP...\n", config.From, recipient)
	err := mail.NewMessage().
		From(config.From, config.FromName).
		To(recipient).
		Subject("Yandex SMTP Plain-Text Test").
		Body("Hello!\n\nThis is a verification email demonstrating successful integration of the Yandex SMTP mail connector in NativeBPM.").
		Send(config)

	if err != nil {
		log.Fatalf("Failed to send plain-text email: %v", err)
	}
	fmt.Println("SUCCESS: Plain-text email sent.")

	// Example 2: Compilation and delivery of a styled MJML responsive layout with embedded banner
	fmt.Printf("Compiling MJML template and sending responsive email with embedded banner to %s...\n", recipient)

	// Generate a beautiful dark-blue banner dynamically as PNG bytes
	bannerImg := image.NewRGBA(image.Rect(0, 0, 600, 150))
	// Fill with dark blue background
	darkBlue := color.RGBA{5, 6, 9, 255}
	draw.Draw(bannerImg, bannerImg.Bounds(), &image.Uniform{darkBlue}, image.Point{}, draw.Src)
	// Draw a cyan accent line at the bottom
	cyan := color.RGBA{0, 240, 255, 255}
	draw.Draw(bannerImg, image.Rect(0, 145, 600, 150), &image.Uniform{cyan}, image.Point{}, draw.Src)

	var bannerBuf bytes.Buffer
	if err := png.Encode(&bannerBuf, bannerImg); err != nil {
		log.Fatalf("Failed to encode banner image: %v", err)
	}
	bannerBytes := bannerBuf.Bytes()

	// MJML Template containing a header banner with overlaid text and a multi-column body
	mjmlTemplate := `
<mjml>
  <mj-head>
    <mj-title>PONEN Campaign</mj-title>
    <mj-attributes>
      <mj-all font-family="Arial, Helvetica, sans-serif" />
    </mj-attributes>
  </mj-head>
  <mj-body background-color="#f3f4f6">
    <!-- Section with a background banner image referenced by CID -->
    <mj-section background-url="cid:banner" background-size="cover" background-repeat="no-repeat" padding="40px 20px" background-color="#050609">
      <mj-column width="100%">
        <mj-text align="center" color="#ffffff" font-size="28px" font-weight="bold">
          PONEN PLATFORM
        </mj-text>
        <mj-text align="center" color="#00f0ff" font-size="16px" font-weight="500">
          Reliable Enterprise Workflow Orchestration
        </mj-text>
      </mj-column>
    </mj-section>

    <!-- Multi-column text section -->
    <mj-section background-color="#ffffff" padding="40px 20px">
      <mj-column width="48%">
        <mj-text font-size="20px" font-weight="bold" color="#111827" padding-bottom="10px">
          High Performance
        </mj-text>
        <mj-text font-size="14px" color="#4b5563" line-height="22px">
          Our cloud-native execution engine runs compiled Go/TinyGo WebAssembly modules in isolated sandboxes with extremely low latency and zero memory fragmentation.
        </mj-text>
      </mj-column>
      <mj-column width="4%">
        <!-- Spacing column -->
      </mj-column>
      <mj-column width="48%">
        <mj-text font-size="20px" font-weight="bold" color="#111827" padding-bottom="10px">
          Durable Sagas
        </mj-text>
        <mj-text font-size="14px" color="#4b5563" line-height="22px">
          With transparent, page-based memory snapshots, NativeBPM guarantees reliable recovery and execution of complex, long-running BPMN 2.0 and DMN processes.
        </mj-text>
      </mj-column>
    </mj-section>

    <!-- Footer -->
    <mj-section background-color="#f3f4f6" padding-bottom="20px" padding-top="20px">
      <mj-column width="100%">
        <mj-text align="center" font-size="12px" color="#9ca3af">
          © 2026 PONEN Platform. All rights reserved.
        </mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`

	ctx := context.Background()
	// Use new Fluent API to compile the template
	htmlBody, err := mjml.Compile(ctx, mjmlTemplate).WithMinify(true).Run()
	if err != nil {
		log.Fatalf("Failed to compile MJML template: %v", err)
	}

	// Build and send HTML mail embedding the banner as an inline CID attachment
	err = mail.NewMessage().
		From(config.From, config.FromName).
		To(recipient).
		Subject("PONEN Responsive Email with Embedded Banner").
		HTML(htmlBody).
		EmbedBytes(bannerBytes, "banner.png", "image/png", "banner").
		Send(config)

	if err != nil {
		log.Fatalf("Failed to send responsive email: %v", err)
	}
	fmt.Println("SUCCESS: Responsive email compiled and sent successfully.")
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if valStr, exists := os.LookupEnv(key); exists {
		var val int
		if _, err := fmt.Sscanf(valStr, "%d", &val); err == nil {
			return val
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if valStr, exists := os.LookupEnv(key); exists {
		return valStr == "true" || valStr == "1" || valStr == "yes"
	}
	return fallback
}
