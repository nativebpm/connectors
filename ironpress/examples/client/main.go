package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nativebpm/connectors/ironpress"
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "Address of the ironpress server")
	outFile := flag.String("out", "output.pdf", "Path to save the generated PDF")
	flag.Parse()

	// Initialize ironpress client
	client, err := ironpress.NewClient(nil, *serverURL)
	if err != nil {
		log.Fatalf("Failed to initialize ironpress client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	htmlContent := `
<!DOCTYPE html>
<html>
<head>
<style>
  body {
    font-family: Arial, sans-serif;
    color: #333;
    padding: 20px;
  }
  .header {
    border-bottom: 2px solid #5d5d5d;
    padding-bottom: 10px;
    margin-bottom: 20px;
  }
  .title {
    color: #4A90E2;
    margin: 0;
  }
  .content {
    line-height: 1.6;
  }
  .footer {
    margin-top: 50px;
    font-size: 12px;
    color: #777;
    text-align: center;
  }
</style>
</head>
<body>
  <div class="header">
    <h1 class="title">NativeBPM Ironpress PDF Generation</h1>
  </div>
  <div class="content">
    <p>This is a PDF document generated dynamically using the <strong>ironpress</strong> pure Rust layout engine via a Go connector client.</p>
    <p>Because ironpress is a lightweight library with zero external dependencies (unlike headless Chrome or LibreOffice), it executes instantly and consumes very few system resources.</p>
    
    <h3>Key Features Demo:</h3>
    <ul>
      <li>Fast compile-free rendering of HTML/CSS</li>
      <li>Compact resulting binary sizes</li>
      <li>Simple deployment model</li>
    </ul>
  </div>
  <div class="footer">
    Generated at ` + time.Now().Format(time.RFC1123) + `
  </div>
</body>
</html>
`

	log.Printf("Sending HTML to PDF conversion request to %s...", *serverURL)
	start := time.Now()

	// Use Fluent API to build and send the request
	pdfBytes, err := client.Convert().
		HTML(htmlContent).
		PageSize("a4").
		Landscape(false).
		Margin(15).
		Header("NativeBPM Reports").
		Footer("Page {page} of {pages}").
		Timeout(5 * time.Second).
		Do(ctx)

	if err != nil {
		log.Fatalf("PDF conversion failed: %v", err)
	}

	log.Printf("Successfully generated PDF in %v (%d bytes)", time.Since(start), len(pdfBytes))

	// Write generated PDF to disk
	if err := os.WriteFile(*outFile, pdfBytes, 0644); err != nil {
		log.Fatalf("Failed to save output PDF file: %v", err)
	}

	fmt.Printf("PDF successfully written to %s\n", *outFile)
}
