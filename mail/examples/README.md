# Mail Connector Examples

This subdirectory contains example usage scenarios for the NativeBPM `connectors/mail` package.

## 1. Yandex SMTP Test Verification

The [main.go](file:///Users/user/github.com/nativebpm/connectors/mail/examples/main.go) script demonstrates using Yandex SMTP configuration settings (port 465 with SSL/TLS) and the fluent API to send a test email to `supabase@internet.ru`.

### Running the Example

Run the script directly from the workspace root:

```bash
go run ./connectors/mail/examples/main.go
```
