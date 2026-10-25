# Maintenance and releases

## Checks

In `api-service`, run `npm ci`, `npm run format:check`, `npm run lint`,
`npm run build` and `npm test`. `npm run format` applies TypeScript formatting.
Lint checks common JavaScript correctness errors plus TypeScript type checking.
In `worker-service`, run `gofmt -w .`, `go vet ./...` and `go test -race ./...`.
Database integration tests require the test stores as configured in CI.
CI enforces these checks on pushes and pull requests.

Dependabot proposes weekly npm, Go, GitHub Actions and Dockerfile updates.
Updates require review and passing checks; they are not automatically merged.
Compose image versions are maintained manually.

## Reproduce the synthetic load test

Use a disposable checkout with Docker Engine, Compose v2 and Node 22 installed.
Start with fresh Compose volumes; do not use a stack containing valuable data.

```bash
docker compose up -d --build
curl --fail http://127.0.0.1:3000/health
curl --fail http://127.0.0.1:9090/ready
JOBS=30 CONCURRENCY=4 node scripts/benchmark-report.cjs
```

Wait until both health endpoints succeed before running the test. The report runs
three sequential trials and writes `benchmark-results.json`. Each job invokes the
synthetic notification handler, which waits 300ms. End-to-end latency includes
enqueue, queue waiting, handler execution and 50ms status polling. Throughput is
completed jobs divided by trial wall time, not API acceptance throughput.
Failures make the command fail. There is no warmup and no performance pass threshold.

Each CI run uploads the raw report as `benchmark-and-worker-assets` and prints it
in the logs. Reports include commit, timestamp, CPU, RAM, OS, Node and container
image identifiers. Compare repeated trials on equivalent hardware and fresh stores;
shared GitHub runners vary. These measurements do not establish production capacity.
The older `benchmark-local.json` is historical, not a baseline for the new harness.

## Versioned worker downloads

Actions → Prepare draft release → Run workflow on `main` → enter `vMAJOR.MINOR.PATCH`.
The workflow checks the exact selected main commit, runs reliability and load tests,
then builds Linux/macOS amd64/arm64 worker archives and SHA256SUMS. Archives include
the ISC license. The draft release receives these files; publication remains manual.
No container registry is required. Existing tags/releases are rejected.

For a local build with the Go version from `worker-service/go.mod`:

```bash
bash scripts/build-worker.sh local
cd dist/worker
sha256sum -c SHA256SUMS
```

Only Linux amd64 is exercised by integration tests; the other targets are cross-built.
Workers still require the database schema, Redis and environment configuration
documented in the README. These binaries are not standalone servers with bundled stores.
Tags created by the draft workflow are public even while the release remains a draft.
If drafting fails after tagging, inspect that tag and recover manually; do not move it.

The root ISC license matches the API package's existing license declaration.
