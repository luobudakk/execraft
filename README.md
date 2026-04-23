# Execraft

`Execraft` 是一个使用 Go 编写的 DAG 任务编排与执行内核。  
它面向“可控执行、可观测状态、可恢复运行”的自动化场景，提供 HTTP API、实时事件流和命令行工具。

## 项目亮点

- 事件驱动任务运行时：任务状态变化以事件形式记录与输出
- 并发调度增强：`worker pool + bounded queue + backpressure`
- 可靠执行机制：支持超时、重试、依赖失败传播与下游跳过
- 状态持久化：`events.log`（事件日志）+ `snapshot.json`（快照）双层恢复
- 对外能力完整：REST API + SSE 实时事件流
- 可选 gRPC 接口：`execraft.v1.TaskService`
- CLI 体验友好：`serve` / `submit` / `watch` 子命令
- 工程化完整：内置 Docker、Compose、CI、Release、GHCR 镜像发布流水线
- 可扩展存储：`memory` / `sqlite` 可切换

## 适用场景

- AI Agent 动作执行层
- 自动化任务流水线（采集 -> 处理 -> 通知）
- 需要“任务状态可追踪 + 失败可恢复”的轻量后端服务
- 需要通过 HTTP 快速集成任务编排能力的系统

## 技术能力总览

| 能力维度 | Execraft 实现方式 | 价值 |
|---|---|---|
| 任务编排核心 | `BuildPlan + ValidateGraph` 两阶段处理 | 规划与执行解耦，降低调度耦合度 |
| 并发模型 | 有界队列 + Worker Pool | 队列过载时主动背压保护 |
| 状态存储 | 事件日志 + 周期快照 | 可审计、可回放、可恢复 |
| 对外接口 | REST + SSE + 可选 gRPC | 便于系统集成和实时订阅 |
| 使用方式 | CLI 子命令化 | 适合开发与运维直接操作 |

## 架构概览

```mermaid
flowchart LR
  client[CLIorHTTPClient] --> api[HTTPAPI]
  api --> planner[PlanBuilder]
  planner --> scheduler[RuntimeScheduler]
  scheduler --> queue[BoundedQueue]
  queue --> workers[WorkerPool]
  workers --> executors[ExecutorRegistry]
  scheduler --> events[EventLog]
  events --> snapshot[Snapshot]
```

## 目录结构

- `cmd/execraft`：CLI 入口与子命令
- `internal/domain`：任务模型、图校验、执行计划
- `internal/engine`：调度器、并发池、重试策略
- `internal/executor`：执行器注册与内置执行器
- `internal/store`：内存状态、事件日志、快照恢复
- `internal/api/http`：HTTP 路由与处理器（含 SSE）
- `tests`：单元/模块/集成测试

## 快速开始（Windows PowerShell）

### 1) 构建

```powershell
go build .\cmd\execraft
```

### 2) 启动服务

```powershell
go run .\cmd\execraft serve --http :8090 --grpc :50051 --data-dir .\data --store sqlite
```

### Docker 启动

```powershell
docker compose up --build
```

### 3) 提交任务图

示例 `graph.json`：

```json
{
  "tasks": [
    {
      "id": "a",
      "kind": "echo",
      "input": { "msg": "hello" }
    },
    {
      "id": "b",
      "kind": "sleep",
      "input": { "duration_ms": 100 },
      "depends_on": ["a"]
    }
  ]
}
```

提交：

```powershell
go run .\cmd\execraft submit http://localhost:8090 graph.json
```

### 4) 监听实时事件

```powershell
go run .\cmd\execraft watch http://localhost:8090
```

## HTTP API

- `POST /tasks`：提交任务图
- `GET /tasks/{id}`：查询单任务状态
- `GET /tasks?status=success&kind=echo`：按条件筛选任务
- `GET /events/stream`：SSE 实时事件流
- `GET /health`：健康检查
- `GET /metrics`：运行指标

## gRPC 契约

- Proto 文件：`proto/execraft/v1/execraft.proto`
- 服务桩（pb）：`internal/api/grpcpb/execraft_grpc.pb.go`
- gRPC 服务实现：`internal/api/grpcserver/server.go`

服务名：`execraft.v1.TaskService`  
方法：`SubmitTaskGraph`、`GetTask`、`ListTasks`、`Health`、`Metrics`

插件执行器示例（`http_request`）：

```json
{
  "tasks": [
    {
      "id": "call-api",
      "kind": "http_request",
      "input": {
        "method": "GET",
        "url": "https://httpbin.org/get",
        "timeout_ms": 3000
      }
    }
  ]
}
```

## 配置项

环境变量（命令行参数优先级更高）：

- `EXECRAFT_HTTP_ADDR`（默认 `:8090`）
- `EXECRAFT_DATA_DIR`（默认 `data`）
- `EXECRAFT_MAX_WORKERS`（默认 `8`）
- `EXECRAFT_QUEUE_SIZE`（默认 `64`）
- `EXECRAFT_SNAPSHOT_SEC`（默认 `20`）
- `EXECRAFT_PLUGINS`（默认 `http-request`，逗号分隔）
- `EXECRAFT_GRPC_ADDR`（默认空，设置后启用 gRPC）
- `EXECRAFT_STORE`（`memory` 或 `sqlite`）
- `EXECRAFT_SQLITE_PATH`（默认 `data/execraft.db`）

## 测试

```powershell
go test ./...
```

覆盖范围：

- Unit：图校验、重试策略
- Module：调度重试、依赖链路
- Integration：HTTP 提交/查询流程

## CI/CD 与发布

- `CI`：`.github/workflows/ci.yml`
  - 格式检查、`go test ./...`、构建验证
- `Release`：`.github/workflows/release.yml`
  - 打标签 `v*` 自动构建多平台二进制并创建 GitHub Release
- `Container`：`.github/workflows/container.yml`
  - 自动构建并推送多架构镜像到 `ghcr.io/<owner>/execraft`

## 文档

- 中文文档入口：`docs/zh/README.md`
- English docs entry: `docs/en/README.md`

## 许可证

本项目采用 MIT 许可证，详见 `LICENSE`。
