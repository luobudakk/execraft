# HTTP API 参考

## 端点

- `POST /tasks`
- `GET /tasks/{id}`
- `GET /tasks?status=success&kind=echo`
- `GET /events/stream`
- `GET /health`
- `GET /metrics`

## 示例：提交任务图

```json
{
  "tasks": [
    {"id":"a","kind":"echo","input":{"msg":"hello"}},
    {"id":"b","kind":"sleep","input":{"duration_ms":200},"depends_on":["a"]}
  ]
}
```
