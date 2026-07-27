# systems-daily

I'm more on the low-level systems side (memory, OS, embedded, GNSS, that kind of thing), and I want a light daily touchpoint with it. This tool emails one write-up each morning on a topic from that area. Length sits in the middle: long enough to learn something, short enough to finish with coffee.

Uses any OpenAI-compatible LLM (local Ollama by default) and plain SMTP.

## Setup

```bash
# LLM (example: Ollama)
ollama pull llama3.2
ollama serve

# config
cp .env.example .env
# set SMTP_* and optionally LLM_*
```

## Build

```bash
go build -o bin/systems-daily ./cmd/systems-daily
```

## Commands

```bash
./bin/systems-daily preview              # generate, print to stdout
./bin/systems-daily preview --topic buddy-allocator
./bin/systems-daily once                 # generate + email
./bin/systems-daily serve                # send daily at SEND_AT (default 09:00)
./bin/systems-daily serve --now          # send once now, then on schedule
./bin/systems-daily topics               # list topic catalog
```

Cron alternative to `serve`:

```cron
0 9 * * * cd /path/to/systems-daily && /path/to/systems-daily once >> /tmp/systems-daily.log 2>&1
```

## Config

Env vars (or `.env` in cwd). See `.env.example`.

| Variable | Default | Notes |
|----------|---------|-------|
| `LLM_BASE_URL` | `http://localhost:11434/v1` | OpenAI-compatible base URL |
| `LLM_MODEL` | `llama3.2` | Model name |
| `LLM_API_KEY` | `ollama` | Bearer token |
| `SMTP_HOST` | | SMTP host |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | | SMTP user |
| `SMTP_PASS` | | SMTP password / app password |
| `SMTP_FROM` | | From address |
| `SMTP_TO` | | To address(es) |
| `SEND_AT` | `09:00` | Daily send time |
| `TIMEZONE` | local | IANA zone, e.g. `Asia/Kolkata` |
| `TARGET_WORDS_MIN` | `700` | Prompt length guidance |
| `TARGET_WORDS_MAX` | `1200` | Prompt length guidance |
| `HISTORY_PATH` | `data/history.json` | Avoids recent topic repeats |
| `DRY_RUN` | `false` | Generate but do not send |

## Layout

```
cmd/systems-daily/ CLI (cobra)
internal/config/   env config
internal/topics/   curated catalog
internal/llm/      OpenAI-compatible client
internal/content/  prompts + article shaping
internal/email/    SMTP
internal/history/  sent topic log
internal/schedule/ daily runner
internal/app/      once pipeline
```
