#!/opt/homebrew/bin/bash
# =============================================================================
# smoke-test.sh — End-to-end smoke test for the Agentic RAG Chatbot
#
# Usage:
#   ./scripts/smoke-test.sh          # Run full smoke test
#   ./scripts/smoke-test.sh --skip-infra  # Skip docker compose (infra already running)
#   ./scripts/smoke-test.sh --keep  # Keep services running after test
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
WEB_DIR="$(cd "$ROOT_DIR/../web" && pwd)"
DOCKER_DIR="$ROOT_DIR/deploy"
CERTS_DIR="$ROOT_DIR/api-gateway/certs"
LOGS_DIR="$ROOT_DIR/logs"

COMPOSE="docker compose -f $DOCKER_DIR/docker-compose.yml"

# Service definitions: name → grpc_port http_port dir
# Only the core services needed for chat smoke test
declare -A SERVICES=(
  [auth-service]="$ROOT_DIR/auth-service:9081"
  [org-service]="$ROOT_DIR/org-service:9085"
  [rag-service]="$ROOT_DIR/rag-service:9082"
  [api-gateway]="$ROOT_DIR/api-gateway:8080"
)

# -------------------------------------------------------------------
# Helpers
# -------------------------------------------------------------------

log()  { echo -e "\033[1m[smoke-test]\033[0m $*"; }
info() { echo -e "\033[1m[smoke-test]\033[0m \033[34mINFO:\033[0m $*"; }
warn() { echo -e "\033[1m[smoke-test]\033[0m \033[33mWARN:\033[0m $*" >&2; }
fail() { echo -e "\033[1m[smoke-test]\033[0m \033[31mERROR:\033[0m $*" >&2; exit 1; }

_cleanup_called=0

cleanup() {
  if [[ "$_cleanup_called" -eq 1 ]]; then
    return
  fi
  _cleanup_called=1

  log "Cleaning up..."

  # Stop Next.js dev server
  if [[ -n "${WEB_PID:-}" ]] && kill -0 "$WEB_PID" 2>/dev/null; then
    info "Stopping Next.js dev server (pid: $WEB_PID)..."
    kill "$WEB_PID" 2>/dev/null || true
    wait "$WEB_PID" 2>/dev/null || true
  fi

  # Stop Go services
  for svc in api-gateway rag-service org-service auth-service; do
    stop_service "$svc"
  done

  # Stop infrastructure unless --keep was passed
  if [[ "${KEEP_INFRA:-0}" -eq 0 ]]; then
    info "Stopping Docker infrastructure..."
    $COMPOSE down >/dev/null 2>&1 || true
  fi

  log "Cleanup complete."
}

trap cleanup EXIT INT TERM

# Check if a port is listening
wait_for_port() {
  local host="$1" port="$2" name="${3:-port $port}"
  local max_wait="${4:-60}"
  info "Waiting for $name at $host:$port..."
  for i in $(seq 1 "$max_wait"); do
    if nc -z "$host" "$port" 2>/dev/null || timeout 1 bash -c "echo >/dev/tcp/$host/$port" 2>/dev/null; then
      info "$name is ready!"
      return 0
    fi
    sleep 1
  done
  fail "$name did not become available in time"
}

# Generate RSA JWT key pair if missing
ensure_certs() {
  mkdir -p "$CERTS_DIR"
  if [[ ! -f "$CERTS_DIR/jwt-private.pem" || ! -f "$CERTS_DIR/jwt-public.pem" ]]; then
    info "Generating JWT RSA key pair..."
    openssl genrsa -out "$CERTS_DIR/jwt-private.pem" 2048
    openssl rsa -in "$CERTS_DIR/jwt-private.pem" -pubout -out "$CERTS_DIR/jwt-public.pem"
    chmod 600 "$CERTS_DIR/jwt-private.pem"
    chmod 644 "$CERTS_DIR/jwt-public.pem"
    info "JWT key pair generated."
  fi
}

# Extract gRPC port for a service from SERVICES array
_service_port() {
  local name="$1"
  local entry="${SERVICES[$name]}"
  echo "${entry##*:}"
}

# Start a Go service in the background
start_service() {
  local name="$1"
  local dir="$2"
  local logfile="$LOGS_DIR/$name.log"
  local port
  port=$(_service_port "$name")
  mkdir -p "$LOGS_DIR"

  if lsof -ti :"$port" > /dev/null 2>&1; then
    info "$name is already running on port $port"
    return 0
  fi

  info "Starting $name..."
  cd "$dir"
  go run main.go -f etc/config.yaml >> "$logfile" 2>&1 &
  local pid=$!
  echo $pid > "$LOGS_DIR/$name.pid"
  info "$name started (pid: $pid)"
}

# Stop a service
stop_service() {
  local name="$1"
  local port
  port=$(_service_port "$name")
  local pidfile="$LOGS_DIR/$name.pid"

  if [[ -f "$pidfile" ]]; then
    local pid
    pid=$(cat "$pidfile")
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
    rm -f "$pidfile"
  fi

  local pids
  pids=$(lsof -ti :"$port" 2>/dev/null | tr '\n' ' ' || true)
  if [[ -n "$pids" ]]; then
    for pid in $pids; do
      kill "$pid" 2>/dev/null || true
    done
    sleep 1
  fi
}

# -------------------------------------------------------------------
# Environment
# -------------------------------------------------------------------

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck source=/dev/null
  source "$ROOT_DIR/.env"
  set +a
  info "Loaded environment from $ROOT_DIR/.env"
fi

# -------------------------------------------------------------------
# Args
# -------------------------------------------------------------------

SKIP_INFRA=0
KEEP_INFRA=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-infra)
      SKIP_INFRA=1
      shift
      ;;
    --keep)
      KEEP_INFRA=1
      shift
      ;;
    *)
      echo "Usage: $0 [--skip-infra] [--keep]"
      exit 1
      ;;
  esac
done

# -------------------------------------------------------------------
# Main
# -------------------------------------------------------------------

log "=== Agentic RAG Chatbot Smoke Test ==="
log ""

# 1. Start infrastructure
if [[ "$SKIP_INFRA" -eq 0 ]]; then
  log "Starting infrastructure (Docker)..."
  $COMPOSE up -d
  log "Infrastructure started."
fi

# 2. Wait for infrastructure
log "Waiting for infrastructure to be healthy..."
wait_for_port localhost 5432 "PostgreSQL"
wait_for_port localhost 6379 "Redis"
wait_for_port localhost 7233 "Temporal"

# 3. Ensure JWT certs
ensure_certs

# 4. Start Go services
log "Starting backend services..."
for svc in auth-service org-service rag-service; do
  dir="${SERVICES[$svc]%:*}"
  start_service "$svc" "$dir"
  sleep 0.5
done

# Wait for gRPC services
for grpc_port in 9081 9085 9082; do
  wait_for_port localhost "$grpc_port" "service on :$grpc_port" || true
done

log "Starting API Gateway..."
start_service "api-gateway" "$ROOT_DIR/api-gateway"
wait_for_port localhost 8080 "API Gateway"

# 5. Start Next.js dev server
log "Starting Next.js dev server..."
cd "$WEB_DIR"
npm run dev -- -p 3010 >> "$LOGS_DIR/web-dev.log" 2>&1 &
WEB_PID=$!
info "Next.js dev server started (pid: $WEB_PID)"
wait_for_port localhost 3010 "Next.js dev server" 120

# 6. Run Playwright smoke test
log "Running Playwright smoke test..."
cd "$WEB_DIR"
set +e
npx playwright test e2e/chat-smoke.spec.ts --reporter=list
TEST_EXIT_CODE=$?
set -e

if [[ "$TEST_EXIT_CODE" -eq 0 ]]; then
  log "✅ Smoke test passed!"
else
  log "❌ Smoke test failed (exit code: $TEST_EXIT_CODE)"
fi

# Cleanup happens via trap EXIT
exit "$TEST_EXIT_CODE"
