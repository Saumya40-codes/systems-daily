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
if [[ "$user" == *'Produce an editorial brief'* ]]; then
  printf '# Evidence brief\n\nUse the supplied mechanism and cite its source.\n'
elif [[ "$user" == *'Act as a strict technical editor'* ]]; then
  printf '# Technical critique\n\nThe smoke-test draft is repetitive but grounded.\n'
else
  remaining="$user"
  source_ids=()
  while [[ "$remaining" =~ \"id\"[[:space:]]*:[[:space:]]*\"([a-z0-9-]+)\" ]]; do
    source_ids+=("${BASH_REMATCH[1]}")
    remaining="${remaining#*"${BASH_REMATCH[0]}"}"
  done
	printf '<h1>CLI provider smoke test</h1>\n<p>'
  for _ in {1..700}; do
    printf 'mechanism '
  done
  for source_id in "${source_ids[@]}"; do
    printf '[[%s]] ' "$source_id"
  done
  printf '</p>\n'
fi
