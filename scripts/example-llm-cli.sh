#!/usr/bin/env bash
# Example LLM_CLI_CMD wrapper for systems-daily.
# Replace the body with a real headless completer you are allowed to use
# (e.g. your own script that calls an API, or a CLI with print mode).
#
# Protocol:
#   stdin:  ### SYSTEM / ### USER blocks
#   env:    SYSTEMS_DAILY_SYSTEM, SYSTEMS_DAILY_USER
#   stdout: the requested editorial stage
#
# Do NOT use this to scrape claude.ai in a browser.

set -euo pipefail
sys="${SYSTEMS_DAILY_SYSTEM:-}"
user="${SYSTEMS_DAILY_USER:-}"
if [[ -z "$sys" || -z "$user" ]]; then
  echo "missing SYSTEMS_DAILY_SYSTEM/USER" >&2
  exit 1
fi
remaining="$user"
source_ids=()
while [[ "$remaining" =~ \"id\"[[:space:]]*:[[:space:]]*\"([a-z0-9-]+)\" ]]; do
  source_ids+=("${BASH_REMATCH[1]}")
  remaining="${remaining#*"${BASH_REMATCH[0]}"}"
done
printf '<h1>CLI provider smoke test</h1>\n<h2>Mechanism</h2>\n<p>'
for _ in {1..350}; do
  printf 'mechanism '
done
for source_id in "${source_ids[@]}"; do
  printf '[[%s]] ' "$source_id"
done
printf '</p>\n<h2>Limit</h2>\n<p>'
for _ in {1..350}; do
  printf 'tradeoff '
done
printf '</p>\n'
