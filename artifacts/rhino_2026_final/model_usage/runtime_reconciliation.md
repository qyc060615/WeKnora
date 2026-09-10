# Model Usage Runtime 对账明细

## 1. 范围

- Tenant: `10000`
- Model: `deepseek-v4-pro`
- Model ID: `5a50bf0a-60f9-4340-974b-0bd85c80b286`
- Fixed DB/API range: `[2026-08-31T13:00:00Z, 2026-09-07T11:00:00Z)`
- Call type: `chat`

## 2. DB → Analytics API → Dashboard

| Metric | DB runtime | Analytics API raw | Dashboard screenshot | Verdict |
|---|---:|---:|---:|---|
| Calls | 658 | 658 | 658 | PASS |
| Input tokens | 1,689,294 | 1,689,294 | 168.9万 | PASS |
| Output tokens | 1,409,722 | 1,409,722 | 141万 | PASS |
| Total tokens | 3,099,016 | 3,099,016 | 309.9万 | PASS |
| Input observed/applicable | 629 / 658 | 629 / 658 | 629 / 658 | PASS |
| Latency sum（ms） | 23,589,191 | 23,589,191 | — | PASS DB/API |
| Average latency | 35,849.834347 ms | 35,849.83434650456 ms | 35.85 s | PASS |
| Prompt hits | 562 | 562 | 562 | PASS |
| Prompt misses | 67 | 67 | 67 | PASS |
| Prompt eligible | 629 | 629 | 629 | PASS |
| Prompt not recorded | 29 | 29 | not separately displayed | PASS DB/API |
| Prompt call hit rate | 0.893482 | 0.8934817170111288 | 89.3% | PASS |
| Prompt cache-read tokens | 913,280 | 913,280 | not separately displayed | PASS DB/API |
| Prompt token denominator | 1,689,294 | 1,689,294 | not separately displayed | PASS DB/API |
| Prompt token cache ratio | 0.540628 | 0.5406282151005094 | 54.1% | PASS |
| Embedding cache input hit rate | null/N/A | null | — | PASS（`NULL != 0`） |

DB values 来自 read-only SQL query。Raw API artifact 通过 running local backend 和已有 encrypted tenant API key 获取；evidence 未保存 key 或 Authorization header，API response 回显 exact fixed range 与 model ID。

Dashboard screenshot 在 Asia/Shanghai 选择 `2026-08-31` 至 `2026-09-07`。Date-only control 请求 `[2026-08-30T16:00:00Z, 2026-09-07T16:00:00Z)`；独立 read-only DB query 证明该 model 在两个 range 中均为相同 658 rows 和相同 aggregates。该结论只适用于 capture time，后续写入后不能继续假设 range 等价。

## 3. 结论

**PASS**。Runtime DB values 等于 authenticated Analytics API values，用户提供的 Dashboard runtime screenshot 显示对应 frontend-formatted values。UI 无法输入任意 UTC timestamp 的限制已明确保留。
