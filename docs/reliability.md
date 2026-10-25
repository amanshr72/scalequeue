# Reliability contract and failure audit

## Durable state

`PENDING` → `PROCESSING` → `COMPLETED`, `PENDING` (scheduled retry), or `DEAD`.

`max_retries=N` permits at most N+1 claims per generation. Claims, including interrupted executions, increment `attempt_count`; `retry_count` is the zero-based attempt index. Invalid payloads/unknown legacy job types fail permanently. The accepted types are `send_email`, `send_notification`, and `fail_job`.

Replay locks a terminal job, increments its generation, resets attempts, and inserts a new outbox intent in the same transaction. A reused enqueue idempotency key continues to refer to the original job, including its current replay generation. Request keys are scoped to API client identity and retained for the lifetime of the job.

## Failure boundaries

| Failure | Result |
| --- | --- |
| API dies before SQL commit | Neither job nor dispatch intent is accepted |
| API loses response after commit | Client retries safely with the same idempotency key; without a key it can create a second job |
| Redis unavailable after acceptance | Durable outbox intent remains due and is retried |
| Dispatcher publishes then dies before commit | Duplicate publication is possible; database claims serialize execution |
| Delivery repeats during a healthy attempt | Active lease blocks a second claim; delivery remains pending until safely recoverable |
| Worker dies mid-handler | Lease eventually expires; periodic recovery or outbox republish delivers another attempt |
| Worker persists outcome then dies before ACK | Duplicate sees terminal state and ACKs without executing |
| Completion/retry/audit transaction fails | Delivery is not ACKed; rollback preserves the recoverable processing state |
| Retry delay spans restart | Due time and dispatch intent survive in MySQL |
| Lease is stolen/expired | Old owner cannot renew or persist an outcome |
| Handler panics or times out | No ACK; later recovery consumes the next attempt or marks exhausted work DEAD |
| Replay receives old messages | Generation mismatch prevents execution |
| Redis stream is lost | Nonterminal dispatch intents republish within the 30-second sweep interval, subject to backlog |

ACK acknowledges a Redis delivery, not a business side effect. An early duplicate during a retry delay can be ACKed because a durable future outbox intent owns the next attempt.

## External effects and fencing limits

Database leases prevent concurrent *valid owners* and fence outcome writes. A paused or partitioned process may resume after expiry and still attempt an external effect. A provider must honor a stable key such as `job_id:generation`, or the effect must be recorded transactionally with a uniqueness constraint, to prevent duplicates. Cancellation is cooperative. This demonstrator's handlers only wait; they do not prove exactly-once email, payments, or arbitrary external effects.

There is no priority scheduling guarantee: priority is retained as job metadata. Redis order and due time govern dispatch. No throughput, no-loss-under-all-disasters, or production-readiness claim is made.

## Upgrade from the original schema

1. Back up MySQL and Redis. Stop API writes and drain/stop old workers. Do not run old and new worker versions together.
2. Preserve the old stream for inspection by renaming it, for example `RENAME jobs:queue jobs:queue:pre-durable` if it exists. Use an unused archive name. Old messages lack generation fields and are intentionally not silently processed by the new consumer.
3. Run `npm run migrate` with the new code. The migration preserves consumed retry budgets and creates outbox intents for existing pending/processing jobs. Legacy interrupted processing becomes pending.
4. Start the new API and workers, then verify health, pending counts, completion, and retry/DLQ behavior.

MySQL DDL auto-commits; migration application is not fully transactional. If interrupted, inspect schema state before repairing/retrying. Rollback via `npm run migrate:down -- --count 1` removes durable-delivery state and request keys. Stop services and back up before rollback; it is not a lossless operational recovery strategy.

## Operations

- API `/health`: dependency readiness (503 if MySQL or Redis is unavailable).
- Worker `/health`: HTTP liveness; `/ready`: dependency readiness.
- Worker `/metrics`: status counts, oldest age per status, in-flight count, attempt/completion/retry/dead/error counters, total processing time. Counters reset per process; status gauges come from MySQL. Aggregate counters across worker replicas. No histogram is currently exposed.
- Structured worker logs include job ID, generation, attempt, status, and worker identity.
- `DEAD` jobs form the durable DLQ in MySQL and are replayed through the authenticated API.
- Malformed stream messages remain pending and emit errors for operator investigation. They require explicit remediation.
- Stream/audit/job retention and resource quotas remain operator responsibilities. No automatic stream trimming is enabled. Outbox dispatch uses bounded database/network timeouts; one dispatch can hold an outbox row lock during publication.
- Local API keys are plaintext demonstrator credentials. The shared queue has no per-client data isolation. Production deployment needs appropriate authentication, authorization, retention, TLS, resource controls, and operational review.
