# chat2api

秽土转生 `free-gpt3.5-2api` -> `chat2api`.

把 ChatGPT Web 侧能力转换为兼容 OpenAI 风格的 HTTP API。

## 支持能力

- `POST /v1/chat/completions`：兼容 Chat Completions，请求支持普通 JSON 与 stream。
- `POST /v1/responses`：兼容 Responses API 文本链路，请求支持普通 JSON 与 stream。
- `GET /v1/accTokens`：查看配置账号池可用数量。
- 本地 `sk-` auth key：使用配置文件中的 `chatgpts` 账号池请求上游。
- 直传 `at-` access token：使用 `Authorization: Bearer at-<real_access_token>`，跳过账号池，直接用 `at-` 后面的真实 access token 请求上游。

说明：当前版本不再做模型映射，`model` 会直接传给上游；请求中未传 `model` 时默认使用 `auto`。

## 配置

服务读取 `conf/app.<ENV>.yaml`，`ENV` 默认为 `dev`，因此本地默认读取 `conf/app.dev.yaml`。

仓库提供了配置模板 `conf/app.demo.yaml`。本地运行时可以复制为 `conf/app.dev.yaml`；Docker Compose 运行时可以复制为 `.chat2api/conf/app.dev.yaml`，因为 compose 会把 `.chat2api/conf` 映射到容器内的 `/app/conf`。

当前版本的业务配置以 YAML 文件为准，环境变量只用于选择配置文件：

| 环境变量 | 默认值 | 作用 |
| --- | --- | --- |
| `ENV` | `dev` | 决定读取哪个配置文件，例如 `ENV=prod` 会读取 `conf/app.prod.yaml`。 |

常见配置文件对应关系：

| 启动方式 | 读取文件 |
| --- | --- |
| `go run ./cmd` | `conf/app.dev.yaml` |
| `ENV=test go run ./cmd` | `conf/app.test.yaml` |
| `ENV=prod go run ./cmd` | `conf/app.prod.yaml` |

```yaml
log_level: debug
log_path: logs
log_file: app.dev.log
bind: 127.0.0.1
port: 3040

auth:
  access_tokens:
    - sk-your-local-key

proxy: http://127.0.0.1:7890
chatgpt_base_url: https://chatgpt.com

chatgpts:
  - id_token: optional_id_token
    access_token: real_access_token
    refresh_token: optional_refresh_token
    account_id: optional_account_id
    last_refresh: ""
    email: optional_email
    type: codex
    expired: ""
    proxy: ""
```

关键规则：

- `auth.access_tokens` 保存裸 token，不要写 `Bearer`；请求时仍使用标准的 `Authorization: Bearer <token>`。
- 如果 `auth.access_tokens` 为空，服务启动时会随机生成一个 `sk-` token，写回配置文件，并在日志中打印 `current auth: ...`。
- `chatgpts` 是账号池配置，每个账号只有 `access_token` 是必要配置；`proxy`、`id_token`、`refresh_token`、`email` 等字段都是可选字段。
- `chatgpts[].access_token` 是账号池的真实上游 access token。通过本地 `sk-` key 请求时会从这里选择账号。
- 代理优先级为账号代理优先：`chatgpts[].proxy` 不为空时使用账号代理；为空时回退到全局 `proxy`。
- `chatgpt_base_url` 为空时默认使用 `https://chatgpt.com`。

## 运行

本地运行：

```bash
go run ./cmd
```

指定环境运行，例如读取 `conf/app.prod.yaml`：

```bash
ENV=prod go run ./cmd
```

Docker Compose：

```bash
docker compose up -d
```

Vercel 运行时不会写入配置文件，也不会默认读取仓库里的 `conf/app.dev.yaml`，避免把本地代理或本地账号配置带到云端。请在 Vercel 环境变量中配置：

| 环境变量 | 作用 |
| --- | --- |
| `AUTH_TOKENS` | 本地 API key，多个值用逗号、分号或换行分隔 |
| `CHATGPT_ACCESS_TOKENS` | 上游 ChatGPT access token，多个值用逗号、分号或换行分隔 |
| `PROXY` | 可选，全局代理 |
| `CHATGPT_BASE_URL` | 可选，默认 `https://chatgpt.com` |
| `LOG_LEVEL` | 可选，默认 `debug` |
| `VERCEL_CONFIG_FILE` | 可选，显式指定要读取的 YAML 配置文件路径，例如 `conf/app.prod.yaml` |

默认 `compose.yaml` 将容器 `3040` 端口映射到宿主机 `7846`，并映射本地配置与日志目录：

```yaml
volumes:
  - .chat2api/conf:/app/conf
  - .chat2api/logs:/app/logs
```

容器内工作目录是 `/app`，因此默认会读取 `/app/conf/app.dev.yaml`，也就是宿主机的 `.chat2api/conf/app.dev.yaml`。如需让容器读取其他环境配置，可以在 `compose.yaml` 中增加 `ENV`：

```yaml
environment:
  - ENV=prod
```

此时容器会读取宿主机映射进去的 `.chat2api/conf/app.prod.yaml`。

## 接口示例

下面示例以本地开发配置 `127.0.0.1:3040` 为例。

### 查看账号池

```bash
curl http://127.0.0.1:3040/v1/accTokens \
  -H 'Authorization: Bearer sk-your-local-key'
```

返回中的 `count` 是账号池账号数量，`canUseCount` 是当前可用账号数量。

### Chat Completions

使用配置账号池：

```bash
curl http://127.0.0.1:3040/v1/chat/completions \
  -H 'Authorization: Bearer sk-your-local-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"auto","messages":[{"role":"user","content":"ping"}]}'
```

直传真实 access token，跳过账号池：

```bash
curl http://127.0.0.1:3040/v1/chat/completions \
  -H 'Authorization: Bearer at-<real_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{"model":"auto","messages":[{"role":"user","content":"ping"}]}'
```

流式返回：

```bash
curl http://127.0.0.1:3040/v1/chat/completions \
  -H 'Authorization: Bearer sk-your-local-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"auto","stream":true,"messages":[{"role":"user","content":"ping"}]}'
```

### Responses

普通文本请求：

```bash
curl http://127.0.0.1:3040/v1/responses \
  -H 'Authorization: Bearer sk-your-local-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"auto","input":"ping"}'
```

带 instructions：

```bash
curl http://127.0.0.1:3040/v1/responses \
  -H 'Authorization: Bearer sk-your-local-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"auto","instructions":"用中文回答","input":"ping"}'
```

流式返回：

```bash
curl http://127.0.0.1:3040/v1/responses \
  -H 'Authorization: Bearer sk-your-local-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"auto","stream":true,"input":"ping"}'
```

当前 Go 版本的 `/v1/responses` 仅实现文本链路；`image_generation` 工具会返回未实现错误。

## 错误排查

- `401 Incorrect API key`：检查请求头是否为 `Authorization: Bearer sk-your-local-key`，以及配置里的 `auth.access_tokens` 是否保存裸 token。
- `turnstile token is required` 或 `turnstile token failed`：上游要求 Turnstile 校验，需确认账号 token、代理和上游访问环境是否可用。
- 账号池不可用：检查 `chatgpts[].access_token` 是否为空、是否过期，以及账号是否处于冷却时间。
- 代理不生效：先检查账号自己的 `chatgpts[].proxy`，它会优先于全局 `proxy`。

## 参考项目

- https://github.com/aurora-develop/aurora
- https://github.com/xqdoo00o/ChatGPT-to-API
- https://github.com/basketikun/chatgpt2api

## Powered By

- codex
- [aurorax-neo](https://github.com/aurorax-neo)

## Friend Links

- [linux.do](https://linux.do/)
- [xiaozhou26](https://github.com/xiaozhou26)

## Sponsor

<a href="https://edgeone.ai/?from=github"><img width="200" src="https://edgeone.ai/media/34fe3a45-492d-4ea4-ae5d-ea1087ca7b4b.png"></a>

CDN acceleration and security protection for this project are sponsored by Tencent EdgeOne.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=aurorax-neo/chat2api&type=Date)](https://star-history.com/#aurorax-neo/chat2api&Date)
