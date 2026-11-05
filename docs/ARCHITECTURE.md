# DS2API 架构

语言 / Language: [中文](ARCHITECTURE.md) | [English](ARCHITECTURE.en.md)

## 主要模块

- `internal/server` 装配 OpenAI 兼容路由、管理接口、健康检查和 WebUI。
- `internal/httpapi/openai` 实现 Chat、Responses、Files 和 Embeddings 协议入口；`internal/httpapi/admin` 管理账号和配置。
- `internal/promptcompat` 与 `internal/prompt` 归一化请求和构建文本上下文，`internal/privatecontext` 负责同进程上下文文件缓存。
- `internal/completionruntime` 协调会话、PoW、完成请求和安全的账号重试；`internal/assistantturn`、`internal/sse` 与 `internal/toolcall` 处理输出。
- `internal/auth`、`internal/account`、`internal/accounthealth` 和 `internal/deepseek/client` 处理认证、账号池、明确的账号限制及上游通信。
- `webui` 是 React 管理台源码；`static/admin` 是构建输出。
- Vercel 使用 `Dockerfile.vercel` 和 `scripts/vercel-container-entrypoint.sh` 运行 Go 网关和本机设备采集服务。

消息、文件与账号归属的详细规则见 [上下文兼容说明](prompt-compatibility.md)，接口见 [API 文档](../API.md)。
