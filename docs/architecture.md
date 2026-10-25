# ScaleQueue design guide

This guide describes implemented behavior. Email and notification handlers simulate work;
no external messages or payments are sent.

## Acceptance and delivery

The API returns acceptance after MySQL commits the job and outbox intent. Redis can be
temporarily unavailable without losing that dispatch intent.

```mermaid
sequenceDiagram
  participant API as TypeScript API
  participant DB as MySQL
  participant Dispatcher as Go dispatcher
  participant Redis as Redis Streams
  API->>DB: Transaction with job, audit, and outbox intent
  DB-->>API: COMMIT
  Note over API,DB: API can now return 202
  Dispatcher->>DB: Lock a due outbox row
  Dispatcher->>Redis: XADD job ID and generation
  Redis-->>Dispatcher: Delivery ID
  Dispatcher->>DB: Record publication and commit
  Note over DB,Redis: Crash before commit can duplicate publication
```

Intents for nonterminal work become eligible for republishing after 30 seconds.
This is a delivery repair path, not a strict completion-time guarantee.

## Execution and durable retry

A row lock serializes claims. A renewable lease controls which owner may persist
an outcome. A retry is a future outbox entry, not an in-memory sleep.

```mermaid
sequenceDiagram
  participant Redis as Redis Streams
  participant Worker as Go consumer
  participant DB as MySQL
  participant Handler as Job handler
  Redis-->>Worker: Job ID and generation
  Worker->>DB: Claim under row lock and increment attempt
  DB-->>Worker: Job and ownership lease
  Worker->>Handler: Execute with cancellation context
  Note over Worker,DB: Heartbeats renew the lease during execution
  Handler-->>Worker: Success or failure
  alt Success
    Worker->>DB: Persist COMPLETED and audit atomically
  else Retryable failure with budget
    Worker->>DB: Persist PENDING, due time, audit, and next intent
  else Permanent failure or exhausted budget
    Worker->>DB: Persist DEAD and audit atomically
  end
  DB-->>Worker: COMMIT
  Worker->>Redis: ACK delivery
```

If persistence or ownership checks fail, the consumer leaves the delivery pending.
Active-lease duplicates wait for recovery; terminal/old-generation duplicates can
be acknowledged without executing a handler.

## Two distinct identities

| Identity | Scope | Purpose |
| --- | --- | --- |
| Client + Idempotency-Key | Job acceptance | Repeated equivalent HTTP requests identify the same job |
| Job ID + generation | Execution/replay | Old deliveries cannot execute a deliberately replayed generation |
| Lease owner | Current attempt | Expired/stale owners cannot persist outcomes |

A replay increments generation and resets the attempt budget. For real external effects,
use an appropriate downstream idempotency key; leases alone cannot prevent every
external duplicate after interruption.

## How to inspect the implementation

- API transactions and replay: `api-service/src/services/jobService.ts`.
- Claims, fencing, outcomes, and dispatch: `worker-service/internal/db/mysql.go`.
- Heartbeat/handler coordination: `worker-service/internal/worker/worker.go`.
- Stream delivery and cursor recovery: `worker-service/internal/queue/consumer.go`.
- Failure tests: `worker-service/internal/db/integration_test.go`,
  `worker-service/internal/queue/consumer_test.go`, and `scripts/integration.cjs`.

See [reliability](reliability.md) for upgrade instructions and boundaries, and
[validation](validation.md) for the checks actually executed.
