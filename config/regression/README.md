# Regression CI 配置

这里是 Benchmark v1.1 Regression Gate 的输入数据与阈值策略。它们只被
`cmd/regression`（comparator）与 `cmd/regression-benchmark`（current generation）
消费，不参与 Benchmark 执行。

## 文件

- `baseline.json` — 冻结的 baseline。`quality` 部分来自真实 Benchmark v1.1
  Cache OFF run（见下方 Baseline Provenance），只保留 quality metrics +
  标识符，不包含 latency / token / cache 等 operational 数据。
- `current.json` — 最新的 current result。初始状态与 baseline 相同（PASS）。
  真实 Benchmark 运行后用 `cmd/regression-benchmark` 产出的 JSON 覆盖它。
- `policy.json` — 阈值策略。`default_allowed_drop` 是默认允许下降的绝对幅度，
  `allowed_drop` 可按 metric 覆盖。

## Baseline Provenance

| 字段 | 值 |
|---|---|
| task_id | `evaluation_10000_1788273787263_0cff0de6_benchmarkv1` |
| evaluation_run_id | `449b15c1-0b49-481c-aef2-00091254d98d` |
| benchmark version | `v1.1` |
| 来源 | `dataset/benchmark_v1/baseline_cache_off_v1_1.json`（真实 run） |
| 冻结 | 是（quality metrics） |

`baseline_cache_off_v1_1.json` 是 Benchmark v1.1 correctness/E2E audit artifact。
其 12 个 quality metrics 是真实的、可追溯的；但该 run 的 embedding provider
出现过 header timeout / retry，因此 `run_wall_clock_duration_ms` / model latency /
provider-request / provider-input / cost 这些 **operational 值不是权威 operational
baseline**（见 `dataset/benchmark_v1/README.md`）。Regression Comparator 只 gate
12 个 quality metrics，不 gate operational 值，因此该 run 的 quality 部分可安全
作为 quality regression baseline。

## 阈值语义

所有 12 个 quality metrics 都是 higher-is-better。对每个 metric：

```
delta     = current - baseline
threshold = -allowed_drop        # 允许的最大绝对下降幅度（取负号后是 delta 下限）
PASS      = delta >= threshold   # 即 current >= baseline - allowed_drop
```

阈值统一使用**绝对下降**（不是百分比下降）。默认 `0.02`，当前仓库没有官方阈值
要求，故采用可配置机制 + 文档化默认值。任一 metric 下降超过自己的 threshold →
整体 FAIL（禁止加权抵消）。

## 运行

Comparator（比较 baseline vs current）：

```bash
go run ./cmd/regression \
  --baseline config/regression/baseline.json \
  --current  config/regression/current.json \
  --policy   config/regression/policy.json
```

生成 current（真实 Benchmark，需要 DB + 模型 provider 环境）：

```bash
go run ./cmd/regression-benchmark --output artifacts/regression/current.json
```

exit code：`0` = PASS，`1` = regression（或 metric 缺失 / 非有限数），`2` =
执行错误（文件不可读 / JSON 损坏 / Benchmark 失败）。质量退化或 Benchmark
执行失败绝不会返回 `0`。
