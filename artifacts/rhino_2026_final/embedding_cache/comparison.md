# Embedding Cache OFF / COLD / WARM 对比

Final candidate: `cd21908a652d1330499986e191fbd704b7d9c13b`

Dataset: `benchmark_v1`，semantic SHA-256 `56fd363d797ee4c1524a5a1a2517b3b30ce955229c37784cf730c0d1dc47fd0d`

3 个有效 phase 均使用相同 32 个 corpus texts、相同 embedding model（`c24c212d-ae14-4bed-b4c3-be95eeb36464`，`text-embedding-v4`）、production pooled-batch path，并返回 32 vectors；只有 cache enablement/state 不同。

| Phase | Cache state | Hits | Misses | Provider inputs | Provider requests | Harness elapsed |
|---|---|---:|---:|---:|---:|---:|
| OFF | disabled | 0 | 0 | 32 | 7 | 670 ms |
| COLD | enabled, empty prefix | 0 | 32 | 32 | 7 | 584 ms |
| WARM | enabled, populated prefix | 32 | 0 | 0 | 0 | 10 ms |

WARM 达到 100% cache hit rate，并在该受控 input set 中消除 provider inputs/requests。Harness elapsed 相比 COLD 低 98.3%（10 ms 对 584 ms），这是本地受控观测，不代表通用 latency SLA。

Model Usage row IDs：OFF `86b59a63-919d-459d-a82b-c4b16fcd692d`；COLD `5a1a90d3-dd18-4165-aeb4-1f94b472302d`；WARM `bd15e611-cf92-47e7-88b0-1c25ffdd34a8`。

早期 setup probe `a51ab543-5e70-4e2f-a736-952b1358aa7f` 使用单个 32-item provider batch，因 provider limit 为 10 而失败。该 probe 排除于对比；有效 run 使用应用的 production pooled-batch path。
