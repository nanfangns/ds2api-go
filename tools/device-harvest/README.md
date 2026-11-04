# device-harvest

Sidecar that produces **real Shumei device tokens** (`device_id`, `"B" + SMID`)
for DeepSeek logins.

DeepSeek's login API rejects requests whose `device_id` is not a genuine
browser fingerprint (`biz_code 11`, `RISK_DEVICE_DETECTED`). This service loads
`https://chat.deepseek.com/sign_in` in a real headed Chromium, waits for the
Shumei SDK, and returns `window.SMSdk.getDeviceId()`. A fresh browser context is
used per token; a token takes ~0.8 s to become ready and ~2 s end-to-end.

**It never submits a login.** Every request to `/api/v0/users/*` is intercepted
and aborted before it leaves the browser, so harvesting cannot consume, mute or
ban an account. The requests are only *generated* — tokens are device
fingerprints, so logs always truncate them (first 6 chars).

## Quick start

### Local (needs Node 20+, Playwright Chromium, Xvfb)

```bash
npm install                                   # once
xvfb-run -a node harvest.mjs serve --port 8090

curl -s http://127.0.0.1:8090/healthz
curl -s -X POST http://127.0.0.1:8090/harvest \
  -H 'Content-Type: application/json' -d '{"account_id":"example"}'
# {"device_id":"BIAbJY..."}
```

Headed Chromium requires an X display; without one the service exits with a
hint to use `xvfb-run` (or the container).

### Docker

```bash
docker build -t ds2api-device-harvest tools/device-harvest
docker run -d --name device-harvest -p 127.0.0.1:8090:8090 -e HARVEST_TOKEN=change-me ds2api-device-harvest
```

The image entry point runs the service under Xvfb (headed Chromium needs a
display), so no extra flags are required; it also keeps `xvfb-run` off PID 1,
which would otherwise hang waiting for Xvfb's readiness signal.

Point DS2API at the reachable service via its environment:

```dotenv
DEVICE_HARVEST_URL=http://127.0.0.1:8090
DEVICE_HARVEST_TOKEN=change-me
```

Leaving `DEVICE_HARVEST_URL` unset keeps DS2API's existing login fallback when
an account has no configured `device_id`. A Vercel deployment needs an
independently hosted, authenticated service; localhost refers to the function
itself, not your own computer. Vercel's environment-backed configuration only
caches harvested tokens in the current instance.

## HTTP contract

| Method | Path       | Description |
|--------|------------|-------------|
| `GET`  | `/healthz` | `200 {"status":"ok","browser":"up","active":0,"queued":0,...}`; `503` when Chromium is down |
| `POST` | `/harvest` | body `{"account_id":"..."}` → `200 {"device_id":"B..."}` |

* `account_id` is only used for logging/queue bookkeeping; the token is
  device-level.
* When `HARVEST_TOKEN` is set, `POST /harvest` requires a matching
  `X-Harvest-Token` header, otherwise `401`.
* `400` invalid JSON, `405` wrong method, `502` harvest failure, `503` queue
  full / browser down.
* `/harvest` is idempotent in the sense that a failure leaves no side effect;
  the caller may retry once.

## Environment variables

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8090` | listen port (overridden by `--port`) |
| `HARVEST_TOKEN` | empty | optional shared secret checked against `X-Harvest-Token` |
| `HARVEST_CONCURRENCY` | `2` | parallel browser contexts (1–8) |
| `HARVEST_MAX_QUEUE` | `64` | requests allowed to wait before `503` |

## CLI mode (optional batch pre-seed)

```bash
node harvest.mjs cli \
  --input accounts.csv \
  --per-account 1 \
  --output accounts.device-ids.csv \
  --state accounts.device-ids.csv.state.json \
  [--limit 20] [--include-discarded]
```

* Input: `email,password[,discarded|device_id]`; the legacy two-column layout
  still works, and a trailing `discarded=true` row is skipped unless
  `--include-discarded` is given.
* Output: `email,password,device_id` with multiple tokens joined by `|` (the
  pool importer keeps the first token).
* The state file records already-harvested accounts, so an interrupted run can
  be resumed by re-running the same command; both state and output are flushed
  after every account.
* Exit code is `1` when any account failed; failures are retried on the next
  run.

## Notes

* Use one token per account (1:1). The community evidence is that reusing a
  token across many accounts raises association/risk-control signals, even
  though the token is technically device-level.
* A `biz_code 11` response on login means the token was rejected; the caller
  clears it, re-harvests once, and retries. This service itself has no token
  state.
* Under Xvfb the window is "headed" but off-screen; `--no-sandbox`,
  `--disable-dev-shm-usage` and `--disable-blink-features=AutomationControlled`
  are passed to Chromium. The page's `navigator.webdriver` is masked and service
  workers are blocked so the route interception cannot be bypassed.
