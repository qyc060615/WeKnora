# Model Usage Dashboard Runtime 截图核验

## 1. Evidence identity

- Page: `/platform/settings?section=models`
- Runtime screenshot: 用户于 2026-09-07 在对话中提供；repo 未复制该图片
- Model: `deepseek-v4-pro`
- UI date range: `2026-08-31` 至 `2026-09-07`
- Interval: Day（`按天`）
- Browser/UI timezone: Asia/Shanghai（UTC+08:00）

Date-only UI 将该选择转换为半开 API range `[2026-08-30T16:00:00Z, 2026-09-07T16:00:00Z)`，比 fixed DB/API capture range `[2026-08-31T13:00:00Z, 2026-09-07T11:00:00Z)` 更宽。Read-only DB comparison 确认较宽 UI range 对该 tenant/model 没有增加 rows，两者均选择相同的 658-row set，aggregate 完全一致。

## 2. 截图可见值

| Dashboard field | Displayed value | DB/API underlying value | Match |
|---|---:|---:|---|
| Calls | 658 | 658 | PASS |
| Total Tokens | 309.9万 | 3,099,016 | PASS（formatted） |
| Input Tokens | 168.9万 | 1,689,294 | PASS（formatted） |
| Output Tokens | 141万 | 1,409,722 | PASS（formatted） |
| Input observation coverage | 629 / 658 | 629 / 658 | PASS |
| Average latency | 35.85 s | 35,849.83434650456 ms | PASS（formatted） |
| Prompt Cache Call Hit Rate | 89.3% | 562 / 629 = 0.8934817170111288 | PASS（formatted） |
| Prompt Token Cache Ratio | 54.1% | 913,280 / 1,689,294 = 0.5406282151005094 | PASS（formatted） |
| Prompt cache detail | hit 562 / miss 67 / eligible 629 | 562 / 67 / 629 | PASS |
| Embedding input hit rate | — | null under chat-model filter | PASS（`NULL != 0`） |

截图可见 model、date range、interval、summary cards 与两项 Prompt Cache metrics。Compact-number 和 percentage rounding 遵循 frontend formatter，不把 rounded display value 当作 raw-value mismatch。

## 3. 结论

**PASS**，并保留 date-control limitation：Dashboard 不能表达任意 UTC timestamp，但 capture time 的生成 range 与 fixed range 选中完全相同的 rows。这是 point-in-time equivalence，不代表未来写入后两个 range 仍等价。
