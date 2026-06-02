# Sequin Outbox CDC to Temporal Example

Этот пример демонстрирует реализацию паттерна **Transactional Outbox** с использованием **Sequin (CDC)** и **Temporal** на языке Go.

## Архитектура

При удалении пользователя из базы данных (`DELETE` в таблице `users`), Sequin захватывает изменения из Postgres WAL и надежно отправляет POST вебхук на HTTP-сервер. HTTP-сервер принимает вебхук и запускает воркфлоу `DeleteUserWorkflow` в Temporal, который гарантирует полное удаление пользователя во внешних системах и отправку письма.

```
[Удаление пользователя из БД] -> [Postgres WAL] -> [Sequin CDC] -> [HTTP-вебхук /delete-user] -> [Temporal Workflow] -> [Worker]
```

## Структура проекта

- `workflow.go` — Определение Temporal Workflow `DeleteUserWorkflow`.
- `activities.go` — Шаги воркфлоу (очистка внешних систем, отправка писем).
- `handler.go` — HTTP-обработчик для вебхуков от Sequin.
- `server/main.go` — Запуск HTTP-сервера.
- `worker/main.go` — Запуск Temporal Worker.
- `integration_test.go` — Тесты воркфлоу и HTTP-обработчика вебхуков.

## Локальный запуск

### 1. Запуск Temporal Server
Запустите локальный сервер Temporal:
```bash
temporal server start-dev
```

### 2. Настройка Postgres и Sequin
1. Создайте таблицу `users` и включите репликацию:
   ```sql
   create table users (
     id uuid primary key default gen_random_uuid (),
     first_name text not null,
     last_name text not null,
     email text not null unique,
     created_at timestamptz default now(),
     updated_at timestamptz default now()
   );

   alter table users replica identity full;
   ```
2. Подключите вашу базу данных к Sequin.
3. Создайте в Sequin новый **Webhook Sink**:
   - **Source Table**: `users`
   - **Filters**: Только `Delete` (Insert и Update выключить)
   - **Endpoint URL**: `http://localhost:3333/delete-user`

### 3. Запуск Worker и HTTP-сервера
В каталоге `temporal` выполните:
```bash
# Запуск воркера Temporal
go run examples/sequin-outbox/worker/main.go

# В другом терминале запустите HTTP-сервер вебхуков
go run examples/sequin-outbox/server/main.go
```

### 4. Тестирование вручную
Удалите пользователя из Postgres:
```sql
delete from users where email = 'user@example.com';
```
Вы увидите, что в консоли HTTP-сервера зафиксирован входящий вебхук от Sequin, запущен воркфлоу в Temporal, а воркер успешно выполнил активности по удалению данных и отправке подтверждения на почту.

## Запуск автоматических тестов

Для запуска тестов примера выполните:
```bash
go test -v ./examples/sequin-outbox/...
```
