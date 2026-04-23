# 插件执行器

## 启用方式

通过配置项 `EXECRAFT_PLUGINS` 或命令行 `--plugins` 启用，多个插件用逗号分隔。

默认启用：

- `http-request`

## 内置插件：http-request

注册执行器类型：`http_request`

输入示例：

```json
{
  "method": "GET",
  "url": "https://httpbin.org/get",
  "timeout_ms": 3000
}
```
