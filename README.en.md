<p align="center">
  <img src="webui/public/ds2api-favicon.svg" width="128" height="128" alt="DS2API icon" />
</p>

# DS2API

<a href="https://trendshift.io/repositories/24508" target="_blank"><img src="https://trendshift.io/api/badge/repositories/24508" alt="CJackHwang%2Fds2api | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>

[![License](https://img.shields.io/github/license/CJackHwang/ds2api.svg)](LICENSE)
![Stars](https://img.shields.io/github/stars/CJackHwang/ds2api.svg)
![Forks](https://img.shields.io/github/forks/CJackHwang/ds2api.svg)
[![Release](https://img.shields.io/github/v/release/CJackHwang/ds2api?display_name=tag)](https://github.com/CJackHwang/ds2api/releases)
[![Docker](https://img.shields.io/badge/docker-ready-blue.svg)](docs/DEPLOY.en.md)
[![Deploy on Zeabur](https://zeabur.com/button.svg)](https://zeabur.com/templates/L4CFHP)
[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https://github.com/CJackHwang/ds2api)

Language: [中文](README.MD) | [English](README.en.md)

DS2API converts DeepSeek Web chat capability into an OpenAI-compatible API. The core backend is Go-based, with a container for Vercel, and the React WebUI admin panel lives in `webui/` (build output auto-generated to `static/admin` during deployment).

Documentation entry: [Docs Index](docs/README.md) / [Architecture](docs/ARCHITECTURE.en.md) / [API Reference](API.en.md)

## Star History

<a href="https://www.star-history.com/?repos=cjackhwang%2Fds2api&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=cjackhwang/ds2api&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=cjackhwang/ds2api&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=cjackhwang/ds2api&type=date&legend=top-left" />
 </picture>
</a>

> **Important Disclaimer**
>
> This repository is provided for learning, research, personal experimentation, and internal validation only. It does not grant any commercial authorization and comes with no warranty of fitness, stability, or results.
>
> The author and repository maintainers are not responsible for any direct or indirect loss, account suspension, data loss, legal risk, or third-party claims arising from use, modification, distribution, deployment, or reliance on this project.
>
> Do not use this project in ways that violate service terms, agreements, laws, or platform rules. Before any commercial use, review the `LICENSE`, the relevant terms, and confirm that you have the author's written permission.

## Table of Contents

- [Architecture Overview (Summary)](#architecture-overview-summary)
- [Key Capabilities](#key-capabilities)
- [Platform Compatibility Matrix](#platform-compatibility-matrix)
- [Model Support](#model-support)
- [Quick Start](#quick-start)
  - [Option 1: Download Release Binaries](#option-1-download-release-binaries)
  - [Option 2: Docker / GHCR](#option-2-docker--ghcr)
  - [Option 3: Vercel](#option-3-vercel)
  - [Option 4: Local Run](#option-4-local-run)
- [Configuration](#configuration)
- [Authentication Modes](#authentication-modes)
- [Concurrency Model](#concurrency-model)
- [Tool Call Adaptation](#tool-call-adaptation)
- [Local Dev Packet Capture](#local-dev-packet-capture)
- [Documentation Index](#documentation-index)
- [Testing](#testing)
- [Release Artifact Automation (GitHub Actions)](#release-artifact-automation-github-actions)
- [Disclaimer](#disclaimer)

## Architecture Overview (Summary)

The Go gateway normalizes OpenAI Chat and Responses requests and uses DeepSeek Web sessions, PoW, and files. The React admin panel manages configuration and accounts. The Vercel container runs the gateway and device-harvest service. See [prompt compatibility](docs/prompt-compatibility.md).

## Key Capabilities

- OpenAI-compatible models, Chat, Responses, files and embeddings endpoints. Embeddings are unconfigured by default; the built-in mode returns deterministic test vectors only.
- Streaming, tool calls, image input, multiple accounts and the admin panel.
- Local, Docker and Vercel container deployment. Runtime account state in environment-variable mode is not shared durably between Vercel instances.

## Platform Compatibility Matrix

Clients using the OpenAI-compatible protocol can connect by selecting the supported model. Dedicated Claude, Gemini and Ollama routes have been removed.

## Model Support

| Model ID | Upstream channel | Thinking | Search |
| --- | --- | --- | --- |
| `deepseek-flash` | `default` | Enabled by default; use `thinking: false` to disable | Disabled |

The old `deepseek-v4-*`, model aliases and `-nothinking` IDs have been removed. Update existing clients to use `deepseek-flash`.

## Quick Start

### Recommended deployment priority

Recommended order when choosing a deployment method:

1. **Download and run release binaries**: the easiest path for most users because the artifacts are already built.
2. **Docker / GHCR image deployment**: suitable for containerized, orchestrated, or cloud environments.
3. **Vercel deployment**: suitable if you already use Vercel and accept its platform constraints.
4. **Run from source / build locally**: suitable for development, debugging, or when you need to modify the code yourself.

### Universal First Step (all deployment modes)

Use `config.json` as the single source of truth (recommended):

```bash
cp config.example.json config.json
# Edit config.json
```

Recommended per deployment mode:
- Local run: read `config.json` directly
- Docker / Vercel: generate Base64 from `config.json` and inject as `DS2API_CONFIG_JSON`, or paste raw JSON directly

The WebUI admin panel’s “Full configuration template” is loaded from the same `config.example.json`, so updating that file keeps the frontend template in sync.

### Option 1: Download Release Binaries

GitHub Actions automatically builds multi-platform archives on each Release:

```bash
# After downloading the archive for your platform
tar -xzf ds2api_<tag>_linux_amd64.tar.gz
cd ds2api_<tag>_linux_amd64
cp config.example.json config.json
# Edit config.json
./ds2api
```

### Option 2: Docker / GHCR

```bash
# Pull prebuilt image
docker pull ghcr.io/cjackhwang/ds2api:latest

# Or run a pinned version
# docker pull ghcr.io/cjackhwang/ds2api:v3.0.0

# Prepare env file and config file
cp .env.example .env
cp config.example.json config.json

# Start with compose
docker-compose up -d
```

The default `docker-compose.yml` uses `ghcr.io/cjackhwang/ds2api:latest` and maps host port `6011` to container port `5001`. If you want `5001` exposed directly, set `DS2API_HOST_PORT=5001` (or adjust the `ports` mapping).
It also mounts `./config.json` to `/data/config.json` and sets `DS2API_CONFIG_PATH=/data/config.json` by default, which avoids runtime token persistence failures caused by read-only `/app`.

Rebuild after updates: `docker-compose up -d --build`

#### Zeabur One-Click (Dockerfile)

1. Click the “Deploy on Zeabur” button above to deploy.
2. After deployment, open `/admin` and login with `DS2API_ADMIN_KEY` shown in Zeabur env/template instructions.
3. Import / edit config in Admin UI (it will be written and persisted to `/data/config.json`).

Fresh Zeabur volumes can start without `/data/config.json`; DS2API will boot with an empty file-backed config and create the file on the first Admin UI save.

For manual deployment without the template, create a Zeabur GitHub service, keep Root Directory as `/`, build with the repo-root `Dockerfile`, mount a persistent volume at `/data`, set `PORT=5001`, `DS2API_ADMIN_KEY=your-strong-secret`, and `DS2API_CONFIG_PATH=/data/config.json`, then expose HTTP port `5001`. See [docs/DEPLOY.en.md](docs/DEPLOY.en.md#manual-deployment-without-the-template) for the full guide.

Note: when Zeabur builds directly from the repo `Dockerfile`, you do not need to pass `BUILD_VERSION`. The image prefers that build arg when provided, and automatically falls back to the repo-root `VERSION` file when it is absent.

### Option 3: Vercel

1. Fork this repo to your GitHub account
2. Import the project on Vercel
3. Set environment variables (minimum: `DS2API_ADMIN_KEY`; recommended to also set `DS2API_CONFIG_JSON`)
4. Deploy

Recommended first step in repo root:

```bash
cp config.example.json config.json
# Edit config.json
```

Recommended: convert `config.json` to Base64 locally, then paste into `DS2API_CONFIG_JSON` to avoid JSON formatting mistakes:

```bash
base64 < config.json | tr -d '\n'
```

> **Vercel**: The container runs the Go gateway and a local device-harvest service; streaming is served directly by Go. Runtime files and account state are not guaranteed to persist across instances.

For detailed deployment instructions, see the [Deployment Guide](docs/DEPLOY.en.md).

### Option 4: Local Run

**Prerequisites**: Go 1.26+, Node.js `20.19+` or `22.12+` (only if building WebUI locally; CI / Docker builds use Node 24), and npm available; npm 10+ is recommended

```bash
# 1. Clone
git clone https://github.com/CJackHwang/ds2api.git
cd ds2api

# 2. Configure
cp config.example.json config.json
# Edit config.json with your DeepSeek account info and API keys

# 3. Start
go run ./cmd/ds2api
```

Default local URL: `http://127.0.0.1:5001`

The server actually binds to `0.0.0.0:5001`, so devices on the same LAN can usually reach it through your private IP as well.

> **WebUI auto-build**: On first local startup, if the WebUI static directory is missing, DS2API auto-runs `npm ci --prefix webui` (only when dependencies are missing) and `npm run build --prefix webui -- --outDir static/admin --emptyOutDir` (requires Node.js; `DS2API_STATIC_ADMIN_DIR` can override the static directory). You can also build manually: `./scripts/build-webui.sh`

## Configuration

`README` keeps only the onboarding path. Use [config.example.json](config.example.json) as the field template, and see the [deployment guide](docs/DEPLOY.en.md#0-prerequisites) plus [API configuration notes](API.en.md#configuration-best-practice) for full details.

Common fields:

- `keys` / `api_keys`: client API keys; `api_keys` adds `name` and `remark` metadata while `keys` remains compatible.
- `accounts`: managed DeepSeek accounts, supporting `email` or `mobile` login plus proxy/name/remark metadata.
- `runtime`: account concurrency, queueing, and token refresh behavior, hot-reloadable via Admin Settings.
- `auto_delete.mode`: remote session cleanup after each request, supporting `none` / `single` / `all`.
- `current_input_file`: the global context split/upload mode; it is enabled by default and uploads non-system messages as a randomly named context file once the character threshold is reached.
- If you turn off `current_input_file`, requests pass through directly without uploading any split context file.

For the full environment variable list, see [docs/DEPLOY.en.md](docs/DEPLOY.en.md). For auth behavior, see [API.en.md](API.en.md#authentication).

## Authentication Modes

For business endpoints (`/v1/*`), DS2API supports two modes:

| Mode | Description |
| --- | --- |
| **Managed account** | Use a key from `config.keys` via `Authorization: Bearer ...` or `x-api-key`; DS2API auto-selects an account |
| **Direct token** | If the token is not in `config.keys`, DS2API treats it as a DeepSeek token directly |

Optional header `X-Ds2-Target-Account`: Pin a specific managed account (value is email or mobile).
When no target account is pinned, if a completion would end as `429 upstream_empty_output` after the same-account empty-output retry, managed-account mode switches to the next available account, creates a fresh session, and retries the original payload once.

## Concurrency Model

```
Per-account inflight = DS2API_ACCOUNT_MAX_INFLIGHT (default 2)
Recommended concurrency = account_count × per_account_inflight
Queue limit = DS2API_ACCOUNT_MAX_QUEUE (default = recommended concurrency)
429 threshold = inflight + queue ≈ account_count × 4
```

- When inflight slots are full, requests enter a waiting queue — **no immediate 429**
- 429 is returned only when total load exceeds inflight + queue capacity; current responses do not include `Retry-After`
- Completion empty-output 429s first get the same-account compensation retry; managed-account mode also tries one alternate-account fresh retry before returning the final 429
- `GET /admin/queue/status` returns real-time concurrency state

## Tool Call Adaptation

Tool definitions remain in the live prompt. The compatibility layer parses model output and returns OpenAI tool-call structures. See [prompt compatibility](docs/prompt-compatibility.md) for context and file-account ownership.

## Local Dev Packet Capture

This is for debugging issues such as Responses reasoning streaming and tool-call handoff. When enabled, DS2API stores the latest N DeepSeek conversation payload pairs (request body + upstream response body), defaulting to 20 entries with auto-eviction; each response body is capped at 5 MB by default.

Enable example:

```bash
DS2API_DEV_PACKET_CAPTURE=true \
DS2API_DEV_PACKET_CAPTURE_LIMIT=20 \
go run ./cmd/ds2api
```

Inspect/clear (Admin JWT required):

- `GET /admin/dev/captures`: list captured items (newest first)
- `DELETE /admin/dev/captures`: clear captured items
- `GET /admin/dev/raw-samples/query?q=keyword&limit=20`: search current in-memory captures by prompt keyword and group `completion + continue` by `chat_session_id`
- `POST /admin/dev/raw-samples/save`: persist a selected capture chain as `tests/raw_stream_samples/<sample-id>/`

Response fields include:

- `request_body`: full payload sent to DeepSeek
- `response_body`: concatenated raw upstream stream body text
- `response_truncated`: whether body-size truncation happened

The save endpoint can target a chain by `query`, `chain_key`, or `capture_id`. Example:

```json
{"query":"Guangzhou weather","sample_id":"gz-weather-from-memory"}
```

## Documentation Index

| Document | Description |
| --- | --- |
| [API.md](API.md) / [API.en.md](API.en.md) | API reference with request/response examples |
| [DEPLOY.md](docs/DEPLOY.md) / [DEPLOY.en.md](docs/DEPLOY.en.md) | Deployment guide (local/Docker/Vercel/systemd) |
| [CONTRIBUTING.md](docs/CONTRIBUTING.md) / [CONTRIBUTING.en.md](docs/CONTRIBUTING.en.md) | Contributing guide |
| [TESTING.md](docs/TESTING.md) | Testsuite guide |

## Testing

For the full testing guide, see [docs/TESTING.md](docs/TESTING.md).

Quick commands:

```bash
# Local PR gates
./scripts/lint.sh
./tests/scripts/check-refactor-line-gate.sh
./tests/scripts/run-unit-all.sh
npm run build --prefix webui

# Live end-to-end tests (real accounts, full request/response logs)
./tests/scripts/run-live.sh
```

## Release Artifact Automation (GitHub Actions)

Workflow: `.github/workflows/release-artifacts.yml`

- **Trigger**: by default only on GitHub Release `published`; you can also run it manually via `workflow_dispatch` and pass `release_tag` to rerun / backfill
- **Outputs**: multi-platform binary archives (`linux/amd64`, `linux/arm64`, `linux/armv7`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `windows/arm64`), Linux Docker image export tarballs, and `sha256sums.txt`
- **Container publishing**: GHCR only (`ghcr.io/cjackhwang/ds2api`)
- **Each binary archive includes**: the `ds2api` executable, `static/admin`, `config.example.json`, `.env.example`, `README.MD`, `README.en.md`, and `LICENSE`

## Disclaimer

This project is built through reverse engineering and is provided for learning, research, personal experimentation, and internal validation only. No commercial authorization is granted, and no warranty of stability, fitness, or results is provided.
The author and repository maintainers are not responsible for any direct or indirect loss, account suspension, data loss, legal risk, or third-party claims arising from use, modification, distribution, deployment, or reliance on this project.

Do not use this project in ways that violate service terms, agreements, laws, or platform rules. Before any commercial use, review the `LICENSE`, the relevant terms, and confirm that you have the author's written permission.
