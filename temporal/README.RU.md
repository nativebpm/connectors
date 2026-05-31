# Коннектор Temporal.io на Go

Высокопроизводительная обертка для Temporal.io Go SDK, предоставляющая упрощенный интерфейс для клиента и воркера, структурированную конфигурацию на основе переменных окружения и готовую Docker-инфраструктуру.

---

## 1. Структура директории

```
temporal/
├── config.go         # Конфигурация подключения и TLS
├── client.go         # Обертка над клиентом (запуск workflow, сигналы, запросы)
├── worker.go         # Обертка над воркером (регистрация workflow и activity)
├── docker/           # Конфигурация Docker Compose с PostgreSQL
└── examples/
    ├── helloworld/   # Простейший Workflow и Activity
    ├── signal/       # Использование Signals и Queries для интерактива
    ├── saga/         # Реализация паттерна Saga с компенсациями LIFO
    └── loadtest/     # Инструмент для конкурентного нагрузочного тестирования
```

---

## 2. Локальный запуск инфраструктуры (Docker)

В модуль включен преднастроенный файл Docker Compose, запускающий **Temporal Server v1.24.2** с СУБД **PostgreSQL 14**.

Для запуска инфраструктуры:
```bash
# С помощью Makefile из папки temporal/:
make infra-up

# Или вручную:
cd temporal/docker
docker compose up -d
```
После запуска:
- **Панель управления Temporal (Web UI)** доступна по адресу: [http://localhost:8233](http://localhost:8233)
- **gRPC эндпоинт Temporal**: `localhost:7233`

---

## 3. Нагрузочное тестирование

Для оценки производительности движка под конкурентной нагрузкой создан инструмент в `examples/loadtest`.

### Запуск нагрузочного теста
1. Убедитесь, что Docker-окружение Temporal запущено.
2. Выполните команду для запуска теста:
   ```bash
   # С помощью Makefile:
   make run-loadtest

   # Или вручную:
   LOAD_CONCURRENCY=50 LOAD_PROCESSES_COUNT=1000 go run temporal/examples/loadtest/main.go
   ```

3. Утилита выводит промежуточный статус прогресса и итоговую статистику: **перцентили задержек p50, p90, p95, p99**, **RPS** (запусков в секунду) и **TPS** (пропускную способность задач).
