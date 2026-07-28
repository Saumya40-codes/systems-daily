# systems-daily

I'm more on the low-level systems side (memory, OS, embedded, GNSS, that kind of thing), and I want a light daily touchpoint with it. This tool generates one write-up each day on a topic from that area. Length sits in the middle: long enough to learn something, short enough to finish with coffee.

Uses an LLM via **HTTP** (OpenAI-compatible: Groq, Ollama, OpenRouter, xAI, ...) or an optional **CLI** provider (your local script / headless CLI; stdout = article). Plain SMTP for notify mail.

### LLM providers

| `LLM_PROVIDER` | Config | Notes |
|----------------|--------|--------|
| `http` (default) | `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL` | Chat Completions API |
| `cli` | `LLM_CLI_CMD`, optional `LLM_CLI_ARGS` | Runs a command; stdin = `### SYSTEM` / `### USER`; also sets `SYSTEMS_DAILY_SYSTEM` / `SYSTEMS_DAILY_USER`. **Does not** open claude.ai in a browser. |

Example smoke test with the included stub:

```bash
LLM_PROVIDER=cli LLM_CLI_CMD=./scripts/example-llm-cli.sh ./bin/systems-daily preview --topic watchdogs
```

Point `LLM_CLI_CMD` at your own wrapper around a real headless completer you are allowed to use.

Reading is on a minimal static site (not the inbox). Mail is a short notify with a link. Optional PDF attach if you want it.

## Setup

```bash
# LLM (example: Ollama)
ollama pull llama3.2
ollama serve

# config
cp .env.example .env
# set SMTP_*, LLM_*, and SITE_BASE_URL for public links
```

## Build

```bash
go build -o bin/systems-daily ./cmd/systems-daily
```

## Commands

```bash
./bin/systems-daily preview              # generate + write site/public; no email
./bin/systems-daily preview --topic buddy-allocator
./bin/systems-daily once                 # publish site + notify email
./bin/systems-daily serve                # daily at SEND_AT (default 09:00)
./bin/systems-daily serve --now          # once now, then on schedule
./bin/systems-daily topics               # list topic catalog
```

### Reading surface (static site)

Each run writes HTML under `SITE_OUT_DIR` (default `site/public`):

| Path | Role |
|------|------|
| `/` | Latest note |
| `/today/` | Always the current write-up (use this in email) |
| `/d/YYYY-MM-DD/` | Dated copy; **pruned after `SITE_WINDOW_DAYS`** (default **7**) |

Design is intentionally plain: system fonts, monochrome, narrow column. No marketing UI.

Local check:

```bash
./bin/systems-daily preview --topic watchdogs
python3 -m http.server 8080 --directory site/public
# http://127.0.0.1:8080/today/
```

Deploy `site/public` to Vercel (or any static host). Set `SITE_BASE_URL` to that origin so email links work. See `site/README.md`.

### Notify email

- Subject + category + date
- `Read: https://.../today/`
- PDF attachment only if `ATTACH_PDF=1`

### Optional PDF

PDF generation still exists for archive/attach (`ATTACH_PDF=1`). Primary path is the site.

## Config

Env vars (or `.env` in cwd). See `.env.example`.

| Variable | Default | Notes |
|----------|---------|-------|
| `LLM_PROVIDER` | `http` | `http` or `cli` |
| `LLM_BASE_URL` | `http://localhost:11434/v1` | HTTP: OpenAI-compatible base URL |
| `LLM_MODEL` | `llama3.2` | HTTP: model name |
| `LLM_API_KEY` | `ollama` | HTTP: bearer token |
| `LLM_CLI_CMD` | | CLI: command or script path |
| `LLM_CLI_ARGS` | | CLI: extra args (space-separated) |
| `SMTP_*` | | Mail notify |
| `SEND_AT` | `09:00` | Daily send time |
| `TIMEZONE` | local | IANA zone, e.g. `Asia/Kolkata` |
| `SITE_OUT_DIR` | `site/public` | Static web root |
| `SITE_BASE_URL` | | Public origin for links (no trailing slash required) |
| `SITE_WINDOW_DAYS` | `7` | Keep dated pages this many days |
| `ATTACH_PDF` | `false` | Also attach PDF to email |
| `HISTORY_PATH` | `data/history.json` | Avoids recent topic repeats |
| `TOPICS_PATH` | (embedded) | Custom topics JSON |
| `DRY_RUN` | `false` | Generate but do not send |

## Topics catalog

Topics live in JSON (`internal/topics/topics.json`), embedded into the binary as the default.

```bash
cp internal/topics/topics.json ./topics.json
export TOPICS_PATH=./topics.json
./bin/systems-daily topics
```

Catalog titles are **already slices** (e.g. "Windowed WDT: kick too early vs too late"), not course overviews.

### Content format (hybrid)
The model preferably returns an **HTML fragment** (headings, paragraphs, `<pre>`, inline SVG). The site shell (brand, date, CSS, CSP) is always ours. Markdown is still accepted as a fallback. Scripts and inline event handlers are stripped.

## Layout

```
cmd/systems-daily/          CLI (cobra)
site/public/                generated static HTML (deploy this)
internal/topics/            catalog
internal/config/            env config
internal/llm/               OpenAI-compatible client
internal/content/           prompts + article shaping
internal/site/              markdown → minimal HTML + publish/prune
internal/diagrams/          SVG→PNG for optional PDF only
internal/pdfdoc/            optional PDF
internal/email/             SMTP notify (+ optional attach)
internal/history/           sent topic log
internal/schedule/          daily runner
internal/app/               once pipeline
```
