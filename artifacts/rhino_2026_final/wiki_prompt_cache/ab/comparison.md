# Wiki Prompt Cache：Final Candidate A/B 对比（AB sequence）

- Repository: `qyc060615/WeKnora`
- Branch: `feat/topic3-evaluation`
- Commit: `cd21908a652d1330499986e191fbd704b7d9c13b`
- Provider: `deepseek-v4-pro`（DeepSeek automatic prefix caching / Context Caching on Disk）
- Design: controlled A/B pair，AB sequence

两臂唯一差异是 `wiki_config.content_instructions` 中的 cohort marker；该内容进入每个 Wiki LLM prompt 的 static instruction prefix。Corpus、models、temperature、chunking 与 Wiki pipeline 均相同。

## 1. Trial 摘要

| | A（ab-a1） | B（ab-b1） |
|---|---|---|
| Marker | `final-wiki-cohort-a` | `final-wiki-cohort-b` |
| knowledge_base_id | `6b01c0a2-a53e-47cb-9ea7-833c6bea6d22` | `bcebcf3d-6534-46c7-885b-5ac08c3140e5` |
| started_at（UTC） | 2026-09-07T09:10:13Z | 2026-09-07T10:04:03Z |
| finished_at（UTC） | 2026-09-07T09:25:43Z | 2026-09-07T10:22:55Z |
| Wall time | 930.4 s（约 15.5 min） | 1132.5 s（约 18.9 min） |
| Knowledge files | 4/4 `completed` | 4/4 `completed` |
| wiki_pages | 50 `published` | 50 `published` |
| Provider timeout calls | 1 | 10 |

## 2. Prompt-cache metrics

范围：`wiki_*` purposes，`call_type=chat`。

| Metric | A | B |
|---|---:|---:|
| Eligible calls（hit + miss） | 59 | 59 |
| Hit calls | 57 | 57 |
| Miss calls | 2 | 2 |
| Timeout calls（excluded，NULL status） | 1 | 10 |
| Prompt Cache Call Hit Rate | 0.966102 | 0.966102 |
| input_tokens | 171,319 | 175,740 |
| cache_read_tokens | 81,408 | 94,976 |
| cache_write_tokens | 0 | 0 |
| Prompt Token Cache Ratio | 0.475184 | 0.540435 |
| total latency_ms | 3,053,857 | 5,358,023 |

每臂 2 次 `miss` 属于结构性结果：`wiki_taxonomy_plan` 与 `wiki_index_intro` 每次 ingest 各调用一次，没有重复 prefix 可复用；其余 57 calls 均命中 provider prefix cache。DeepSeek automatic prefix caching 不报告 explicit cache writes，因此 `cache_write_tokens` 为 0，复用通过 `cache_read_tokens` 观测。

## 3. Per-purpose reuse

| Purpose | A calls / status / cache_read | B calls / status / cache_read |
|---|---|---|
| wiki_candidate_slug | 4 × hit / 8,576 | 4 × hit / 8,576 |
| wiki_chunk_citation | 4 × hit / 2,560 | 4 × hit / 2,560 |
| **wiki_summary** | **4 × hit / 2,048** | **4 × hit / 2,048** |
| wiki_page_modify | 45 × hit / 68,224 | 45 × hit / 81,792 |
| wiki_taxonomy_plan | 1 × miss / 0 | 1 × miss / 0 |
| wiki_index_intro | 1 × miss / 0 | 1 × miss / 0 |

## 4. 数据支持的结论

1. `wiki_summary` layer 的 prompt-cache reuse 稳定：两臂均为 4/4 `hit`，且 `cache_read_tokens = 2048`。
2. 相同 frozen configuration 下，两次独立 run 的 Call Hit Rate 均为 `0.966102`，hit/miss 均为 57/2，说明 hit behaviour 可复现。
3. Token Cache Ratio 存在 run-to-run variance（0.475 对 0.540），主要来自 `wiki_page_modify`；不能将其解释为 before/after improvement。
4. 本实验不声明 `wiki_page_modify` 获得提升，因为该 single-commit cohort A/B 不包含 old/new contrast。
5. Latency 仅为辅助指标。B arm 位于 provider-degraded window，10 次 timeout 拉长 wall time 与 total latency，但没有改变 hit rate；timeouts 被保留而未隐藏。

允许的表述：Wiki summary layer 展现 prompt-cache reuse；prompt-cache call hit rate `0.966` 在两个受控 arm 中可复现。

不允许的表述：所有 Wiki stages 都取得显著 cache improvement。

## 5. BA replication

未执行。B arm 的 provider-degraded window 记录 10 次 timeout，并在随后触及 provider limits。A/B pair 可证明 Final Candidate `wiki_summary` reuse，但不满足完整 AB/BA crossover；该限制保持不变。

历史 `artifacts/wiki_prompt_cache_v1/` before/after 数据继续作为辅助 sanity evidence 保留，不声明为 Final Candidate run。
