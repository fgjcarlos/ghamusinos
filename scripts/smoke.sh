#!/usr/bin/env bash
# Smoke del binario con la SPA embebida. Arranca ./bin/ghamusinos, espera
# a /healthz, y verifica que la SPA se sirve de verdad (no el 503 de
# "frontend no construido") y que las cabeceras de seguridad están.
#
# Variables de entorno requeridas:
#   DATABASE_URL      - Postgres reachable desde el runner.
#   CLERK_JWKS_URL    - URL del JWKS de Clerk (config sólo valida que esté
#                       definida; el smoke no toca rutas /api/*).
# Opcionales:
#   PORT              - puerto del binario (default 8080)
#   BINARY            - ruta al binario (default ./bin/ghamusinos)
#
# Sale con código 0 si todo va bien; en cualquier fallo, imprime el log
# del binario y sale con código 1.

set -euo pipefail

PORT="${PORT:-8080}"
BASE_URL="http://127.0.0.1:${PORT}"
BINARY="${BINARY:-./bin/ghamusinos}"
LOG=$(mktemp --suffix=.smoke.log)
PID=""

if [ ! -x "$BINARY" ]; then
  echo "smoke: binario no encontrado o no ejecutable: $BINARY" >&2
  exit 1
fi

cleanup() {
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    # Da margen al shutdown ordenado del binario.
    for _ in 1 2 3 4 5; do
      kill -0 "$PID" 2>/dev/null || break
      sleep 0.2
    done
    kill -9 "$PID" 2>/dev/null || true
  fi
  rm -f "$LOG"
}
trap cleanup EXIT

# Helper: ejecuta curl -fsS y, si falla, muestra el log del binario y sale 1.
fetch() {
  local url="$1"
  local body
  if ! body=$(curl -fsS "$url" 2>&1); then
    echo "smoke: GET $url falló. Log del binario:" >&2
    cat "$LOG" >&2
    exit 1
  fi
  printf '%s' "$body"
}

fetch_headers() {
  local url="$1"
  if ! curl -fsSI "$url" >/tmp/smoke.headers.$$ 2>&1; then
    echo "smoke: HEAD $url falló. Log del binario:" >&2
    cat "$LOG" >&2
    rm -f /tmp/smoke.headers.$$
    exit 1
  fi
  cat /tmp/smoke.headers.$$
  rm -f /tmp/smoke.headers.$$
}

DATABASE_URL="${DATABASE_URL:?smoke: DATABASE_URL es obligatoria}"
CLERK_JWKS_URL="${CLERK_JWKS_URL:-https://clerk.example.invalid/jwks}"

DATABASE_URL="$DATABASE_URL" \
CLERK_JWKS_URL="$CLERK_JWKS_URL" \
PORT="$PORT" \
"$BINARY" >"$LOG" 2>&1 &
PID=$!

# Espera hasta 10s a /healthz.
ready=0
for _ in $(seq 1 100); do
  if curl -fsS "$BASE_URL/healthz" >/dev/null 2>&1; then
    ready=1
    break
  fi
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "smoke: el binario terminó antes de estar listo. Log:" >&2
    cat "$LOG" >&2
    exit 1
  fi
  sleep 0.1
done

if [ "$ready" != "1" ]; then
  echo "smoke: /healthz no respondió en 10s. Log del binario:" >&2
  cat "$LOG" >&2
  exit 1
fi

# 1) /healthz devuelve 200 con "status":"ok".
if ! fetch "$BASE_URL/healthz" | grep -q '"status":"ok"'; then
  echo "smoke: /healthz no devolvió status ok" >&2
  exit 1
fi

# 2) / sirve el index.html real de la SPA, no el 503 de "frontend no construido".
index_body=$(fetch "$BASE_URL/")
if ! printf '%s' "$index_body" | grep -q '<div id="root"'; then
  echo "smoke: / no sirvió el index.html de la SPA (¿se saltó pnpm build?). Cuerpo:" >&2
  echo "$index_body" | head -20 >&2
  exit 1
fi

# 3) Una ruta cliente cae al mismo index.html (SPA routing).
client_body=$(fetch "$BASE_URL/actividades")
if ! printf '%s' "$client_body" | grep -q '<div id="root"'; then
  echo "smoke: /actividades no cayó al index.html. Cuerpo:" >&2
  echo "$client_body" | head -20 >&2
  exit 1
fi

# 4) Cabecera de seguridad CSP presente.
if ! fetch_headers "$BASE_URL/" | grep -qi '^content-security-policy:'; then
  echo "smoke: cabecera Content-Security-Policy ausente" >&2
  exit 1
fi

echo "smoke OK"
