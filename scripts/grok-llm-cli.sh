#!/usr/bin/env bash
# systems-daily LLM_CLI_CMD wrapper for Grok Build CLI (headless).
# Pure article generation - no tools, no project exploration.
#
# Env: SYSTEMS_DAILY_SYSTEM, SYSTEMS_DAILY_USER
# Optional: GROK_BIN, GROK_MODEL, GROK_MAX_TURNS (default 6), GROK_EXTRA_ARGS

set -euo pipefail

sys="${SYSTEMS_DAILY_SYSTEM:-}"
user="${SYSTEMS_DAILY_USER:-}"
if [[ -z "$sys" || -z "$user" ]]; then
  echo "systems-daily grok wrapper: missing SYSTEMS_DAILY_SYSTEM/USER" >&2
  exit 1
fi
if [[ -z "${user// }" ]]; then
  echo "systems-daily grok wrapper: empty user prompt" >&2
  exit 1
fi

GROK_BIN="${GROK_BIN:-grok}"
MAX_TURNS="${GROK_MAX_TURNS:-6}"

# Empty cwd so the agent has nothing to "check" in the systems-daily tree.
EMPTY_CWD="${GROK_CWD:-}"
if [[ -z "$EMPTY_CWD" ]]; then
  EMPTY_CWD=$(mktemp -d /tmp/systems-daily-grok-XXXXXX)
  cleanup() { rm -rf "$EMPTY_CWD"; }
  trap cleanup EXIT
  # tiny marker so the dir isn't completely sterile if tools leak
  printf '%s\n' 'systems-daily content generation only; do not explore.' >"$EMPTY_CWD/README"
fi

combined="CRITICAL: You are NOT an agent exploring a codebase. Do NOT use tools. Do NOT read files. Do NOT plan. Write the article body NOW.

Output ONLY the article (HTML fragment preferred: <h1>, <p>, <pre>, optional <svg>; or markdown with # heading).
No preamble. No sentences like \"Checking...\", \"I'll look...\", \"Let me...\". No tool narration.

## System instructions
${sys}

## Task
${user}
"

# Deny essentially everything tool-related. Prefer generation-only turn loop.
args=(
  -p "$combined"
  --cwd "$EMPTY_CWD"
  --max-turns "$MAX_TURNS"
  --permission-mode dontAsk
  --output-format plain
  --no-subagents
  --disallowed-tools "run_terminal_command,search_replace,read_file,grep,list_dir,glob,Agent,web_search,web_fetch,image_gen,image_edit,spawn_subagent,todo_write,image_to_video,reference_to_video,open_page,open_page_with_find,use_tool,search_tool,monitor,workflow,enter_plan_mode,exit_plan_mode"
)

if [[ -n "${GROK_MODEL:-}" ]]; then
  args+=(-m "$GROK_MODEL")
fi
if [[ -n "${GROK_EXTRA_ARGS:-}" ]]; then
  # shellcheck disable=SC2206
  extra=( $GROK_EXTRA_ARGS )
  args+=("${extra[@]}")
fi

exec "$GROK_BIN" "${args[@]}"
