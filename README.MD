<p align="center">
  <img src="webui/public/ds2api-favicon.svg" width="128" height="128" alt="DS2API icon" />
</p>

# DS2API

<a href="https://trendshift.io/repositories/24508" target="_blank"><img src="https://trendshift.io/api/badge/repositories/24508" alt="CJackHwang%2Fds2api | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>

[![License](https://img.shields.io/github/license/CJackHwang/ds2api.svg)](LICENSE)
![Stars](https://img.shields.io/github/stars/CJackHwang/ds2api.svg)
![Forks](https://img.shields.io/github/forks/CJackHwang/ds2api.svg)
[![Release](https://img.shields.io/github/v/release/CJackHwang/ds2api?display_name=tag)](https://github.com/CJackHwang/ds2api/releases)
[![Docker](https://img.shields.io/badge/docker-ready-blue.svg)](docs/DEPLOY.md)

[![Deploy on Zeabur](https://zeabur.com/button.svg)](https://zeabur.com/templates/L4CFHP)
[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https://github.com/CJackHwang/ds2api)

语言 / Language: [中文](README.MD) | [English](README.en.md)

将 DeepSeek Web 对话能力转换为 OpenAI 兼容 API。核心后端以 **Go** 实现，Vercel 使用容器运行 Go 网关和设备采集器，前端为 React WebUI 管理台（源码在 `webui/`，部署时自动构建到 `static/admin`）。

文档入口：[文档导航](docs/README.md) / [架构说明](docs/ARCHITECTURE.md) / [接口文档](API.md)

【感谢Linux.do社区及GitHub社区各位开发者对项目的支持与贡献】

本分支参考并移植了 [Whale2API](https://github.com/tangsong404/whale2api) 的模型、上下文、图片输入、工具调用、账号状态和设备令牌相关实现。

## Star History

<a href="https://www.star-history.com/?repos=cjackhwang%2Fds2api&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=cjackhwang/ds2api&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=cjackhwang/ds2api&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=cjackhwang/ds2api&type=date&legend=top-left" />
 </picture>
</a>

> **重要免责声明**
>
> 本仓库仅供学习、研究、个人实验和内部验证使用，不提供任何形式的商业授权、适用性保证或结果保证。
>
> 作者及仓库维护者不对因使用、修改、分发、部署或依赖本项目而产生的任何直接或间接损失、账号封禁、数据丢失、法律风险或第三方索赔负责。
>
> 请勿将本项目用于违反服务条款、协议、法律法规或平台规则的场景。商业使用前请自行确认 `LICENSE`、相关协议以及你是否获得了作者的书面许可。

## 目录

- [架构概览（摘要）](#架构概览摘要)
- [核心能力](#核心能力)
- [平台兼容矩阵](#平台兼容矩阵)
- [模型支持](#模型支持)
- [快速开始](#快速开始)
  - [方式一：下载 Release 构建包](#方式一下载-release-构建包)
  - [方式二：Docker 运行](#方式二docker-运行)
  - [方式三：Vercel 部署](#方式三vercel-部署)
  - [方式四：本地源码运行](#方式四本地源码运行)
- [配置说明](#配置说明)
- [鉴权模式](#鉴权模式)
- [并发模型](#并发模型)
- [Tool Call 适配](#tool-call-适配)
- [本地开发抓包工具](#本地开发抓包工具)
- [文档索引](#文档索引)
- [测试](#测试)
- [Release 自动构建（GitHub Actions）](#release-自动构建github-actions)
- [免责声明](#免责声明)

## 架构概览（摘要）

OpenAI Chat / Responses 请求经 Go 网关归一化后，使用 DeepSeek 网页会话、PoW 和文件接口。React 管理台负责配置和账号管理；Vercel 容器运行网关与设备采集服务。详细流程见 [上下文兼容说明](docs/prompt-compatibility.md)。

## 核心能力

- OpenAI 兼容的模型查询、Chat、Responses、文件接口及 Embeddings 接口；Embeddings 默认未配置，内置模式仅生成确定性的测试向量。
- 流式与非流式输出、工具调用、图片输入、多账号和管理台。
- 支持本地、Docker 和 Vercel 容器部署；Vercel 环境变量模式下，运行时账号状态不会跨实例持久共享。

## 平台兼容矩阵

使用 OpenAI 兼容协议并允许指定模型 ID 的客户端可以接入。Claude、Gemini 和 Ollama 专用协议入口已移除。

## 模型支持

| 模型 ID | 上游通道 | 默认思考 | 搜索 |
| --- | --- | --- | --- |
| `deepseek-flash` | `default` | 开启，可通过 `thinking: false` 关闭 | 关闭 |

旧的 `deepseek-v4-*`、模型别名和 `-nothinking` 模型 ID 已移除，使用旧名称的客户端需要改为 `deepseek-flash`。

## 快速开始

### 部署方式优先级建议

推荐按以下顺序选择部署方式：

1. **下载 Release 构建包运行**：最省事，产物已编译完成，最适合大多数用户。
2. **Docker / GHCR 镜像部署**：适合需要容器化、编排或云环境部署。
3. **Vercel 部署**：适合已有 Vercel 环境且接受其平台约束的场景。
4. **本地源码运行 / 自行编译**：适合开发、调试或需要自行修改代码的场景。

### 通用第一步（所有部署方式）

把 `config.json` 作为唯一配置源（推荐做法）：

```bash
cp config.example.json config.json
# 编辑 config.json
```

后续部署建议：
- 本地运行：直接读取 `config.json`
- Docker / Vercel：由 `config.json` 生成 `DS2API_CONFIG_JSON`（Base64）注入环境变量，也可以直接写原始 JSON

WebUI 管理台里的“全量配置模板”也直接复用同一份 `config.example.json`，所以更新示例文件后，前端模板会自动保持一致。

### 方式一：下载 Release 构建包

每次发布 Release 时，GitHub Actions 会自动构建多平台二进制包：

```bash
# 下载对应平台的压缩包后
tar -xzf ds2api_<tag>_linux_amd64.tar.gz
cd ds2api_<tag>_linux_amd64
cp config.example.json config.json
# 编辑 config.json
./ds2api
```

### 方式二：Docker 运行

```bash
# 1. 准备环境变量和配置文件
cp .env.example .env
cp config.example.json config.json

# 2. 编辑 .env（至少设置 DS2API_ADMIN_KEY；如需修改宿主机端口，可额外设置 DS2API_HOST_PORT）
#    DS2API_ADMIN_KEY=请替换为强密码

# 3. 启动
docker-compose up -d

# 4. 查看日志
docker-compose logs -f
```

默认 `docker-compose.yml` 会把宿主机 `6011` 映射到容器内的 `5001`。如果你希望直接对外暴露 `5001`，请设置 `DS2API_HOST_PORT=5001`（或者手动调整 `ports` 配置）。
同时默认把 `./config.json` 挂载到容器 `/data/config.json`，并设置 `DS2API_CONFIG_PATH=/data/config.json`，用于避免 `/app` 只读导致运行时 token 持久化失败。
镜像会预创建 `/data` 并授权给非 root 的 `ds2api` 用户；如果使用单文件 bind mount，请确保宿主机 `config.json` 对容器用户可读写，例如 `chmod 644 config.json`。

更新镜像：`docker-compose up -d --build`

#### Zeabur 一键部署（Dockerfile）

1. 点击上方 “Deploy on Zeabur” 按钮，一键部署。
2. 部署完成后访问 `/admin`，使用 Zeabur 环境变量/模板指引中的 `DS2API_ADMIN_KEY` 登录。
3. 在管理台导入/编辑配置（会写入并持久化到 `/data/config.json`）。

Zeabur 首次空卷启动时可以没有 `/data/config.json`；DS2API 会先使用空的文件模式配置启动，并在管理台首次保存时创建该文件。

不依赖模板手动部署时，在 Zeabur 中选择 GitHub 仓库服务，Root Directory 保持 `/`，使用仓库根目录 `Dockerfile` 构建；添加持久卷 `/data`，设置 `PORT=5001`、`DS2API_ADMIN_KEY=你的强密钥`、`DS2API_CONFIG_PATH=/data/config.json`，然后暴露 HTTP 端口 `5001`。更完整步骤见 [docs/DEPLOY.md](docs/DEPLOY.md#不使用模板手动部署)。

说明：Zeabur 使用仓库内 `Dockerfile` 直接构建时，不需要额外传入 `BUILD_VERSION`；镜像会优先读取该构建参数，未提供时自动回退到仓库根目录的 `VERSION` 文件。

### 方式三：Vercel 部署

1. Fork 仓库到自己的 GitHub
2. 在 Vercel 上导入项目
3. 配置环境变量（最少设置 `DS2API_ADMIN_KEY`；推荐同时设置 `DS2API_CONFIG_JSON`）
4. 部署

建议先在仓库目录复制模板并填写：

```bash
cp config.example.json config.json
# 编辑 config.json
```

推荐：先本地把 `config.json` 转成 Base64，再粘贴到 `DS2API_CONFIG_JSON`，避免 JSON 格式错误：

```bash
base64 < config.json | tr -d '\n'
```

> **Vercel 说明**：使用 Vercel Services 容器运行 Go 网关，并在容器内运行仅监听本机的设备令牌采集器；流式响应由 Go 直接处理。Vercel Services 目前为 Beta，容器镜像存储和运行资源可能产生费用，冷启动也可能较慢。运行时文件和采集结果不保证跨实例持久保存。

详细部署说明请参阅 [部署指南](docs/DEPLOY.md)。

### 方式四：本地源码运行

**前置要求**：Go 1.26+，Node.js `20.19+` 或 `22.12+`（仅在需要构建 WebUI 时；CI / Docker 构建使用 Node 24）；同时确保 `npm` 可用，建议 `npm 10+`

```bash
# 1. 克隆仓库
git clone https://github.com/CJackHwang/ds2api.git
cd ds2api

# 2. 配置
cp config.example.json config.json
# 编辑 config.json，填入你的 DeepSeek 账号信息和 API key

# 3. 启动
go run ./cmd/ds2api
```

默认本地访问地址：`http://127.0.0.1:5001`

服务实际绑定：`0.0.0.0:5001`，因此同一局域网设备通常也可以通过你的内网 IP 访问。

> **WebUI 自动构建**：本地首次启动时，若 WebUI 静态目录不存在，会自动尝试执行 `npm ci --prefix webui`（仅在缺少依赖时）和 `npm run build --prefix webui -- --outDir static/admin --emptyOutDir`（需要本机有 Node.js 和 npm；静态目录可用 `DS2API_STATIC_ADMIN_DIR` 覆盖）。你也可以手动构建：`./scripts/build-webui.sh`

## 配置说明

`README` 只保留快速入口，完整字段请以 [config.example.json](config.example.json) 为模板，并参考 [部署指南](docs/DEPLOY.md#0-前置要求) 与 [API 配置最佳实践](API.md#配置最佳实践)。

常用字段：

- `keys` / `api_keys`：客户端访问密钥，`api_keys` 支持 `name` 与 `remark` 元信息，`keys` 继续兼容。
- `accounts`：DeepSeek 托管账号，支持 `email` 或 `mobile` 登录，可配置代理、名称和备注。
- `accounts[].device_id`：可选的账号设备令牌。Vercel 容器默认启动本机采集服务，也可通过 `DEVICE_HARVEST_URL` 对接独立服务（见 `tools/device-harvest`）；环境变量模式下运行时采集结果只保存在当前实例内。
- `runtime`：账号并发、队列与 token 刷新策略，可通过 Admin Settings 热更新。
- `auto_delete.mode`：请求结束后的远端会话清理策略，支持 `none` / `single` / `all`。
- `current_input_file`：全局生效的上下文拆分上传策略；默认开启且阈值为 `0`，触发时将非系统消息合并上传为随机命名的上下文文件。
- 如果关闭 `current_input_file`，请求会直接透传，不上传拆分上下文文件。
- `thinking_injection`：默认开启；在系统指令中追加思考增强提示词，提高高强度推理与工具调用前的思考稳定性；`prompt` 留空时使用内置默认提示词。

`deepseek-flash` 使用上游 `default` 通道，支持图片数据和公网 HTTP(S) 图片 URL。上下文预检默认按 Whale 的 896,000 展示值与 2,850,000 本地估算阈值计算；这是估算保护，不保证上游一定接受相同长度。可通过 `DS2API_USER_FACING_CONTEXT_LIMIT_TOKENS` 和 `DS2API_LOGICAL_CONTEXT_LIMIT_TOKENS` 调整。

上游明确返回账号禁言或封禁时，账号会被暂时移出号池，管理台可查看状态并手动恢复；已知禁言到期后可自动重新使用，未知到期时间时一小时后重试。文件模式将状态写入配置文件，环境变量模式只在当前实例内保存，重启或切换实例可能丢失。

环境变量完整列表见 [部署指南](docs/DEPLOY.md)，接口鉴权规则见 [API.md](API.md#鉴权规则)。

## 鉴权模式

调用业务接口（`/v1/*`）时支持两种模式：

| 模式 | 说明 |
| --- | --- |
| **托管账号模式** | `Bearer` 或 `x-api-key` 传入 `config.keys` 中的 key，由服务自动轮询选择账号 |
| **直通 token 模式** | 传入 token 不在 `config.keys` 中时，直接作为 DeepSeek token 使用 |

可选请求头 `X-Ds2-Target-Account`：指定使用某个托管账号（值为 email 或 mobile）。
如果指定账号不存在，或者当前管理账号队列已满，请求会返回 `429`；当前 `429` 不附带 `Retry-After` 头。若账号存在但登录/刷新失败，则返回对应的鉴权错误。
未指定目标账号时，如果 completion 因上游 thinking-only 空输出在同账号补偿重试后仍将返回 `429 upstream_empty_output`，托管账号模式会自动切到下一个可用账号，新建 session，并用原始 payload 再 fresh retry 一次。

## 并发模型

```
每账号可用并发 = DS2API_ACCOUNT_MAX_INFLIGHT（默认 2）
建议并发值 = 账号数量 × 每账号并发上限
等待队列上限 = DS2API_ACCOUNT_MAX_QUEUE（默认 = 建议并发值）
429 阈值 = in-flight + 等待队列 ≈ 账号数量 × 4
```

- 当 in-flight 槽位满时，请求进入等待队列，**不会立即 429**
- 超出总承载上限后才返回 `429 Too Many Requests`，当前响应不附带 `Retry-After`
- completion 空输出类 429 会先做同账号补偿重试；托管账号模式还会在最终返回 429 前切到另一个可用账号 fresh retry 一次
- `GET /admin/queue/status` 返回实时并发状态

## Tool Call 适配

工具定义保留在实时提示中，模型输出中的工具调用由兼容层解析并转换为 OpenAI 结构。上下文和文件归属规则见 [上下文兼容说明](docs/prompt-compatibility.md)。

## 本地开发抓包工具

用于定位「responses 思考流/工具调用」等问题。开启后会自动记录最近 N 条 DeepSeek 对话上游请求体与响应体（默认 20 条，超出自动淘汰；单条响应体默认最多记录 5 MB）。

启用示例：

```bash
DS2API_DEV_PACKET_CAPTURE=true \
DS2API_DEV_PACKET_CAPTURE_LIMIT=20 \
go run ./cmd/ds2api
```

查询/清空（需 Admin JWT）：

- `GET /admin/dev/captures`：查看抓包列表（最新在前）
- `DELETE /admin/dev/captures`：清空抓包
- `GET /admin/dev/raw-samples/query?q=关键词&limit=20`：按问题关键词查询当前内存抓包，并按 `chat_session_id` 归并 `completion + continue` 链
- `POST /admin/dev/raw-samples/save`：把命中的某条抓包链保存为 `tests/raw_stream_samples/<sample-id>/` 回放样本

返回字段包含：

- `request_body`：发送给 DeepSeek 的完整请求体
- `response_body`：上游返回的原始流式内容拼接文本
- `response_truncated`：是否触发单条大小截断

保存接口支持用 `query`、`chain_key` 或 `capture_id` 选中目标。例如：

```json
{"query":"广州天气","sample_id":"gz-weather-from-memory"}
```

## 文档索引

| 文档 | 说明 |
| --- | --- |
| [API.md](API.md) / [API.en.md](API.en.md) | API 接口文档（含请求/响应示例） |
| [DEPLOY.md](docs/DEPLOY.md) / [DEPLOY.en.md](docs/DEPLOY.en.md) | 部署指南（本地/Docker/Vercel/systemd） |
| [CONTRIBUTING.md](docs/CONTRIBUTING.md) / [CONTRIBUTING.en.md](docs/CONTRIBUTING.en.md) | 贡献指南 |
| [TESTING.md](docs/TESTING.md) | 测试集使用指南 |

## 测试

详细测试指南请参阅 [docs/TESTING.md](docs/TESTING.md)。

### 快速测试命令

```bash
# 本地 PR 门禁
./scripts/lint.sh
./tests/scripts/check-refactor-line-gate.sh
./tests/scripts/run-unit-all.sh
npm run build --prefix webui

# 端到端全链路测试（真实账号，生成完整请求/响应日志）
./tests/scripts/run-live.sh
```

## Release 自动构建（GitHub Actions）

工作流文件：`.github/workflows/release-artifacts.yml`

- **触发条件**：默认仅在 GitHub Release `published` 时自动触发；也支持在 Actions 页面手动 `workflow_dispatch`，并填写 `release_tag` 复跑/补发
- **构建产物**：多平台二进制包（`linux/amd64`、`linux/arm64`、`linux/armv7`、`darwin/amd64`、`darwin/arm64`、`windows/amd64`、`windows/arm64`）、Linux Docker 镜像导出包 + `sha256sums.txt`
- **容器镜像发布**：仅推送到 GHCR（`ghcr.io/cjackhwang/ds2api`）
- **每个二进制压缩包包含**：`ds2api` 可执行文件、`static/admin`、`config.example.json`、`.env.example`、`README.MD`、`README.en.md`、`LICENSE`

## 免责声明

本项目基于逆向方式实现，仅供学习、研究、个人实验和内部验证使用，不提供任何商业授权、稳定性保证或可用性保证。
作者及仓库维护者不对因使用、修改、分发、部署或依赖本项目而产生的任何直接或间接损失、账号封禁、数据丢失、法律风险或第三方索赔负责。

请勿将本项目用于违反服务条款、协议、法律法规或平台规则的场景。商业使用前请自行确认 `LICENSE`、相关协议以及你是否获得了作者的书面许可。
