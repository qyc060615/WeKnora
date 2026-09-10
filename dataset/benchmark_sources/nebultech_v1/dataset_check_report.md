# NebulaTech RAG Benchmark 数据集检查报告

## 总体统计

- 文档数量：10
- QA 数量：30
- Single-hop 数量：20
- Hard-negative 数量：5
- Multi-hop 数量：5

## 文档规格检查

- `employee_handbook.md`：1096 个中文汉字；核心 Benchmark 事实 12 条。
- `expense_policy.md`：1017 个中文汉字；核心 Benchmark 事实 12 条。
- `database_policy.md`：1033 个中文汉字；核心 Benchmark 事实 13 条。
- `deployment_manual.md`：905 个中文汉字；核心 Benchmark 事实 12 条。
- `incident_response.md`：956 个中文汉字；核心 Benchmark 事实 12 条。
- `security_policy.md`：888 个中文汉字；核心 Benchmark 事实 12 条。
- `api_guidelines.md`：833 个中文汉字；核心 Benchmark 事实 13 条。
- `product_manual.md`：886 个中文汉字；核心 Benchmark 事实 12 条。
- `backup_recovery.md`：843 个中文汉字；核心 Benchmark 事实 12 条。
- `oncall_manual.md`：918 个中文汉字；核心 Benchmark 事实 13 条。

- 10 份文档是否全部落在 800～1500 中文汉字：是
- 每份文档核心 Benchmark 事实是否全部落在 8～15 条：是

## 每个文档被引用的问题数

- `employee_handbook.md`：2 题
- `expense_policy.md`：3 题
- `database_policy.md`：4 题
- `deployment_manual.md`：4 题
- `incident_response.md`：5 题
- `security_policy.md`：4 题
- `api_guidelines.md`：4 题
- `product_manual.md`：3 题
- `backup_recovery.md`：4 题
- `oncall_manual.md`：2 题

## 一致性与覆盖检查

- 是否存在没有被任何问题覆盖的文档：否
- 是否存在被少于 2 个问题覆盖的文档：否
- 是否存在相同问题对应多个互相冲突答案：否
- relevant_sections 是否都能在对应文档标题中精确定位：是
- qid 是否连续且唯一：是

## Hard Negative 设计覆盖

- 数据库环境：Development / Testing / Production 使用高度相似的 Schema 变更词汇，但审批要求分别为无需人工审批、Team Lead、DBA + 当周值班 SRE。
- 发布环境：Development / Staging / Production 均出现“发布、审批、环境”等词，但审批和验证要求不同。
- 差旅：国内一线城市 / 国内其他城市 / 海外酒店标准共享“酒店、上限、每晚”等词，但金额不同。
- 凭证：Password / API Key / SSH Key 均属于认证凭证，但长度、有效期、存储和泄露处置规则不同。
- 数据副本：Production Backup / Staging Snapshot / Archive 均涉及备份与保留，但用途和保留周期不同。

## Ground Truth 使用注意

- `relevant_documents` 是文档级标注，`relevant_sections` 是章节级标注。导入 WeKnora 后若要严格计算 Chunk-level Precision、Recall、MRR、NDCG、MAP，应把实际生成的 `chunk_id` 再映射到这些章节，不能把文档命中直接当成 Chunk 命中。
- BLEU / ROUGE 可以直接对 `answer` 做生成答案对比，但建议同时保留 Exact Match 或基于规范化文本的 F1，因为短事实答案仅用 BLEU 容易失真。
- Multi-hop 题的全部 `relevant_documents` 都应视为必要证据；只命中其中一个文档不应按完整检索成功计分。
