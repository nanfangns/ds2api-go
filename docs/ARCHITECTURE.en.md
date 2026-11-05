# DS2API Architecture

Language: [中文](ARCHITECTURE.md) | [English](ARCHITECTURE.en.md)

- `internal/server` assembles OpenAI-compatible routes, administration, health endpoints and the WebUI.
- `internal/httpapi/openai` implements Chat, Responses, Files and Embeddings; `internal/httpapi/admin` manages configuration and accounts.
- `internal/promptcompat` and `internal/prompt` normalize requests and construct text context; `internal/privatecontext` caches context uploads within one process.
- `internal/completionruntime` coordinates sessions, PoW, completion requests and account-safe retries. `internal/assistantturn`, `internal/sse` and `internal/toolcall` process output.
- `internal/auth`, `internal/account`, `internal/accounthealth` and `internal/deepseek/client` handle authentication, the account pool, explicit account restrictions and upstream calls.
- `webui` contains the React admin source and `static/admin` contains its build output.
- Vercel runs the Go gateway and a local device-harvest service using `Dockerfile.vercel` and `scripts/vercel-container-entrypoint.sh`.

See [prompt compatibility](prompt-compatibility.md) for message, file and account ownership, and [API documentation](../API.en.md) for endpoints.
