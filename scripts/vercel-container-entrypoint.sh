#!/usr/bin/env bash
set -eu
harvest_pid=""
app_pid=""
cleanup() {
  [ -z "$app_pid" ] || kill -TERM "$app_pid" 2>/dev/null || true
  [ -z "$harvest_pid" ] || kill -TERM "$harvest_pid" 2>/dev/null || true
}
trap cleanup EXIT TERM INT
export DS2API_CHAT_HISTORY_PATH="${DS2API_CHAT_HISTORY_PATH:-/tmp/chat_history.json}"
if [ -z "${DEVICE_HARVEST_URL:-}" ]; then
  export HARVEST_TOKEN="$(node -e "process.stdout.write(require('crypto').randomBytes(32).toString('hex'))")"
  export DEVICE_HARVEST_TOKEN="$HARVEST_TOKEN"
  export DEVICE_HARVEST_URL="http://127.0.0.1:8090"
  export HARVEST_HOST="127.0.0.1"
  /app/docker-entrypoint.sh node /app/harvest.mjs serve --port 8090 &
  harvest_pid=$!
  ready=0
  for attempt in $(seq 1 60); do
    if curl -fsS http://127.0.0.1:8090/healthz >/dev/null 2>&1; then ready=1; break; fi
    kill -0 "$harvest_pid" 2>/dev/null || break
    sleep 0.5
  done
  if [ "$ready" != 1 ]; then echo "device harvest service failed to start" >&2; exit 1; fi
fi
/usr/local/bin/ds2api &
app_pid=$!
if [ -n "$harvest_pid" ]; then
  set +e
  wait -n "$app_pid" "$harvest_pid"
  status=$?
  set -e
else
  set +e
  wait "$app_pid"
  status=$?
  set -e
fi
exit "$status"
