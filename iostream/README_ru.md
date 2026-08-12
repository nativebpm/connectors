# iostream: Потоковый Fluent-помощник с нулевой буферизацией памяти для Go

Пакет `iostream` (`github.com/nativebpm/connectors/iostream`) скрывает внутреннюю сложность `io.Pipe()`, управления горутинами записи, проброса ошибок `CloseWithError` и интеграции с HTTP/Storage в лаконичный Fluent API.

## Возможности

- **Полная инкапсуляция `io.Pipe`**: Автоматическое управление горутинами, закрытием пайпа и обработкой ошибок.
- **Fluent Stream Builder (`StreamBuilder`)**: Цепочка вызовов для потоковой сериализации JSON, кастомных `WriterFunc` и HTTP-запросов.
- **Нулевая выделяемая буферизация в RAM**: Потоковая передача данных напрямую от кодировщика в сокет без промежуточного создания больших гигантских `[]byte` массивов.

---

## Пример использования

```go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/nativebpm/connectors/iostream"
)

type ProcessVariables struct {
	ProcessInstanceID string         `json:"process_instance_id"`
	Variables         map[string]any `json:"variables"`
}

func main() {
	payload := ProcessVariables{
		ProcessInstanceID: "proc-1001",
		Variables:         map[string]any{"status": "APPROVED", "score": 98.5},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Однострочный потоковый HTTP-запрос через Fluent API
	resp, err := iostream.NewStream().
		WithJSONPayload(payload).
		ToURL(http.MethodPost, "https://api.nativebpm.cloud/v1/execution/variables").
		WithHeader("Authorization", "Bearer glpat-token-secret").
		ExecuteHTTPRequest(ctx)

	if err != nil {
		log.Fatalf("Ошибка потокового запроса: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Статус ответа: %d", resp.StatusCode)
}
```
