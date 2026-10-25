#!/usr/bin/env bash
set -euo pipefail
version=${1:?Usage: bash scripts/build-worker.sh VERSION}
[[ "$version" =~ ^[a-zA-Z0-9._-]+$ ]] || exit 1
root=$(cd "$(dirname "$0")/.." && pwd)
out="$root/dist/worker"
mkdir -p "$out"
rm -f "$out"/*.tar.gz "$out"/SHA256SUMS
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  os=${target%/*}; arch=${target#*/}
  stage=$(mktemp -d)
  trap 'rm -rf "$stage"' EXIT
  (cd "$root/worker-service" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -o "$stage/scalequeue-worker" ./cmd/worker)
  cp "$root/LICENSE" "$stage/LICENSE"
  tar -czf "$out/scalequeue-worker_${version}_${os}_${arch}.tar.gz" -C "$stage" scalequeue-worker LICENSE
  rm -rf "$stage"
done
(cd "$out" && sha256sum ./*.tar.gz > SHA256SUMS)
