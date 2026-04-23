# gRPC API

Enable gRPC with `EXECRAFT_GRPC_ADDR` (or `--grpc`).

Service: `execraft.v1.TaskService`

Methods:
- `SubmitTaskGraph`
- `GetTask`
- `ListTasks`
- `Health`
- `Metrics`

The current transport uses `google.protobuf.Any`, with JSON payload in `Any.value`.
