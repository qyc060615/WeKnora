# Answer-level 人工正确性审计

- Commit: `cd21908a652d1330499986e191fbd704b7d9c13b`
- 状态：**HUMAN REVIEW**

## 1. 结论

Aggregate `BenchmarkResult` 未持久化 generated answers 和 retrieved chunks，因此无法从现有证据重建 answer-level audit。本轮没有伪造缺失数据，也不声明“15/15 correct”。

Final Candidate benchmark artifacts 只保存 aggregate `BenchmarkResult`：

- `benchmark/run_*/result.json` 的 top-level keys 为 `metrics`、`usage`、`latency`、`reproducibility`、`models`、`runtime`，没有 per-question answer/evidence；
- `evaluation_runs` 只提供 Precision、Recall、NDCG@3/10、MRR、MAP、BLEU-1/2/4、ROUGE-1/2/L、status 与 counts 等 aggregate columns，没有 question/answer/evidence column。

因此，仅凭 aggregate `BenchmarkResult` 无法重建逐题人工正确性审计。

## 2. 可用材料

`answers.json` 列出 `benchmark_v1` 的全部 15 个冻结 questions、reference answers 和 qrel documents。不可用的 generated-answer 与 retrieved-output fields 均明确为 `null`/empty，每题 provisional verdict 为 `needs_human_review`。

## 3. 边界与后续

- 任何 answer correctness 结论都需要当前未持久化的 per-question answers。
- Final human confirmation 仍由人工完成。
- 若必须重新执行 human audit，需要新的 benchmark execution 显式持久化 per-question answers/evidence；这超出当前冻结 Final Candidate 范围。
