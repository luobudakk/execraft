# gRPC API 参考

Execraft 提供可选 gRPC 服务（通过 `EXECRAFT_GRPC_ADDR` 或 `--grpc` 启用）。

## 服务名

- `execraft.v1.TaskService`

## 方法列表

- `SubmitTaskGraph`
- `GetTask`
- `ListTasks`
- `Health`
- `Metrics`

## 消息体约定

当前实现使用 `google.protobuf.Any` 作为统一载体，`Any.value` 中存放 JSON。

- `SubmitTaskGraph` 请求：`TaskGraph` JSON
- `GetTask` 请求：`{"id":"<task_id>"}`
- `ListTasks` 请求：`{"status":"success","kind":"echo"}`
