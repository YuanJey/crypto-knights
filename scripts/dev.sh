#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DIR="${ROOT_DIR}/.run"
BIN_DIR="${RUN_DIR}/bin"
LOG_DIR="${RUN_DIR}/logs"
PIDS=()
TAIL_PID=""

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

check_port() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1 &&
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "port ${port} is already in use" >&2
    exit 1
  fi
}

build_service() {
  local name="$1"
  echo "Building ${name}..."
  (
    cd "${ROOT_DIR}/services/${name}"
    GOWORK=off go build -trimpath -o "${BIN_DIR}/${name}" ./cmd/server
  )
}

start_service() {
  local name="$1"
  shift
  echo "Starting ${name}..."
  env "$@" "${BIN_DIR}/${name}" >"${LOG_DIR}/${name}.log" 2>&1 &
  PIDS+=("$!")
}

wait_for_health() {
  local name="$1"
  local url="$2"
  local attempts=0
  while ((attempts < 100)); do
    if curl --silent --fail --max-time 1 "${url}" >/dev/null; then
      return
    fi
    attempts=$((attempts + 1))
    sleep 0.1
  done
  echo "${name} failed its health check: ${url}" >&2
  tail -n 50 "${LOG_DIR}/${name}.log" >&2 || true
  exit 1
}

cleanup() {
  local exit_code="${1:-0}"
  trap - EXIT INT TERM
  if [[ -n "${TAIL_PID}" ]]; then
    kill "${TAIL_PID}" 2>/dev/null || true
  fi
  if ((${#PIDS[@]} > 0)); then
    kill "${PIDS[@]}" 2>/dev/null || true
    wait "${PIDS[@]}" 2>/dev/null || true
  fi
  echo "Crypto Knights services stopped."
  exit "${exit_code}"
}

require_command go
require_command curl
for port in 8081 8082 8083 8084; do
  check_port "${port}"
done

mkdir -p "${BIN_DIR}" "${LOG_DIR}"
rm -f "${LOG_DIR}"/*.log

trap 'cleanup $?' EXIT
trap 'cleanup 130' INT TERM

build_service macro-service
build_service options-service
build_service execution-service
build_service trigger-service

start_service macro-service \
  HTTP_ADDR=:8081
start_service options-service \
  HTTP_ADDR=:8082 \
  MIN_PREMIUM_NOTIONAL=100000 \
  MAX_KEY_LEVELS=10
start_service execution-service \
  HTTP_ADDR=:8084 \
  EXCHANGE_MODE=paper \
  ALLOWED_SYMBOLS=BTCUSDT,ETHUSDT \
  MAX_ORDER_NOTIONAL=10000 \
  MAX_TRIGGER_AGE_SECONDS=30
start_service trigger-service \
  HTTP_ADDR=:8083 \
  EXECUTION_SERVICE_URL=http://localhost:8084

wait_for_health macro-service http://localhost:8081/healthz
wait_for_health options-service http://localhost:8082/healthz
wait_for_health execution-service http://localhost:8084/healthz
wait_for_health trigger-service http://localhost:8083/healthz

cat <<'EOF'
Crypto Knights is running:
  Macro:    http://localhost:8081
  Options:  http://localhost:8082
  Trigger:  http://localhost:8083
  Execution:http://localhost:8084

Press Ctrl+C to stop all services.
EOF

tail -n +1 -F "${LOG_DIR}"/*.log &
TAIL_PID="$!"

while true; do
  for pid in "${PIDS[@]}"; do
    if ! kill -0 "${pid}" 2>/dev/null; then
      set +e
      wait "${pid}"
      status="$?"
      set -e
      echo "A service exited unexpectedly with status ${status}." >&2
      exit "${status}"
    fi
  done
  sleep 1
done
