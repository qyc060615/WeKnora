# Wiki Prompt Cache BEFORE/AFTER 对照实验

Repository: `qyc060615/WeKnora` · Branch: `feat/topic3-evaluation`
实验日期: 2026-09-05

---

## 1. Executive Result

**MODEST IMPROVEMENT**

AFTER commit（`22f9120`）把 `WikiSummaryPrompt` / `WikiCandidateSlugPrompt` 的静态 `<instructions>` 规则块从 document 之后**前置**到 prompt 开头。在真实 DeepSeek provider 上，这一改动产生了**稳定、可复现**的 cache 改善，但改善集中在一层：

- **`wiki_summary`**：cache hit call rate **0% → 87.5%**，token cache ratio **0% → 26.7%**（两对 trial 一致：BEFORE 恒为 0 hit，AFTER 为 3/4、4/4 hit）。
- **`wiki_candidate_slug`**：冷启动时 **0% → 25%** hit rate（Pair 1）；provider 缓存预热后两者都达到 100%（Pair 2），改善被 cache 预热抹平。
- **`wiki_page_modify`**（本实验预期的核心层）：**没有归因于本 commit 的改善**。它的 91–100% hit rate 来自进程内 `awaitWikiPromptWarmup` 串行化 + 稳定 `SharedSourceContexts` 前缀，这是 BEFORE/AFTER **共有的既有机制**，并非 `22f9120` 引入。

一句话总结：**本 commit 的 prompt 前置对 `wiki_summary` 有真实、稳定的 cache 收益；对 `wiki_page_modify` 无额外收益（其高命中来自既有 warmup 机制）。**

---

## 2. Experiment Contract

| 项 | 值 |
|---|---|
| BEFORE | `3f9a054ec94e92c3089a6574281aa09760068e38` |
| AFTER | `22f9120216d554181db041b24e3e191363c34366` |
| corpus | `dataset/benchmark_sources/nebultech_v1/` 4 个 Markdown（security_policy / incident_response / database_policy / api_guidelines），纯文本无图片 |
| model | `deepseek-v4-pro`（config id `5a50bf0a-...`，provider `generic` → effective `deepseek`） |
| endpoint | `https://api.deepseek.com`（automatic prefix caching） |
| tenant | `10000` |
| trial 数 | 2 对（AB/BA）：Pair1 BEFORE/AFTER（冷启动）、Pair2 AFTER/BEFORE（预热） |
| 每 trial | 全新 wiki KB + 完整重新 ingest（初态天然一致） |

---

## 3. Control Validation（控制变量一致性）

BEFORE/AFTER 两侧完全一致，唯一变量是 git commit：

- corpus：4 个文件 SHA256 固定（见 `experiment_config.json`），纯文本、无图片 → chunk 切分确定。
- model config：`synthesis_model_id` / `summary_model_id` / `embedding_model_id` 相同。
- WikiConfig：`granularity=standard`、batch/map/reduce concurrency 默认值相同；每 pair 内 marker 相同。
- 硬编码控制变量：`temperature=0.3`、`thinking=false`、`max_tokens=32768`、`llm_max_attempts=3`、`backoff=2s`。
- 运行环境：同一 WSL2 主机、同一 postgres/redis/docreader、同一执行命令 `go run ./cmd/server`。
- **初态**：每 trial 全新 KB（无 wiki pages、无 model_usage 残留），corpus 重新 ingest。

**注意到的唯一非对称**：BEFORE 与 AFTER 的 `wiki_pages` 数量不同（BEFORE 59/59，AFTER 49/49）。这是 LLM 在两种 prompt 结构下 candidate 提取的固有差异，**不是 cache 的产物**，见 §8。

---

## 4. Usage Results（用量维度，合并 2 对 trial）

| Layer | Metric | BEFORE | AFTER | Delta | Relative |
|---|---|---|---|---|---|
| **ALL wiki_*** | logical_calls | 138 | 117 | −21 | −15.2% |
| | cache_read_tokens | 201,600 | 152,832 | −48,768 | −24.2% |
| | token_cache_ratio | 48.1% | 44.1% | −4.0pp | — |
| | call_hit_rate | 87.0% | 88.0% | +1.0pp | — |
| **wiki_page_modify** | logical_calls | 108 | 88 | −20 | −18.5% |
| | cache_read_tokens | 187,904 | 134,144 | −53,760 | −28.6% |
| | token_cache_ratio | 52.2% | 46.2% | −6.0pp | — |
| | call_hit_rate | 100% | 95.5% | −4.5pp | — |
| **wiki_candidate_slug** | logical_calls | 8 | 8 | 0 | 0% |
| | cache_read_tokens | 8,576 | 10,624 | +2,048 | +23.9% |
| | token_cache_ratio | 49.1% | 60.8% | +11.7pp | — |
| | call_hit_rate | 50.0% | 62.5% | +12.5pp | — |
| **wiki_summary** | logical_calls | 8 | 8 | 0 | 0% |
| | cache_read_tokens | 0 | 3,584 | +3,584 | ∞ |
| | token_cache_ratio | 0% | 26.7% | +26.7pp | — |
| | call_hit_rate | 0% | 87.5% | +87.5pp | — |
| **wiki_chunk_citation** | logical_calls | 8 | 8 | 0 | 0% |
| | cache_read_tokens | 5,120 | 4,480 | −640 | −12.5% |
| | token_cache_ratio | 27.9% | 25.3% | −2.6pp | — |
| | call_hit_rate | 100% | 87.5% | −12.5pp | — |

> `logical_calls` 定义 = `COUNT(*)`（每行 model_usage = 1 次 logical invocation）。`token_cache_ratio = SUM(cache_read_tokens) / (SUM(cache_read_tokens) + SUM(cache_miss_tokens))`。

---

## 5. Time Results（时间维度）

整体 wiki 任务的 DB 观测延迟（所有 trial 合并，latency 含 concurrency 等待 + provider 往返 + 流式消费）：

| purpose | n | avg | P50 | P95 | max |
|---|---|---|---|---|---|
| wiki_candidate_slug | 17 | 112.7s | 107.1s | 147.4s | 157.6s |
| wiki_chunk_citation | 17 | 111.4s | 103.9s | 173.7s | 194.0s |
| wiki_summary | 17 | 79.0s | 74.6s | 119.6s | 147.3s |
| wiki_page_modify | 207 | 20.6s | 17.5s | 42.8s | 85.8s |
| wiki_taxonomy_plan | 5 | 181.2s | 201.3s | 267.3s | 283.7s |
| wiki_index_intro | 5 | 7.9s | 7.9s | 9.9s | 10.1s |

Wall clock（orchestrator 观测，4 文档单 trial）：AFTER ≈ 7.5 min，BEFORE ≈ 8 min。受 DeepSeek 生成长 JSON（candidate_slug 单次 completion 可达 ~10k token）主导，**latency 维度 BEFORE/AFTER 无一致差异，波动大**。

---

## 6. Cost Results

**不可得（unpriced）**：本环境 `model_pricing` 表为空，`model_usage_cost` 全部 `status=NULL`、`total_cost=NULL`。按约定不自行重算价格替代 DB truth。

间接推断（仅供参考，非 DB truth）：`wiki_summary` 的 cache_read_tokens 从 0 → 3,584，若按 DeepSeek cache-hit 输入定价（显著低于未命中输入），AFTER 在该层有输入成本下降趋势；`wiki_page_modify` 因 page 数差异成本不可比。

---

## 7. Cache Analysis

**`call_hit_rate` ≠ `token_cache_ratio`**（两个不同指标，勿混用）：

- `call_hit_rate = hit_calls / (hit_calls + miss_calls)`：一次调用只要命中了**任意数量**的 cache token 即记为 hit。它衡量「多少比例的调用触发了缓存」。
- `token_cache_ratio = Σ cache_read_tokens / (Σ cache_read_tokens + Σ cache_miss_tokens)`：衡量「输入 token 中有多大比例从缓存读取」。它更能反映**成本节省量**，因为 DeepSeek 按命中 token 计价。

典型例子：`wiki_summary` AFTER 的 `call_hit_rate=87.5%` 但 `token_cache_ratio=26.7%` —— 7/8 调用有命中，但每个 summary 只命中了静态 `<instructions>` 前缀（约 500 token），动态 document 部分仍按 miss 计费。

分层结论：
- **wiki_summary**：AFTER 把静态规则前置后，稳定前缀命中（BEFORE 的 `<document>` 在前导致无共享前缀，恒 0 hit）。
- **wiki_page_modify**：命中来自 warmup 串行化（第一个 page 写 cache，后续并发复用 `SharedSourceContexts`），BEFORE/AFTER 共有。
- **wiki_candidate_slug / wiki_chunk_citation**：静态规则较长（~2000 token），预热后 BEFORE/AFTER 都能被 DeepSeek 的 common-prefix 检测命中。

---

## 8. Causal Interpretation（因果归因）

| 收益来源 | 证据 | 归因 |
|---|---|---|
| **Summary 静态规则前置** | `wiki_summary` 0→87.5% hit（BEFORE 恒 0，两对一致） | **归因于本 commit**（`WikiSummaryPrompt` reorder） |
| **CandidateSlug 静态规则前置** | 冷启动 0→25%，预热后都 100% | 部分归因本 commit（冷启动有效，预热后被抹平） |
| **PageModify 高命中** | 91–100% hit，BEFORE/AFTER 共有 | **不归因本 commit**（既有 `awaitWikiPromptWarmup` + 稳定 `SharedSourceContexts`） |
| **stable image placeholders** | corpus 纯文本无图片 | 本实验**无法验证**（无 image URL 需要 mask） |
| **PromptCacheKey 用 resolved model name** | 不改变 DeepSeek 实际请求内容 | 对 DeepSeek **无直接收益**（automatic prefix caching 不看 key） |

**page 数差异**：BEFORE 59 pages vs AFTER 49 pages，源于两种 prompt 结构下 candidate 提取的 LLM 差异（非 cache）。这使 `wiki_page_modify` 的绝对 token/cost 不可直接比较；但 `call_hit_rate` / `token_cache_ratio` 是相对指标，结论仍成立。

---

## 9. Limitations

1. **provider automatic cache 不可控**：DeepSeek cache 是 provider-native，无法本地清除或强制冷态；cache 单元在多次请求后持久化，导致 Pair 2（预热后）BEFORE/AFTER 的 candidate_slug/chunk_citation 差异被抹平。marker（在 `ContentInstructions`）无法隔离共享的静态 `<instructions>` 前缀。
2. **并发时序噪声**：`map_parallel=10` 使同文档/跨文档的 candidate_slug 几乎同时发出，cache 命中率受「首请求写完 cache 前后续是否到达」影响，逐 trial 波动大。
3. **page 数不一致**：LLM 提取非确定，BEFORE 59 vs AFTER 49 pages，引入跨层绝对量的混淆。
4. **成本不可得**：`model_pricing` 空，无 DB cost truth。
5. **latency 波动大**：受 DeepSeek 生成长度主导，无法从 latency 维度可靠区分 BEFORE/AFTER。
6. **corpus 规模小**：4 文档、纯文本、无图片，未覆盖「多图文档稳定 placeholder」这一本 commit 的另一个目标场景。

---

## 10. Acceptance Mapping

**问题：Wiki 生成阶段缓存命中率的提升是否有真实证据？**

有**部分真实证据**，集中在 `wiki_summary` 层：
- BEFORE `wiki_summary` 恒为 0 cache hit（document 前置 → 无共享前缀）。
- AFTER `wiki_summary` 两对 trial 分别为 3/4、4/4 hit（静态规则前置 → 稳定前缀命中）。

但 `wiki_page_modify`（原预期的核心层）的提升**不是本 commit 带来**：它的高命中来自既有的 warmup 机制，BEFORE 同样 100% hit。因此结论是「**MODEST**」，不是「STRONG」。若验收口径只看 `wiki_page_modify` 的 cache 命中率，则本 commit 在该层无可归因收益；若口径包含 `wiki_summary` 的稳定前缀复用，则有真实证据。

---

## 11. Artifact Paths

```
artifacts/wiki_prompt_cache_v1/
├── README.md
├── experiment_config.json
├── before_trials.json
├── after_trials.json
├── comparison.json
└── raw_queries.sql
```

未保存：API key、Authorization header、完整敏感 prompt。

---

## 12. Git State

- 实验期间生产代码**未做任何修改**，未 commit、未 push。
- 使用 `git worktree add /tmp/weknora-before 3f9a054` 编译 BEFORE 二进制（`/tmp/weknora-server-before`），主工作区保持 AFTER（`22f9120`）不变。
- 实验专用 KB 已全部删除；`model_usage` 中新增的是本实验产生的 `wiki_*` 观测行（属数据、非代码）。
