# Camunda External Task Client

A Go client for Camunda 7 external tasks with fluent API support.

## Features

- Fluent API for all external task operations
- Fetch and lock external tasks
- Complete tasks with variables
- Handle task failures with retry logic
- Extend and unlock task locks
- Middleware support for logging and tracing
- Structured logging with slog
- Type-safe variable handling
- Process deployment support
- Process instance management

## Installation

```bash
go get github.com/nativebpm/connectors/camunda
```

## Examples

Check out the [examples](./examples) directory for complete working examples:

- **[loan-granting](./examples/loan-granting)** - Complete external task worker with BPMN deployment, process start, and multi-instance subprocess handling

## API Reference

### Client Creation

- `NewClient(hostURL, workerID)` - Create a new client (automatically adds `/engine-rest`)
- `WithLogger(logger)` - Add logging middleware
- `Use(middleware)` - Add custom middleware

### Task Operations

- `FetchAndLock(ctx, topics, maxTasks, asyncTimeout)` - Fetch and lock tasks
- `Complete(taskID)` - Create a completion builder
- `Failure(taskID)` - Create a failure builder
- `ExtendLock(taskID, newDuration)` - Create a lock extension builder
- `Unlock(taskID)` - Create an unlock builder
- `PollTasks(ctx, topics, maxTasks, handler)` - Poll for tasks continuously

### Process Operations

- `DeployProcess(ctx, deploymentName, reader, filename)` - Deploy BPMN process
- `StartProcessInstance(ctx, processDefinitionKey, variables)` - Start process instance

## Development

### Run Tests

```bash
go test -v
```

### Run Benchmarks

```bash
go test -bench=. -benchmem
```

## License

See the main repository LICENSE file.