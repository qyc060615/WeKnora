# Historical Wiki Prompt Cache Before/After 审计

## 1. Evidence identity

- Before commit: `3f9a054ec94e92c3089a6574281aa09760068e38`
- After commit: `22f9120216d554181db041b24e3e191363c34366`
- Artifact commit: `e1ff86d02708ff7d3b98162bd0c63a4f44afdd8d`
- Commit chronology: `3f9a054e` 是 `22f91202` 的直接父提交；artifact commit 是 `22f91202` 的后代
- Tenant: `10000`
- Chat model: `deepseek-v4-pro`
- Model config ID: `5a50bf0a-60f9-4340-974b-0bd85c80b286`
- Configured provider: `generic`；effective provider: DeepSeek；resolved model: `deepseek-v4-pro`
- Workload: `dataset/benchmark_sources/nebultech_v1` 中 4 个 Markdown files：`security_policy`、`incident_response`、`database_policy`、`api_guidelines`

4 个文件当前 SHA-256 与 `experiment_config.json` 一致。原 JSON artifacts 未持久化精确 trial time bounds；existing KB creation timestamps 可定位 2026-09-05 UTC 的 4 次顺序执行，read-only DB slices 可复现核心 per-purpose aggregates。

## 2. Prompt change

`git diff 3f9a054e..22f91202` 证明存在真实 code/assembly before-after：

- Before：`WikiSummaryPrompt` 和 `WikiCandidateSlugPrompt` 将 dynamic document content 及 request-varying slugs 放在 shared instruction block 之前；
- After：stable instructions 与 assembled business instructions 被放在 dynamic document/slugs 之前，形成可复用 provider prefix；
- 同一 commit 还为 PageModify 增加 stable provider prompt-cache keys，并稳定 image-placeholder ordering。该 corpus 为 plain text，因此未覆盖 image change；DeepSeek automatic prefix caching 不直接使用 explicit application cache key。`wiki_summary` 结果只与 prompt reordering 对齐。

## 3. Metric definitions

- Prompt Cache Call Hit Rate = `hit_calls / (hit_calls + miss_calls)`；unsupported、unreported、NULL、timeout、failed rows 排除。
- Prompt Token Cache Ratio = `SUM(cache_read_tokens) / SUM(input_tokens)`，只统计 cache-accounted 且 input/read tokens 已观测的 rows；missing values 保持 NULL。

Historical README/SQL 使用 `cache_read / (cache_read + cache_miss)`。这些 DeepSeek `wiki_summary` rows 满足 `input_tokens = cache_read_tokens + cache_miss_tokens`，因此按当前公式重算结果不变。原 JSON files 未修改。

## 4. `wiki_summary` 重算结果

| Mode | Calls | Hit | Miss | Cache read | Input tokens | Call hit rate | Current token ratio |
|---|---:|---:|---:|---:|---:|---:|---:|
| BEFORE, pair 1 | 4 | 0 | 4 | 0 | 6,952 | 0% | 0% |
| BEFORE, pair 2 | 4 | 0 | 4 | 0 | 6,761 | 0% | 0% |
| **BEFORE total** | **8** | **0** | **8** | **0** | **13,713** | **0%** | **0%** |
| AFTER, pair 1 | 4 | 3 | 1 | 1,536 | 6,661 | 75% | 23.0596% |
| AFTER, pair 2 | 4 | 4 | 0 | 2,048 | 6,773 | 100% | 30.2377% |
| **AFTER total** | **8** | **7** | **1** | **3,584** | **13,434** | **87.5%** | **26.6786%** |

可辩护的 historical 结论仅限 summary layer：call-hit rate `0% → 87.5%`，current-formula token-cache ratio `0% → 26.6786%`。

## 5. 六项审计

1. **真实 old-vs-optimized prompt ordering：PASS。** Commit ancestry 与 source diff 建立真实结构变化，不是 relabelled cold/warm comparison。
2. **Workload comparability：`wiki_summary` PASS；all-Wiki totals 有 confounder。** Corpus、每 trial 4 次 summary calls、pipeline、pair 内 markers 与 model 受控，但 generated pages 为 BEFORE 59、AFTER 49，PageModify 和 all-Wiki totals 不能支持优化结论。
3. **Model comparability：PASS。** 两侧 model config、resolved model、provider path 和 tenant 相同。
4. **Metric semantics：显式重算后 PASS。** 使用当前 denominator；与旧 denominator 相等是数据特性，不作为一般假设。
5. **Data provenance：summary-layer claim PASS，但有限制。** Artifacts 包含 SQL 与 DB-derived aggregates，现存 DB rows 可复现 8-vs-8 counts/tokens；但 JSON 缺 timestamps/usage IDs，`raw_queries.sql` 缺 explicit model filter，部分 `wiki_index_intro` totals 有 boundary carry-over，因此 all-Wiki artifact 不作为 clean trial-isolated provenance。
6. **Commit linkage：PASS。** Git 可直接恢复 before/after commits 和 precise stable-prefix move，不依赖 README narrative。

## 6. Confounders 与限制

- 记录方案称为 `AB/BA`，但两组实际 sequence 都是 `AFTER then BEFORE`，不是 balanced crossover。
- DeepSeek provider-native prefix cache 无法清空；pair 2 受早期 request warm-up 影响，不能形成 clean cold-state randomized causal estimate。
- 每个 mode 只有 8 个 successful summary calls，provider variance 仍存在。
- Generated page counts 不同，使 PageModify/all-Wiki tokens、cost、latency 混杂；不声明 overall Wiki improvement。
- 原 JSON 未保留 exact per-trial timestamps 与 row IDs。
- 4-document corpus 较小且无图片。

## 7. 与 Final Candidate A/B 的关系

Historical evidence 回答方向性：shared summary instructions 移到 dynamic content 前，使 `wiki_summary` 从 0/8 hits 变为 7/8 hits。Final Candidate A/B 回答实现结果的可复现性：两个独立 arm 均为 `wiki_summary` 4/4 hits、2,048 cache-read tokens，whole-Wiki eligible hit rate 均为 57/59（`0.966102`）。Final Candidate A/B 不是另一次 old/new comparison，不得这样表述。

## 8. Verdict

对于“optimized ordering 改善 `wiki_summary` provider-prefix reuse”这一 layer-scoped historical claim，结论为 **VALID WITH LIMITATIONS**。它不能证明所有 Wiki stages、PageModify、overall latency 或 total cost 均得到改善。
