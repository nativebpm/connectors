# Gotenberg & Telegram Integration Example in Temporal

Этот пример демонстрирует **нативную оркестрацию бизнес-процессов** с использованием Temporal и двух готовых коннекторов из монорепозитория:
1. **`gotenberg`** — для рендеринга и генерации PDF-документов из HTML.
2. **`telegram`** — для отправки сгенерированных отчетов пользователю.

Особенность примера в том, что он работает **напрямую**, без использования реляционных баз данных, промежуточных таблиц (Outbox) и CDC (Change Data Capture), полностью раскрывая возможности Temporal как распределенной среды выполнения.

---

## Требования

1. Запущенный сервер Temporal (например, через `docker compose` из папки `docker/`).
2. Запущенный инстанс Gotenberg (обычно доступен по адресу `http://localhost:3000`).
3. Токен Telegram-бота и ваш Telegram Chat ID.

---

## Настройка

Создайте или отредактируйте файл `temporal/temporal.env`, добавив туда параметры для Telegram и Gotenberg:

```env
# Параметры Temporal
TEMPORAL_HOST_PORT=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE=gotenberg-telegram-queue

# Параметры коннекторов
GOTENBERG_URL=http://localhost:3000
TELEGRAM_BOT_TOKEN=123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ
TELEGRAM_CHAT_ID=987654321
```

---

## Запуск

Из корневого каталога `temporal/` выполните:

1. **Запуск воркера**:
   ```bash
   make run-worker-gotenberg-telegram
   ```
   *(Или напрямую: `go run examples/gotenberg-telegram/worker/main.go`)*

2. **Запуск стартера (триггер бизнес-процесса)**:
   ```bash
   make run-starter-gotenberg-telegram
   ```
   *(Или напрямую: `go run examples/gotenberg-telegram/starter/main.go`)*
