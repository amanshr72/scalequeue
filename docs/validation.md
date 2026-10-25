# Local validation — 25 October 2026

All changes are local and unpublished. Base: `cffb771800b48bf70bb8e7d6846e430bd0f8a43e`.

## Executed

- TypeScript build and the Node request-hash test passed.
- Go 1.25.5 package build, `go vet ./...`, and race-enabled tests passed.
- 11 top-level Go tests passed, including six integration tests using real MariaDB 10.11.14 and Redis 7.0.15. No integration test was skipped in the full run.
- Legacy migration test passed: existing retry budgets retained, interrupted work backfilled, down/up applied, repeated up was a no-op.
- Full API/worker exercise passed: auth, ten concurrent same-key requests, conflicting payload rejection, durable acceptance during Redis pause, retries/DLQ, concurrent replay, old-generation delivery, worker SIGKILL/restart, invalid payload, audit logs, metrics, and bounded shutdown.
- Added a deterministic regression test for handler completion while a heartbeat query is still in flight.
- JavaScript syntax, YAML parsing, Docker copy inputs, tracked lockfile, and whitespace were checked locally.

## Not executed here

Docker Engine is unavailable in this environment. Container image builds, Compose startup, and the MySQL 8.4/Redis 7.4 matrix are configured in CI but have **not** run. The native integration run uses MariaDB 10.11.14/Redis 7.0.15; this is not a substitute claim for a passed MySQL 8.4 container run. Node locally was v24.22.1; images/CI target Node 22. The GitHub workflow itself is unrun while the push hold is active.

## Small measured workload

See [raw local output](benchmark-local.json). Twenty synthetic notification jobs, four concurrent clients, two Go workers, 300ms simulated handler delay, local native database/Redis, Linux environment exposing eight CPUs:

- Completed: 20/20; errors: 0.
- Duration: 3.206 seconds; completion throughput: 6.238 jobs/second.
- End-to-end p50: 585ms; p95: 769ms.

This is a functional demo workload including network polling and artificial handler delay. It is not a saturation benchmark or a production capacity claim. No CPU/memory resource limits were imposed, and the shared environment can affect timing.
