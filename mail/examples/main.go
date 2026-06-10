package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nativebpm/connectors/mail"
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

	fmt.Printf("Sending test email from %s to %s via Yandex SMTP...\n", config.From, recipient)

	// Create and send message using the mail package's fluent builder API
	err := mail.NewMessage().
		From(config.From, config.FromName).
		To(recipient).
		Subject("Yandex SMTP Connector Test Verification").
		Body("Hello!\n\nThis is a verification email demonstrating successful integration of the Yandex SMTP mail connector in NativeBPM.").
		Send(config)

	if err != nil {
		log.Fatalf("Failed to send test email: %v", err)
	}

	fmt.Println("SUCCESS: Test email sent successfully to", recipient)
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
