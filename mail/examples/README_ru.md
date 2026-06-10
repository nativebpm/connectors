# Примеры использования Mail коннектора

Этот каталог содержит примеры использования пакета `connectors/mail`.

## 1. Проверка отправки через Yandex SMTP

Скрипт [main.go](file:///Users/user/github.com/nativebpm/connectors/mail/examples/main.go) демонстрирует отправку тестового письма на адрес `supabase@internet.ru` с использованием настроек Yandex SMTP (порт 465 с шифрованием SSL/TLS) и Fluent API.

### Запуск примера

Запустите пример из корневой директории воркспейса:

```bash
go run ./connectors/mail/examples/main.go
```
