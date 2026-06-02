package sequinoutbox

import (
	"context"
	"log"
	"time"
)

// CleanUpExternalSystems simulates removing user data from external storage (e.g. S3) and third-party APIs.
func CleanUpExternalSystems(ctx context.Context, userID string) error {
	log.Printf("[Activity] Starting external cleanup for user: %s", userID)

	// Simulate API call delay
	select {
	case <-time.After(1 * time.Second):
		log.Printf("[Activity] Successfully cleaned up external systems for user: %s", userID)
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// SendDeletionConfirmation simulates sending a confirmation email to the deleted user.
func SendDeletionConfirmation(ctx context.Context, email string) error {
	log.Printf("[Activity] Sending deletion confirmation email to: %s", email)

	// Simulate SMTP delay
	select {
	case <-time.After(500 * time.Millisecond):
		log.Printf("[Activity] Confirmation email sent successfully to: %s", email)
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
