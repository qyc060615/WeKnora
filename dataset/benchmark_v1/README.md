# NebulaTech Evaluation Benchmark v1

This directory contains a small deterministic dataset for WeKnora Evaluation.
The human-reviewed source of truth is `benchmark.json`; the five Parquet files
are generated artifacts.

From the repository root, regenerate them with:

```bash
go run ./dataset/benchmark_v1 \
  -manifest dataset/benchmark_v1/benchmark.json \
  -output dataset/benchmark_v1
```

The benchmark contains 32 passages and 15 questions: 8 single-hop, 4
hard-negative, and 3 boundary questions. Passage IDs are contiguous from 0 to
31 so the Evaluation PID-to-ChunkIndex invariant remains memory efficient.

`baseline_cache_off.json` and `baseline_cache_off_run2.json` are retained as
historical audit artifacts only. They were produced before the Benchmark v1.1
full-corpus fix, when Evaluation indexed only passages referenced by qrels
instead of all 32 corpus passages. They are therefore superseded and must not
be used as official Retrieval Regression baselines.

`baseline_cache_off_v1_1.json` is a Benchmark v1.1 correctness/E2E audit
artifact generated after the full-corpus boundary, reproducibility snapshot,
unified result, and observed-model aggregation passed automated tests and local
runtime validation. It proves the full-corpus and quality/usage structure, but
its embedding provider experienced header timeouts and retries. Consequently,
its run wall-clock, model latency, provider-request, and provider-input values
are not an authoritative operational regression baseline. A clean Cache OFF
run must pass the provider-health gate before those operational values can be
frozen. Cost regression remains unavailable while no matching versioned pricing
rule exists; `no_cost_row_calls` is unknown cost, not zero cost.
