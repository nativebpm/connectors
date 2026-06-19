# Go-клиент для Wasmee (Wasmee Go Client Connector)

Этот пакет предоставляет клиент на Go для взаимодействия с Rust-демоном изолированного выполнения WebAssembly Wasmee.

## Генерация кода Protobuf

Код Go Protobuf в директории `pb/wasmee.pb.go` генерируется из центрального файла схемы [wasmee.proto](file:///Users/user/github.com/nativebpm/wasmee/wasmee.proto), расположенного в репозитории `wasmee`.

### Предварительные требования

Убедитесь, что на вашей системе установлены `protoc` и плагин `protoc-gen-go`:
```bash
# Проверка наличия protoc
protoc --version

# Установка плагина protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

### Перегенерация кода

Чтобы перегенерировать Go-коллаут структуры, выполните команду `go generate` из этой директории:
```bash
go generate
```

Или вызовите `protoc` вручную:
```bash
protoc --proto_path=../../wasmee --go_out=pb --go_opt=paths=source_relative ../../wasmee/wasmee.proto
```
