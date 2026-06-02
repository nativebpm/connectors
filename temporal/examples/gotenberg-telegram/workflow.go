package gotenbergtelegram

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DocumentGenerationWorkflow orchestrates HTML-to-PDF conversion and Telegram upload
func DocumentGenerationWorkflow(ctx workflow.Context, htmlContent string, chatID int64, filename string) error {
	// Configure options for activities execution
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		// Standard exponential backoff policy for automatic retries
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var activities *Activities
	var pdfBytes []byte

	// Step 1: Convert HTML to PDF using Gotenberg Chromium
	err := workflow.ExecuteActivity(ctx, activities.ConvertHTMLToPDF, htmlContent).Get(ctx, &pdfBytes)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to convert HTML to PDF", "error", err)
		return err
	}

	// Step 2: Upload generated PDF bytes as a document to Telegram
	err = workflow.ExecuteActivity(ctx, activities.SendTelegramDocument, chatID, pdfBytes, filename).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send PDF document via Telegram", "error", err)
		return err
	}

	workflow.GetLogger(ctx).Info("DocumentGenerationWorkflow completed successfully")
	return nil
}
