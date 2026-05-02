#!/opt/homebrew/bin/bash
# =============================================================================
# start-dev.sh — Start all Sovereign AI backend services for local development
#
# Usage:
#   ./scripts/start-dev.sh          # Start everything
#   ./scripts/start-dev.sh --infra   # Start infrastructure only (docker)
#   ./scripts/start-dev.sh --stop   # Stop all services
#   ./scripts/start-dev.sh --logs   # Tail logs from all services
#   ./scripts/start-dev.sh --restart # Restart all services
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DOCKER_DIR="$ROOT_DIR/deploy"
CERTS_DIR="$ROOT_DIR/api-gateway/certs"

COMPOSE="docker compose -f $DOCKER_DIR/docker-compose.yml"

# Service definitions: name → grpc_port http_port dir
declare -A SERVICES=(
  [auth-service]="$ROOT_DIR/auth-service:9081"
  [org-service]="$ROOT_DIR/org-service:9085"
  [repo-service]="$ROOT_DIR/repo-service:9083"
  [rag-service]="$ROOT_DIR/rag-service:9082"
  [audit-service]="$ROOT_DIR/audit-service:9086"
  [doc-service]="$ROOT_DIR/doc-service:9888"
  [notification-service]="$ROOT_DIR/notification-service:9087"
  [agent-orchestrator]="$ROOT_DIR/agent-orchestrator:9088"
  [monitoring-service]="$ROOT_DIR/monitoring-service:9089"
  [api-gateway]="$ROOT_DIR/api-gateway:8080"
)

# -------------------------------------------------------------------
# Helpers
# -------------------------------------------------------------------

log()  { echo -e "\033[1m[sovereign]\033[0m $*"; }
info() { echo -e "\033[1m[sovereign]\033[0m \033[34mINFO:\033[0m $*"; }
warn() { echo -e "\033[1m[sovereign]\033[0m \033[33mWARN:\033[0m $*" >&2; }
fail() { echo -e "\033[1m[sovereign]\033[0m \033[31mERROR:\033[0m $*" >&2; exit 1; }

# Check if a port is listening
wait_for_port() {
  local host="$1" port="$2" name="${3:-port $port}"
  info "Waiting for $name at $host:$port..."
  for i in {1..60}; do
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
    info "JWT key pair generated at $CERTS_DIR/"
  else
    info "JWT key pair already exists at $CERTS_DIR/"
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
  local logfile="$ROOT_DIR/logs/$name.log"
  local port
  port=$(_service_port "$name")
  mkdir -p "$ROOT_DIR/logs"

  # Check if port is already in use (more reliable than pgrep for go run binaries)
  if lsof -ti :"$port" > /dev/null 2>&1; then
    info "$name is already running on port $port"
    return 0
  fi

  info "Starting $name..."
  cd "$dir"
  go run main.go -f etc/config.yaml >> "$logfile" 2>&1 &
  local pid=$!
  echo $pid > "$ROOT_DIR/logs/$name.pid"
  info "$name started (pid: $pid, log: $logfile)"
}

# Stop a service
stop_service() {
  local name="$1"
  local port
  port=$(_service_port "$name")
  local pidfile="$ROOT_DIR/logs/$name.pid"

  # Try pidfile first
  if [[ -f "$pidfile" ]]; then
    local pid
    pid=$(cat "$pidfile")
    if kill -0 "$pid" 2>/dev/null; then
      info "Stopping $name (pid: $pid)..."
      kill "$pid" 2>/dev/null || true
    fi
    rm -f "$pidfile"
  fi

  # Fallback: gracefully terminate by port (go run compiles to cache, path patterns don't match)
  local pids
  pids=$(lsof -ti :"$port" 2>/dev/null | tr '\n' ' ' || true)
  if [[ -n "$pids" ]]; then
    info "Gracefully stopping $name processes on port $port: $pids"
    for pid in $pids; do
      kill "$pid" 2>/dev/null || true
    done
    sleep 1
  fi
}

# -------------------------------------------------------------------
# Commands
# -------------------------------------------------------------------

cmd_infra() {
  log "Starting infrastructure (Docker)..."
  $COMPOSE up -d
  log "Infrastructure started. Use './start-dev.sh --logs' to watch logs."
}

cmd_wait_infra() {
  log "Waiting for infrastructure to be healthy..."
  $COMPOSE ps --format json | grep -q '"Health":"healthy"' || {
    $COMPOSE ps
    fail "Docker containers are not healthy. Run './start-dev.sh --infra' first."
  }
  wait_for_port localhost 5432 "PostgreSQL"
  wait_for_port localhost 6379 "Redis"
  wait_for_port localhost 7233 "Temporal"
  info "All infrastructure services are ready."
}

cmd_start() {
  ensure_certs
  cmd_wait_infra

  log "Starting gRPC services..."

  # Start gRPC services first (no HTTP gateway, no inter-service deps beyond infra)
  for svc in auth-service org-service repo-service rag-service audit-service doc-service notification-service agent-orchestrator monitoring-service; do
    dir="${SERVICES[$svc]%:*}"
    start_service "$svc" "$dir"
    sleep 0.5  # slight stagger to avoid thundering herd
  done

  # Wait for gRPC services to bind
  for grpc_port in 9081 9085 9083 9082 9086 9888 9087 9088 9089; do
    wait_for_port localhost "$grpc_port" "service on :$grpc_port" || true
  done

  log "Starting API Gateway..."
  start_service "api-gateway" "$ROOT_DIR/api-gateway"
  wait_for_port localhost 8080 "API Gateway"

  log ""
  log "All services are running:"
  log "  API Gateway:    http://localhost:8080"
  log "  Auth Service:   localhost:9081 (gRPC)"
  log "  Org Service:    localhost:9085 (gRPC)"
  log "  Repo Service:   localhost:9083 (gRPC)"
  log "  RAG Service:    localhost:9082 (gRPC)"
  log "  Audit Service:  localhost:9086 (gRPC)"
  log "  Doc Service:    localhost:9888 (gRPC)"
  log "  Notification:   localhost:9087 (gRPC)"
  log "  Agent Orch.:    localhost:9088 (gRPC)"
  log "  Monitoring:     localhost:9089 (gRPC)"
  log ""
  log "Infrastructure:"
  log "  PostgreSQL:     localhost:5432"
  log "  Redis:          localhost:6379"
  log "  Temporal:       localhost:7233"
  log "  Temporal UI:    http://localhost:8233"
  log "  Grafana:        http://localhost:3000"
  log "  Prometheus:     http://localhost:9001"
  log ""
  log "Logs: $ROOT_DIR/logs/"
  log "PIDs: $ROOT_DIR/logs/*.pid"
}

cmd_stop() {
  log "Stopping all services..."
  for svc in api-gateway agent-orchestrator monitoring-service notification-service doc-service audit-service rag-service repo-service org-service auth-service; do
    stop_service "$svc"
  done
  log "All services stopped."
}

cmd_logs() {
  if [[ ! -d "$ROOT_DIR/logs" ]]; then
    fail "No logs directory found. Services may not be running."
  fi
  exec tail -F "$ROOT_DIR/logs"/*.log
}

cmd_status() {
  log "Service status:"
  for svc in auth-service org-service repo-service rag-service audit-service doc-service notification-service agent-orchestrator monitoring-service api-gateway; do
    local pidfile="$ROOT_DIR/logs/$svc.pid"
    if [[ -f "$pidfile" ]]; then
      local pid
      pid=$(cat "$pidfile")
      if kill -0 "$pid" 2>/dev/null; then
        echo -e "  \033[32m✓\033[0m $svc (pid $pid)"
      else
        echo -e "  \033[33m?\033[0m $svc (stale pidfile)"
      fi
    else
      echo -e "  \033[31m✗\033[0m $svc (not running)"
    fi
  done
  echo ""
  echo "Docker containers:"
  $COMPOSE ps --format "table {{.Name}}\t{{.Status}}" 2>/dev/null || echo "(docker compose not available)"
}

cmd_restart() {
  cmd_stop
  sleep 2
  cmd_start
}

# -------------------------------------------------------------------
# Main
# -------------------------------------------------------------------

case "${1:-start}" in
  --infra)
    cmd_infra
    ;;
  --stop)
    cmd_stop
    ;;
  --logs)
    cmd_logs
    ;;
  --status)
    cmd_status
    ;;
  --restart)
    cmd_restart
    ;;
  --wait-infra)
    cmd_wait_infra
    ;;
  start|"")
    cmd_start
    ;;
  *)
    echo "Usage: $0 {start|--infra|--stop|--restart|--logs|--status|--wait-infra}"
    exit 1
    ;;
esac
