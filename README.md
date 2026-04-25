# Execraft

**面向 Agent 与自动化系统的企业级 DAG 编排执行平台（Go）**

---

## 目录

- [产品定位](#产品定位)
- [功能总览](#功能总览)
- [系统架构](#系统架构)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [CLI 使用](#cli-使用)
- [HTTP API 使用](#http-api-使用)
- [核心执行器与示例](#核心执行器与示例)
- [LLM 路由与 Fallback 策略](#llm-路由与-fallback-策略)
- [MCP Adapter 标准层](#mcp-adapter-标准层)
- [与 Codebot 联动](#与-codebot-联动)
- [观测与治理](#观测与治理)
- [测试与质量保障](#测试与质量保障)
- [部署方式](#部署方式)
- [项目结构](#项目结构)
- [路线图](#路线图)
- [常见问题](#常见问题)
- [文档索引](#文档索引)
- [许可证](#许可证)

## 产品定位

`Execraft` 不是一次性脚本工具，而是一个可持续运行的执行底座：

- **任务编排标准化**：将复杂自动化流程抽象为 DAG（任务图）
- **执行可靠性工程化**：超时、重试、背压、依赖失败传播、跳过机制
- **平台集成能力完整**：CLI + REST + SSE + 可选 gRPC
- **Agent 扩展友好**：内置 LLM 规划、MCP Adapter、外部系统联动能力

适合用于：

- Agent 的动作执行内核
- CI/CD 自动化流水线编排
- 数据处理、通知、质量巡检等多阶段任务链
- 多系统协同的任务调度与可观测执行

## 功能总览

### 编排与执行

- DAG 校验与执行计划构建（防循环、依赖合法性检查）
- Worker Pool + Bounded Queue 并发执行模型
- 自动重试、指数延迟、下游任务跳过传播
- 可插拔执行器注册表（内置 + 插件）

### 平台与接口

- REST API：任务提交、状态查询、事件流、指标、工具目录
- SSE 实时事件流：便于前端或平台实时订阅
- 可选 gRPC 服务：`execraft.v1.TaskService`
- CLI：`serve` / `submit` / `watch`

### Agent 与生态

- `llm_plan`：任务级规划执行器（支持路由/fallback）
- `mcp_adapter`：标准化外部能力接入（discover/invoke）
- `codebot_scan`：直接编排 Codebot 质量巡检
- 兼容矩阵 + 自动降级：alias 与 fallback chain

### 观测与运维

- 运行指标：提交量、运行中、成功、失败、跳过
- AI 指标：请求量、fallback、错误、平均时延、质量评分、成本估算
- 事件日志 + 快照恢复：支持故障恢复与追踪

## 系统架构

```text
Client (CLI / API Caller / Agent)
            |
            v
      HTTP Router
            |
            v
   Scheduler + Worker Pool
            |
      Executor Registry
   (builtin / plugin / agent)
            |
   +--------+---------+-------------------+
   |                  |                   |
   v                  v                   v
http_request       llm_plan          mcp_adapter
                                       |
                                       v
                                   External MCP
            |
            v
 Task Store + Event Journal + Snapshot
            |
            v
  Metrics (/metrics, /metrics/ai)
```

## 快速开始

### 环境要求

- Go `>= 1.25`
- Windows PowerShell / Linux Shell / macOS Terminal
- Docker（可选）

### 1) 构建

```powershell
go build .\cmd\execraft
```

### 2) 启动服务

```powershell
go run .\cmd\execraft serve --http :8090 --grpc :50051 --data-dir .\data --store sqlite
```

### 3) 提交任务图

创建 `graph.json`：

```json
{
  "tasks": [
    { "id": "prepare", "kind": "echo", "input": { "msg": "start" } },
    { "id": "wait", "kind": "sleep", "input": { "duration_ms": 80 }, "depends_on": ["prepare"] },
    { "id": "call", "kind": "http_request", "depends_on": ["wait"], "input": { "method": "GET", "url": "https://httpbin.org/get", "timeout_ms": 3000 } }
  ]
}
```

提交：

```powershell
go run .\cmd\execraft submit http://localhost:8090 graph.json
```

### 4) 监听事件

```powershell
go run .\cmd\execraft watch http://localhost:8090
```

### Docker 运行（可选）

```powershell
docker compose up --build
```

## 配置说明

优先级：**命令行参数 > 环境变量 > 默认值**

| 变量 | 说明 | 默认值 |
|---|---|---|
| `EXECRAFT_HTTP_ADDR` | HTTP 监听地址 | `:8090` |
| `EXECRAFT_GRPC_ADDR` | gRPC 监听地址（空表示关闭） | 空 |
| `EXECRAFT_DATA_DIR` | 数据目录（日志/快照） | `data` |
| `EXECRAFT_STORE` | 存储后端（`memory`/`sqlite`） | `memory` |
| `EXECRAFT_SQLITE_PATH` | SQLite 文件路径 | `data/execraft.db` |
| `EXECRAFT_MAX_WORKERS` | Worker 数量 | `8` |
| `EXECRAFT_QUEUE_SIZE` | 队列容量 | `64` |
| `EXECRAFT_SNAPSHOT_SEC` | 快照间隔（秒） | `20` |
| `EXECRAFT_PLUGINS` | 插件执行器列表 | `http-request` |
| `EXECRAFT_LLM_PROVIDER` | LLM Provider（`mock/openai_compat/ollama`） | `mock` |
| `EXECRAFT_LLM_MODEL` | LLM 模型名 | `gpt-4o-mini` |
| `EXECRAFT_LLM_BASE_URL` | LLM Base URL | 空 |
| `EXECRAFT_LLM_API_KEY` | LLM API Key（openai_compat 需要） | 空 |
| `EXECRAFT_CODEBOT_BASE_URL` | Codebot 服务地址 | `http://localhost:8711` |
| `EXECRAFT_CODEBOT_TOKEN` | Codebot Token | `dev-token` |
| `EXECRAFT_CODEBOT_TIMEOUT_MS` | 等待 Codebot 结果超时 | `120000` |
| `EXECRAFT_CODEBOT_WEBHOOK` | Codebot 完成后回调地址 | 空 |

## CLI 使用

### 启动服务

```powershell
go run .\cmd\execraft serve --http :8090
```

### 提交图

```powershell
go run .\cmd\execraft submit http://localhost:8090 .\graph.json
```

### 监听事件

```powershell
go run .\cmd\execraft watch http://localhost:8090
```

## HTTP API 使用

关键接口：

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/health` | 健康检查 |
| `GET` | `/metrics` | 运行指标 |
| `GET` | `/metrics/ai` | AI 成本/时延/质量指标 |
| `POST` | `/tasks` | 提交 DAG 任务图 |
| `GET` | `/tasks` | 任务列表（可按状态/类型筛选） |
| `GET` | `/tasks/{id}` | 单任务详情 |
| `GET` | `/events/stream` | SSE 事件流 |
| `GET` | `/tools` | 当前可用执行器 |
| `GET` | `/tools/matrix` | 兼容别名/降级矩阵 |

### 提交任务图示例（curl）

```bash
curl -X POST "http://localhost:8090/tasks" \
  -H "Content-Type: application/json" \
  -d @graph.json
```

## 核心执行器与示例

### 1) `http_request`

```json
{
  "id": "call-api",
  "kind": "http_request",
  "input": {
    "method": "GET",
    "url": "https://httpbin.org/get",
    "timeout_ms": 3000
  }
}
```

### 2) `llm_plan`

```json
{
  "id": "plan",
  "kind": "llm_plan",
  "input": {
    "objective": "生成发布前质量改进计划",
    "context": "仓库包含 API 和 Web 模块",
    "constraints": ["优先低风险改动", "保留现有接口"],
    "preferred_provider": "openai_compat",
    "fallback_providers": ["ollama", "mock"],
    "max_latency_ms": 6000,
    "min_quality": 75
  }
}
```

### 3) `mcp_adapter`

能力发现：

```json
{
  "id": "discover-mcp",
  "kind": "mcp_adapter",
  "input": { "mode": "discover" }
}
```

调用执行：

```json
{
  "id": "invoke-mcp",
  "kind": "mcp_adapter",
  "input": {
    "mode": "invoke",
    "endpoint": "https://example.com/mcp/execute",
    "method": "POST",
    "auth_type": "bearer",
    "auth_token": "token-xxx",
    "headers": { "x-tenant-id": "demo" },
    "schema": {
      "required_headers": ["x-tenant-id"],
      "require_payload": true
    },
    "payload": { "tool": "lint", "target": "repo-a" },
    "max_retries": 2
  }
}
```

### 4) `codebot_scan`

```json
{
  "id": "quality-scan",
  "kind": "codebot_scan",
  "input": {
    "target": "C:/repo/project-a",
    "mode": "scan",
    "callback_url": "http://localhost:8090/hooks/codebot"
  }
}
```

## LLM 路由与 Fallback 策略

`llm_plan` 的路由策略支持三层：

- **偏好层**：`preferred_provider` / `preferred_model`
- **容错层**：`fallback_providers`
- **动态层**：按历史时延、失败率、质量评分动态排序 provider

这样在真实环境下可实现“先最优、再稳定、最后兜底”的模型调用策略。

## MCP Adapter 标准层

`mcp_adapter` 的目标是把外部能力接入从“硬编码调用”升级为“标准协议层”：

- 统一输入结构（endpoint/auth/schema/retry/timeout）
- 支持能力发现（discover）
- 支持调用前 schema 验证（headers/payload）
- 适合作为后续 MCP provider 统一封装入口

## 与 Codebot 联动

推荐链路：

1. `llm_plan` 产出执行计划
2. `codebot_scan` 执行质量巡检
3. `mcp_adapter` 推送结果到外部系统（工单/消息）

并可通过 `EXECRAFT_CODEBOT_WEBHOOK` 开启自动回调闭环。

## 观测与治理

### 运行指标

- `submitted/running/success/failed/skipped`

### AI 指标（`/metrics/ai`）

- `requests`
- `fallbacks`
- `errors`
- `avg_latency_ms` / `max_latency_ms`
- `cost_milli_usd`
- `avg_quality_score`

### 兼容矩阵（`/tools/matrix`）

- `kinds`：可用执行器列表
- `aliases`：兼容映射
- `fallbacks`：自动降级链

## 测试与质量保障

运行测试：

```powershell
go test ./...
```

覆盖层级：

- **Unit**：图校验、重试策略、核心逻辑
- **Module**：调度重试与依赖传播
- **Integration**：HTTP 提交流程与状态查询

CI/CD：

- `CI`：构建 + 测试 + 质量检查
- `Release`：版本打标发布
- `Container`：容器镜像构建与推送

## 部署方式

### 本地部署

```powershell
go build .\cmd\execraft
go run .\cmd\execraft serve --http :8090
```

### Docker Compose

```powershell
docker compose up --build
```

## 项目结构

```text
.
├── cmd/execraft/
├── internal/
│   ├── api/
│   │   ├── http/
│   │   └── grpcserver/
│   ├── app/
│   ├── config/
│   ├── domain/
│   ├── engine/
│   ├── executor/
│   ├── llm/
│   ├── observability/
│   └── store/
├── proto/
├── tests/
├── docs/
└── docker-compose.yml
```

## 路线图

- 多租户与配额治理（tenant quota / throttle）
- 更细粒度 RBAC（按执行器与资源域）
- SLO 告警与自动化恢复策略
- MCP provider 注册中心与版本策略

## 常见问题

### 1) 为什么 `llm_plan` 没有调用真实模型？

默认 provider 是 `mock`。请设置：

- `EXECRAFT_LLM_PROVIDER=openai_compat`
- `EXECRAFT_LLM_API_KEY=<your-key>`

### 2) 为什么任务出现降级执行？

这是兼容矩阵策略生效（alias/fallback）。可通过 `/tools/matrix` 查看映射和降级链。

### 3) `codebot_scan` 一直等待超时怎么办？

检查：

- `EXECRAFT_CODEBOT_BASE_URL` 是否可访问
- `EXECRAFT_CODEBOT_TOKEN` 是否有效
- `EXECRAFT_CODEBOT_TIMEOUT_MS` 是否过小

## 文档索引

- 中文文档入口：`docs/zh/README.md`
- English docs entry: `docs/en/README.md`

## 许可证

本项目采用 MIT 许可证，详见 `LICENSE`。
