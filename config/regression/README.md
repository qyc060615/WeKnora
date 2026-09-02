# Regression CI 配置

这里是 Benchmark v1.1 Regression Gate 的输入数据与阈值策略。它们只被
`cmd/regression` 消费，不参与 Benchmark 执行。

## 文件

- `baseline.json` — 冻结的 baseline。是 `types.BenchmarkResult` 的一个最小
  JSON 投影，只包含 `quality`（12 个 quality metrics）+ 标识符。当前值是
  **占位 baseline**，还没有对应真实 Benchmark 运行。要让 Gate 真正有意义，
  需要用一个真实 Benchmark v1.1 结果导出为完整 `BenchmarkResult` JSON 替换它
  （adapter 只读取 `quality`，多出的 `config` / `model_facts` 字段会被忽略）。
- `current.json` — 最新的 current result。初始状态与 baseline 相同，因此 Gate
  为 PASS。每次真实 Benchmark 运行后用它覆盖 `current.json`（或通过
  `workflow_dispatch` 的 `current` 输入指向另一路径）。
- `policy.json` — 阈值策略。`default_allowed_drop` 是默认允许下降的绝对幅度，
  `allowed_drop` 可按 metric 覆盖。

## 阈值语义

所有 12 个 quality metrics 都是 higher-is-better。对每个 metric：

```
delta     = current - baseline
threshold = -allowed_drop        # 允许的最大绝对下降幅度（取负号后是 delta 下限）
PASS      = delta >= threshold   # 即 current >= baseline - allowed_drop
```

阈值统一使用**绝对下降**（不是百分比下降）。默认 `0.02`，当前仓库没有官方阈值
要求，故采用可配置机制 + 文档化默认值。

## 运行

```bash
go run ./cmd/regression \
  --baseline config/regression/baseline.json \
  --current  config/regression/current.json \
  --policy   config/regression/policy.json
```

exit code：`0` = PASS，`1` = regression（或 metric 缺失 / 非有限数），`2` =
执行错误（文件不可读 / JSON 损坏）。质量退化绝不会返回 `0`。
