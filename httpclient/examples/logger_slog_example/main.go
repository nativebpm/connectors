package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nativebpm/connectors/httpclient"
	"github.com/nativebpm/connectors/httpclient/examples/logger_slog_example/middleware"
)

func main() {
	client, err := httpclient.NewClient(http.Client{Timeout: 10 * time.Second}, "https://httpbin.org")
	if err != nil {
		log.Fatal(err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	client.Use(middleware.LoggingMiddleware(logger))

	resp, err := client.GET(context.Background(), "/get").Send()
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
}
