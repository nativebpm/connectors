# Restate Connector for Go

This package provides a simplified, fluent, and robust Go SDK wrapper for [Restate](https://github.com/restatedev/restate), enabling durable, fault-tolerant execution of microservice handlers, workflows, and stateful Virtual Objects.

---

## Features
- **Fluent Configuration Builder**: Load environment variables and customize connection parameters using a method-chaining configuration builder with sticky error handling.
- **Server Wrapper**: Bind and run multiple services, virtual objects, or workflows easily on a standalone HTTP/2 server.
- **Generic Client Wrapper**: Send request-response queries and fire-and-forget signals to Restate services using a type-safe generic client interface.
- **Durable Executions & Timers**: Easily structure stateful objects and workflows with durable state persistence and resilient timer suspension.

---

## Configuration

The package includes a fluent `ConfigBuilder` to define connection properties. It parses defaults from env variables (`RESTATE_HOST_PORT` and `RESTATE_SERVER_URL`):

```go
import restateconn "github.com/nativebpm/connectors/restate"

cfg, err := restateconn.NewConfigBuilder().
    FromEnv().
    WithHostPort("0.0.0.0:9080").
    WithServerURL("http://localhost:8080").
    Build()
```

---

## Defining and Registering Services

### 1. Stateful Virtual Objects
A Virtual Object encapsulates logic and persistent state. It receives a `restate.ObjectContext` parameter, allowing read/write operations to Restate's key-value store.

```go
type Counter struct{}

func (Counter) Add(ctx restate.ObjectContext, amount int) (int, error) {
    // Read state
    val, _ := restate.Get[int](ctx, "count")
    newVal := val + amount

    // Write state
    restate.Set(ctx, "count", newVal)

    // Execute side effects durably
    _, _ = restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
        fmt.Printf("Counter updated to %d\n", newVal)
        return "ok", nil
    })

    return newVal, nil
}
```

### 2. Workflows with Durable Timers
A workflow receives a `restate.WorkflowContext` parameter, supporting durable timers (`restate.Sleep`) and sequential activities:

```go
type OrderWorkflow struct{}

func (OrderWorkflow) Run(ctx restate.WorkflowContext, orderID string) (string, error) {
    // Step 1: Validate payment
    _, err := restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
        return "PAID", nil
    })

    // Step 2: Durable Sleep (pauses execution and suspends billing resources)
    err = restate.Sleep(ctx, 5 * time.Second)

    // Step 3: Ship items
    _, err = restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
        return "SHIPPED", nil
    })

    return "SUCCESS", nil
}
```

### 3. Server Registration (Fluent API)
To serve your definitions, bind them to the HTTP/2 server wrapper:

```go
err := restateconn.NewServer(cfg).
    Bind(Counter{}).
    Bind(OrderWorkflow{}).
    Start(context.Background())
```

---

## Invoking Services from Go Client (Ingress)

To interact with Restate services from a standard Go application (outside the Restate context), use the generic `Client` wrapper:

```go
// 1. Initialize client
client := restateconn.NewClient(cfg)

// 2. Call basic service or workflow request-response
result, err := restateconn.Service[string, string](client, "MyService", "MyHandler").
    Request(context.Background(), "input_payload")

// 3. Invoke Virtual Object method (requires key)
newVal, err := restateconn.Object[int, int](client, "Counter", "my-counter-key", "Add").
    Request(context.Background(), 10)

// 4. Send fire-and-forget one-way signal
err = restateconn.Object[int, int](client, "Counter", "my-counter-key", "Add").
    Send(context.Background(), 5)
```
