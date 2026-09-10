# 确定性验证

## Go

Command:

```text
env GOCACHE=/tmp/weknora-go-cache go test -p 1 -count=1 ./internal/application/service ./internal/regression ./internal/models/embedding ./internal/models/chat ./internal/application/repository ./internal/handler -run 'TestBenchmarkV1Integrity|TestWikiPromptCache|TestWikiPromptPurposeMapping|TestCompareRegression|TestEmbeddingCacheDisabledAndNilRedisPreserveProvider|TestEmbeddingCacheStatsSharedAcrossDecorators|TestBuildOutbound_WikiExplicitPromptCacheKey|TestModelUsageAnalytics|TestAggregateAnalytics'
```

结果：6 个 package 均 PASS。

## Frontend analytics contract

Command:

```text
npm test -- src/api/modelUsageAnalytics.test.ts src/views/settings/components/modelUsageAnalyticsHelpers.test.ts
```

结果：14/14 tests PASS。

## Artifact 与 Git 检查

- `artifacts/rhino_2026_final/` 下全部 JSON 可解析；
- secret-pattern scan 未发现 API key、Authorization header 或 `api_key` field；
- `git diff --check`: PASS；
- acceptance 阶段 production code change：NO。
