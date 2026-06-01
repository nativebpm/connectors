package main

import (
	"log"
	"net/http"
	"os"

	"github.com/nativebpm/connectors/temporal"
	"go.temporal.io/sdk/converter"
)

func main() {
	log.Println("Starting Temporal Codec Server...")

	// Загружаем общую конфигурацию
	cfg := temporal.LoadFromEnv()

	if len(cfg.EncryptionKey) == 0 {
		log.Fatal("TEMPORAL_ENCRYPTION_KEY environment variable is required but not set")
	}

	// Создаем шифрующий кодек
	codec, err := temporal.NewCryptCodec(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize CryptCodec: %v", err)
	}

	// Создаем HTTP обработчик для кодека
	handler := converter.NewPayloadCodecHTTPHandler(codec)

	// Добавляем CORS-заголовки для доступа из браузера (админки Temporal Web UI)
	corsHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*") // В prod рекомендуется ограничить доменом админки
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Namespace, Authorization")
			
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	port := os.Getenv("TEMPORAL_CODEC_SERVER_PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Codec Server is listening on port %s...", port)
	if err := http.ListenAndServe(":"+port, corsHandler(handler)); err != nil {
		log.Fatalf("Codec Server HTTP listener failed: %v", err)
	}
}
