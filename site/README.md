# Static site output

`systems-daily` writes HTML into `site/public` each run:

| Path | Role |
|------|------|
| `/` or `/index.html` | Latest note (same as today) |
| `/today/` | Always the current write-up |
| `/d/YYYY-MM-DD/` | Dated page; removed after `SITE_WINDOW_DAYS` (default 7) |

## Local preview

```bash
./bin/systems-daily preview --topic watchdogs
python3 -m http.server 8080 --directory site/public
# open http://127.0.0.1:8080/today/
```

## Vercel

1. Import this repo.
2. Framework preset: **Other**.
3. **Output Directory**: `site/public` (or set Root Directory to `site` and output `public`).
4. After each `once` run, commit/push `site/public` (or deploy via CI/hook).

Env for the generator (machine that runs cron):

```bash
SITE_OUT_DIR=site/public
SITE_BASE_URL=https://your-deployment.vercel.app
SITE_WINDOW_DAYS=7
```
