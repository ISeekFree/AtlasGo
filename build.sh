#!/usr/bin/env bash
set -euo pipefail

modules=(
  "."
  "integrations/mongo"
  "integrations/redis"
  "integrations/grpc"
  "demo"
)

for module in "${modules[@]}"; do
  echo "==> go test ${module}"
  (cd "${module}" && go test ./...)
done
