#!/usr/bin/env bash
# Example LLM_CLI_CMD wrapper for systems-daily.
# Replace the body with a real headless completer you are allowed to use
# (e.g. your own script that calls an API, or a CLI with print mode).
#
# Protocol:
#   stdin:  ### SYSTEM / ### USER blocks
#   env:    SYSTEMS_DAILY_SYSTEM, SYSTEMS_DAILY_USER
#   stdout: article body only (HTML fragment or markdown)
#
# Do NOT use this to scrape claude.ai in a browser.

set -euo pipefail
sys="${SYSTEMS_DAILY_SYSTEM:-}"
user="${SYSTEMS_DAILY_USER:-}"
if [[ -z "$sys" || -z "$user" ]]; then
  echo "missing SYSTEMS_DAILY_SYSTEM/USER" >&2
  exit 1
fi
cat <<'HTML'
<h1>CLI provider smoke test</h1>
<p>Replace <code>scripts/example-llm-cli.sh</code> with a real completer.</p>
<pre>pipeline works</pre>
HTML
