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

`baseline_cache_off.json` is the complete successful Evaluation response from
the initial Cache OFF candidate run. It records the model IDs, retrieval
configuration, task counts, and all retrieval and generation metrics used for
later regression comparisons.
