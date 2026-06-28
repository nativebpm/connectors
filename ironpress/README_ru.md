# Go-коннектор для Ironpress

Этот пакет предоставляет Go-коннектор (клиентский SDK + HTTP-сервер обертка) для конвертера документов `ironpress` PDF (https://github.com/gastongouron/ironpress).

`ironpress` — это конвертер HTML/Markdown в PDF, написанный на чистом Rust. Он использует собственный встроенный движок разметки и не требует headless Chrome или каких-либо внешних системных зависимостей.

## Предварительные требования

Для работы этого коннектора необходимо установить утилиту `ironpress` CLI на хост-машине, где будет запущен HTTP-сервер.

```bash
cargo install ironpress
```

Убедитесь, что скомпилированный бинарный файл доступен в вашей переменной окружения `PATH` (обычно находится в `~/.cargo/bin/`).

## Структура проекта

- `client.go`: Клиентский Go SDK с Fluent API.
- `server.go`: Легковесный HTTP-сервер, оборачивающий `ironpress` CLI с ограничением параллелизма (семафор) и плавным завершением работы (graceful shutdown).
- `examples/server/`: Точка входа для запуска HTTP-сервера.
- `examples/client/`: Пример создания PDF-документа из HTML с помощью клиентского SDK.
- `examples/k6/`: Скрипт для нагрузочного тестирования с помощью `k6`.

## Запуск HTTP-сервера

Запуск сервера обертки:

```bash
go run examples/server/main.go --addr :8080
```

Доступные флаги:
- `--addr`: Сетевой адрес для прослушивания (по умолчанию `:8080`).
- `--bin`: Абсолютный путь к бинарному файлу `ironpress` (если пустой, определяется автоматически в PATH).
- `--workers`: Максимальное количество одновременно выполняемых процессов конвертации (по умолчанию равно числу ядер CPU).

## Использование Go Client SDK

Пример генерации PDF-документа "на лету":

```go
package main

import (
	"context"
	"os"
	"time"
	"github.com/nativebpm/connectors/ironpress"
)

func main() {
	client, err := ironpress.NewClient(nil, "http://localhost:8080")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pdfBytes, err := client.Convert().
		HTML("<h1>Привет, NativeBPM!</h1><p>Сгенерировано с помощью ironpress.</p>").
		PageSize("a4").
		Landscape(false).
		Margin(10).
		Header("Мой заголовок").
		Footer("Страница {page} из {pages}").
		Do(ctx)

	if err != nil {
		panic(err)
	}

	err = os.WriteFile("output.pdf", pdfBytes, 0644)
	if err != nil {
		panic(err)
	}
}
```

## Запуск Unit-тестов

Запуск тестов локально:

```bash
go test -v -race ./...
```

## Нагрузочное тестирование с k6

Убедитесь, что у вас установлен `k6`. Запустите сервер в одном окне терминала:

```bash
go run examples/server/main.go --addr :8080
```

В другом окне запустите нагрузочный тест:

```bash
k6 run examples/k6/load_test.js
```
