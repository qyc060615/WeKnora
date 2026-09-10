# Runtime API Capture 历史尝试

早期曾两次准备从以下 endpoint 保存 authenticated response：

```text
GET /api/v1/model-usage/analytics
```

两次命令均在执行前因 external-command permission review timeout 而停止；当时没有发送 request、没有生成 response artifact，也没有写入 credential。

该记录仅描述早期尝试，后来已经完成真实 authenticated API capture，最终证据为 [runtime_api_capture.json](runtime_api_capture.json) 和 [runtime_reconciliation.md](runtime_reconciliation.md)。因此这里的历史失败不再表示最终 runtime reconciliation 未完成。
