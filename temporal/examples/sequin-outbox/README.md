# Sequin Outbox CDC to Temporal Example

This example demonstrates the implementation of the **Transactional Outbox** pattern using **Sequin (CDC)** and **Temporal** in Go.

## Architecture

When a user is deleted from the database (`DELETE` in the `users` table), Sequin captures the change from the Postgres WAL and reliably sends a POST webhook to the HTTP server. The HTTP server receives the webhook and triggers the `DeleteUserWorkflow` in Temporal, which guarantees the complete removal of the user from external systems and sends a confirmation email.

```
[Delete User in DB] -> [Postgres WAL] -> [Sequin CDC] -> [HTTP Webhook /delete-user] -> [Temporal Workflow] -> [Worker]
```

## Project Structure

- `workflow.go` — Definition of the Temporal Workflow `DeleteUserWorkflow`.
- `activities.go` — Steps of the workflow (cleanup external systems, send emails).
- `handler.go` — HTTP handler for webhooks from Sequin.
- `server/main.go` — Execution of the HTTP server.
- `worker/main.go` — Execution of the Temporal Worker.
- `integration_test.go` — Tests for the workflow and the HTTP webhook handler.

## Local Setup

### 1. Start Temporal Server
Run the local Temporal server:
```bash
temporal server start-dev
```

### 2. Configure Postgres and Sequin
1. Create the `users` table and enable full replica identity:
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
2. Connect your database to Sequin.
3. Create a new **Webhook Sink** in Sequin:
   - **Source Table**: `users`
   - **Filters**: `Delete` only (disable `Insert` and `Update`)
   - **Endpoint URL**: `http://localhost:3333/delete-user`

### 3. Run Worker and HTTP Server
From the `temporal` directory:
```bash
# Start the Temporal Worker
go run examples/sequin-outbox/worker/main.go

# Start the webhook HTTP server in another terminal
go run examples/sequin-outbox/server/main.go
```

### 4. Manual Verification
Delete a user from Postgres:
```sql
delete from users where email = 'user@example.com';
```
You will see that the HTTP server logs a webhook payload received from Sequin, starts the `DeleteUserWorkflow` in Temporal, and the worker executes activities to remove data and send a confirmation email.

## Running Tests

To run the automated tests for this example, execute:
```bash
go test -v ./examples/sequin-outbox/...
```
