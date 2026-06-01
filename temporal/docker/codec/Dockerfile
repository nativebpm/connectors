FROM golang:1.26-alpine AS builder
WORKDIR /app
# Копируем всю рабочую область Go Workspaces
COPY . .
# Собираем бинарник codec-server
RUN go build -o codec-server temporal/cmd/codec-server/main.go

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/codec-server .
EXPOSE 8082
CMD ["./codec-server"]
