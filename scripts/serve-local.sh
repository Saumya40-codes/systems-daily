#!/usr/bin/env bash
# Serve site/public until you have a VPS. Pair with ngrok for a public URL.
#
# Usage:
#   ./scripts/serve-local.sh              # http://127.0.0.1:8765/
#   PORT=9000 ./scripts/serve-local.sh
#
# Other terminal:
#   ngrok http 8765
# Then set SITE_BASE_URL to the https URL ngrok prints (no trailing path).

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${PORT:-8765}"
DIR="${SITE_OUT_DIR:-$ROOT/site/public}"

if [[ ! -d "$DIR" ]]; then
  echo "missing $DIR — run: ./bin/systems-daily preview" >&2
  exit 1
fi

echo "serving $DIR on http://127.0.0.1:${PORT}/"
echo "  today:  http://127.0.0.1:${PORT}/today/"
echo "then:     ngrok http ${PORT}"
echo "set:      SITE_BASE_URL=https://<ngrok-host>   (in .env, no /today)"
echo

cd "$DIR"
exec python3 -m http.server "$PORT" --bind 127.0.0.1
