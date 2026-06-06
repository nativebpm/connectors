# Коннектор Restate для Go

Этот пакет предоставляет упрощенную, удобную (fluent) и надежную обертку над Go SDK для [Restate](https://github.com/restatedev/restate), обеспечивающую отказоустойчивое и долговечное (durable) выполнение обработчиков микросервисов, рабочих процессов (workflows) и виртуальных объектов с сохранением состояния.

---

## Возможности
- **Fluent Configuration Builder**: Загрузка переменных окружения и настройка параметров подключения с помощью цепочки методов (method chaining) и встроенной обработкой ошибок (sticky errors).
- **Server Wrapper**: Простая привязка и запуск нескольких сервисов, виртуальных объектов или воркфлоу на базе встроенного HTTP/2 сервера.
- **Generic Client Wrapper**: Типизированные синхронные запросы (request-response) и асинхронные сигналы (one-way fire-and-forget) к сервисам Restate из внешнего Go-кода.
- **Долговечное выполнение и таймеры**: Удобная структура кода с персистентным хранилищем состояния и энергоэффективной приостановкой выполнения на время таймеров.

---

## Настройка конфигурации

Пакет включает удобный `ConfigBuilder` для создания конфигурации. По умолчанию считываются переменные окружения `RESTATE_HOST_PORT` и `RESTATE_SERVER_URL`:

```go
import restateconn "github.com/nativebpm/connectors/restate"

cfg, err := restateconn.NewConfigBuilder().
    FromEnv().
    WithHostPort("0.0.0.0:9080").
    WithServerURL("http://localhost:8080").
    Build()
```

---

## Создание и регистрация сервисов

### 1. Виртуальные объекты (Virtual Objects)
Виртуальный объект инкапсулирует бизнес-логику и постоянное состояние, привязанное к уникальному ключу. Обработчики принимают `restate.ObjectContext`, предоставляющий операции чтения/записи во встроенное хранилище Restate K/V.

```go
type Counter struct{}

func (Counter) Add(ctx restate.ObjectContext, amount int) (int, error) {
    // Чтение состояния
    val, _ := restate.Get[int](ctx, "count")
    newVal := val + amount

    // Запись состояния
    restate.Set(ctx, "count", newVal)

    // Выполнение побочных эффектов с гарантией однократности
    _, _ = restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
        fmt.Printf("Counter updated to %d\n", newVal)
        return "ok", nil
    })

    return newVal, nil
}
```

### 2. Рабочие процессы (Workflows) и надежные таймеры
Воркфлоу принимает `restate.WorkflowContext`, поддерживающий долговечные таймеры (`restate.Sleep`) и цепочки операций (activities):

```go
type OrderWorkflow struct{}

func (OrderWorkflow) Run(ctx restate.WorkflowContext, orderID string) (string, error) {
    // Шаг 1: Валидация оплаты (durable run)
    _, err := restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
        return "PAID", nil
    })

    // Шаг 2: Долговечный таймер сна (освобождает вычислительные ресурсы)
    err = restate.Sleep(ctx, 5 * time.Second)

    // Шаг 3: Доставка
    _, err = restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
        return "SHIPPED", nil
    })

    return "SUCCESS", nil
}
```

### 3. Регистрация на сервере (Fluent API)
Для запуска обработчиков привяжите их к серверу с помощью цепочки методов:

```go
err := restateconn.NewServer(cfg).
    Bind(Counter{}).
    Bind(OrderWorkflow{}).
    Start(context.Background())
```

---

## Вызов сервисов из внешнего Go-клиента (Ingress)

Для отправки запросов к сервисам Restate из стандартного приложения Go (вне контекста Restate) используйте типизированный клиент:

```go
// 1. Инициализация клиента
client := restateconn.NewClient(cfg)

// 2. Вызов простого сервиса или воркфлоу (запрос-ответ)
result, err := restateconn.Service[string, string](client, "MyService", "MyHandler").
    Request(context.Background(), "input_payload")

// 3. Вызов метода виртуального объекта (требуется ключ объекта)
newVal, err := restateconn.Object[int, int](client, "Counter", "my-counter-key", "Add").
    Request(context.Background(), 10)

// 4. Отправка асинхронного сигнала (fire-and-forget)
err = restateconn.Object[int, int](client, "Counter", "my-counter-key", "Add").
    Send(context.Background(), 5)
```
