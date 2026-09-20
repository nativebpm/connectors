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
	"strconv"

	"github.com/nativebpm/connectors/mail"
	"github.com/nativebpm/connectors/mail/mjml"
)

// Helper to generate a banner
func generateBannerBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 600, 150))
	darkBlue := color.RGBA{5, 6, 9, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{darkBlue}, image.Point{}, draw.Src)
	cyan := color.RGBA{0, 240, 255, 255}
	draw.Draw(img, image.Rect(0, 145, 600, 150), &image.Uniform{cyan}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func main() {
	// Read SMTP configuration from environment variables
	smtpHost := getEnv("SMTP_HOST", "smtp.yandex.ru")
	smtpPort := getEnvInt("SMTP_PORT", 465)
	smtpUser := getEnv("SMTP_USER", "")
	smtpPass := getEnv("SMTP_PASSWORD", "")
	fromEmail := getEnv("SMTP_FROM", "")
	fromName := getEnv("SMTP_FROM_NAME", "PONEN Debugger")
	useSSL := getEnvBool("SMTP_USE_SSL", true)
	recipient := getEnv("SMTP_TO", "")

	if smtpUser == "" || smtpPass == "" || fromEmail == "" || recipient == "" {
		fmt.Println("Error: SMTP_USER, SMTP_PASSWORD, SMTP_FROM, and SMTP_TO environment variables must be set.")
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

	ctx := context.Background()
	bannerBytes := generateBannerBytes()

	// Define the 4 testing scenarios from simple to complex
	scenarios := []struct {
		Name        string
		Subject     string
		MJML        string
		HasBanner   bool
		IsBgBanner  bool
	}{
		{
			Name:    "Scenario 1: Minimal HTML (Simple paragraph)",
			Subject: "PONEN Email Debug - Scenario 1 (Minimal HTML)",
			MJML: `
<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text font-size="16px" color="#333333">
          This is Scenario 1: A minimal, simple HTML layout containing only one column of text.
        </mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`,
		},
		{
			Name:    "Scenario 2: Multi-column Table Layout (No Images)",
			Subject: "PONEN Email Debug - Scenario 2 (Multi-column)",
			MJML: `
<mjml>
  <mj-body background-color="#f3f4f6">
    <mj-section background-color="#ffffff" padding="20px">
      <mj-column width="48%">
        <mj-text font-size="16px" font-weight="bold" color="#111827">Column A</mj-text>
        <mj-text font-size="14px" color="#4b5563">This is the left text column in a double-column layout.</mj-text>
      </mj-column>
      <mj-column width="4%"></mj-column>
      <mj-column width="48%">
        <mj-text font-size="16px" font-weight="bold" color="#111827">Column B</mj-text>
        <mj-text font-size="14px" color="#4b5563">This is the right text column in a double-column layout.</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`,
		},
		{
			Name:      "Scenario 3: Multi-column + Top Banner Inline Image (CID)",
			Subject:   "PONEN Email Debug - Scenario 3 (Banner Inline Image)",
			HasBanner: true,
			MJML: `
<mjml>
  <mj-body background-color="#f3f4f6">
    <mj-section background-color="#050609" padding="0">
      <mj-column width="100%">
        <!-- Banner embedded as a normal centered image -->
        <mj-image src="cid:banner" alt="PONEN Banner" width="600px" padding="0" />
      </mj-column>
    </mj-section>
    <mj-section background-color="#ffffff" padding="20px">
      <mj-column width="48%">
        <mj-text font-size="16px" font-weight="bold" color="#111827">Column A</mj-text>
        <mj-text font-size="14px" color="#4b5563">This layout includes a top banner as a normal inline image.</mj-text>
      </mj-column>
      <mj-column width="4%"></mj-column>
      <mj-column width="48%">
        <mj-text font-size="16px" font-weight="bold" color="#111827">Column B</mj-text>
        <mj-text font-size="14px" color="#4b5563">The banner is embedded using Content-ID (cid:banner) attachment.</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`,
		},
		{
			Name:       "Scenario 4: Multi-column + Banner Background Image (CID)",
			Subject:    "PONEN Email Debug - Scenario 4 (Banner Background)",
			HasBanner:  true,
			IsBgBanner: true,
			MJML: `
<mjml>
  <mj-body background-color="#f3f4f6">
    <!-- Banner embedded as a background image with text overlaid -->
    <mj-section background-url="cid:banner" background-size="cover" background-repeat="no-repeat" padding="40px 20px" background-color="#050609">
      <mj-column width="100%">
        <mj-text align="center" color="#ffffff" font-size="28px" font-weight="bold">
          OVERLAID TEXT
        </mj-text>
        <mj-text align="center" color="#00f0ff" font-size="16px">
          This text is positioned on top of the background banner image
        </mj-text>
      </mj-column>
    </mj-section>
    <mj-section background-color="#ffffff" padding="20px">
      <mj-column width="48%">
        <mj-text font-size="16px" font-weight="bold" color="#111827">Column A</mj-text>
        <mj-text font-size="14px" color="#4b5563">This layout tests background image support in email clients.</mj-text>
      </mj-column>
      <mj-column width="4%"></mj-column>
      <mj-column width="48%">
        <mj-text font-size="16px" font-weight="bold" color="#111827">Column B</mj-text>
        <mj-text font-size="14px" color="#4b5563">Outlook and Gmail should render the background and text layers correctly.</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`,
		},
	}

	for _, sc := range scenarios {
		fmt.Printf("\n--- Executing %s ---\n", sc.Name)

		// 1. Compile MJML
		html, err := mjml.Compile(ctx, sc.MJML).WithMinify(true).Run()
		if err != nil {
			log.Fatalf("  [ERROR] MJML compilation failed: %v", err)
		}

		// 2. Prepare Message
		msg := mail.NewMessage().
			From(config.From, config.FromName).
			To(recipient).
			Subject(sc.Subject).
			HTML(html)

		if sc.HasBanner {
			msg.EmbedBytes(bannerBytes, "banner.png", "image/png", "banner")
		}

		// 3. Size Inspection
		htmlSize := len(html)
		bannerPartSize := 0
		if sc.HasBanner {
			bannerPartSize = (len(bannerBytes) * 4 / 3) + 200 // approximation including headers
		}
		totalEstSize := htmlSize + bannerPartSize
		fmt.Printf("  Compiled HTML Size: %.2f KB\n", float64(htmlSize)/1024)
		if sc.HasBanner {
			fmt.Printf("  Embedded Banner Size (Base64): %.2f KB\n", float64(bannerPartSize)/1024)
		}
		fmt.Printf("  Estimated Total Message Size: %.2f KB\n", float64(totalEstSize)/1024)

		if totalEstSize > 102*1024 {
			fmt.Println("  [WARNING] Total size exceeds 102 KB! Gmail might truncate this email.")
		}

		// 4. Send Message
		fmt.Println("  Sending email...")
		if err := msg.Send(config); err != nil {
			fmt.Printf("  [FAILED] SMTP Send failed: %v\n", err)
		} else {
			fmt.Println("  [SUCCESS] Sent successfully.")
		}
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if valStr, exists := os.LookupEnv(key); exists {
		if val, err := strconv.Atoi(valStr); err == nil {
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
