package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAggregateEvaluationRunSingleCallTypes(t *testing.T) {
	for _, callType := range []types.CallType{types.CallTypeChat, types.CallTypeEmbedding, types.CallTypeRerank} {
		t.Run(string(callType), func(t *testing.T) {
			db := newModelUsageFKTestDB(t)
			runID := uuid.NewString()
			insertEvaluationRun(t, db, runID, 41)
			repo := NewModelUsageRepository(db)
			usage := aggregationUsage(41, runID, callType)
			zero := 0
			usage.InputTokens = &zero
			if callType == types.CallTypeChat {
				usage.OutputTokens = &zero
			}
			require.NoError(t, repo.Create(context.Background(), usage))

			got, err := repo.AggregateEvaluationRun(context.Background(), 41, runID)
			require.NoError(t, err)
			require.Equal(t, int64(1), got.Calls.Total)
			require.Equal(t, int64(1), callCountFor(got.Calls, callType))
			require.Equal(t, int64(1), got.InputTokens.ObservedCalls)
			require.NotNil(t, got.InputTokens.Sum)
			require.Zero(t, *got.InputTokens.Sum, "explicit zero must remain observed")
			require.Equal(t, int64(1), got.NoCostRowCalls.Total)
		})
	}
}

func TestAggregateEvaluationRunNullCoverageStatusesAndIsolation(t *testing.T) {
	db := newModelUsageFKTestDB(t)
	repo := NewModelUsageRepository(db)
	ctx := context.Background()
	runID, otherRunID := uuid.NewString(), uuid.NewString()
	insertEvaluationRun(t, db, runID, 51)
	insertEvaluationRun(t, db, otherRunID, 51)

	chatUnknown := aggregationUsage(51, runID, types.CallTypeChat)
	chatUnknown.LatencyMS = nil
	require.NoError(t, repo.Create(ctx, chatUnknown))

	chatZero := aggregationUsage(51, runID, types.CallTypeChat)
	zero := 0
	zeroMS := int64(0)
	unreported := types.PromptCacheStatusUnreported
	chatZero.InputTokens, chatZero.OutputTokens, chatZero.TotalTokens = &zero, &zero, &zero
	chatZero.CacheReadTokens, chatZero.CacheWriteTokens, chatZero.CacheMissTokens = &zero, &zero, &zero
	chatZero.PromptCacheStatus, chatZero.LatencyMS = &unreported, &zeroMS
	chatZero.ProviderRequests = 2
	require.NoError(t, repo.Create(ctx, chatZero))

	embeddingUnknown := aggregationUsage(51, runID, types.CallTypeEmbedding)
	embeddingUnknown.LatencyMS = nil
	require.NoError(t, repo.Create(ctx, embeddingUnknown))
	embeddingDisabled := aggregationUsage(51, runID, types.CallTypeEmbedding)
	disabled := types.EmbeddingCacheStatusDisabled
	embeddingDisabled.EmbeddingCacheStatus = &disabled
	embeddingDisabled.ProviderRequests, embeddingDisabled.ProviderInputs = 1, 3
	embeddingDisabled.EmbeddingInputs, embeddingDisabled.CacheHits, embeddingDisabled.CacheMisses = 4, 1, 3
	require.NoError(t, repo.Create(ctx, embeddingDisabled))
	rerank := aggregationUsage(51, runID, types.CallTypeRerank)
	rerank.ProviderPairs, rerank.Queries, rerank.Documents, rerank.Pairs = 6, 1, 2, 2
	require.NoError(t, repo.Create(ctx, rerank))

	// Same tenant but another evaluation run must not leak into the target.
	require.NoError(t, repo.Create(ctx, aggregationUsage(51, otherRunID, types.CallTypeChat)))

	got, err := repo.AggregateEvaluationRun(ctx, 51, runID)
	require.NoError(t, err)
	require.Equal(t, types.CallCounts{Total: 5, Chat: 2, Embedding: 2, Rerank: 1}, got.Calls)
	require.Equal(t, int64(5), got.InputTokens.ApplicableCalls)
	require.Equal(t, int64(1), got.InputTokens.ObservedCalls)
	require.NotNil(t, got.InputTokens.Sum)
	require.Zero(t, *got.InputTokens.Sum)
	require.Equal(t, int64(2), got.OutputTokens.ApplicableCalls)
	require.Equal(t, int64(1), got.OutputTokens.ObservedCalls)
	require.Equal(t, int64(2), got.CacheReadTokens.ApplicableCalls)
	require.Equal(t, int64(1), got.CacheReadTokens.ObservedCalls)
	require.Equal(t, int64(1), got.PromptCache.NotRecorded)
	require.Equal(t, int64(1), got.PromptCache.Unreported)
	require.Equal(t, int64(1), got.EmbeddingCache.NotRecorded)
	require.Equal(t, int64(1), got.EmbeddingCache.Disabled)
	require.Equal(t, int64(3), got.Counters.ProviderRequests)
	require.Equal(t, int64(3), got.Counters.ProviderInputs)
	require.Equal(t, int64(6), got.Counters.ProviderPairs)
	require.Equal(t, int64(4), got.Counters.EmbeddingInputs)
	require.Equal(t, int64(1), got.Counters.CacheHits)
	require.Equal(t, int64(3), got.Counters.CacheMisses)
	require.Equal(t, int64(1), got.Counters.Queries)
	require.Equal(t, int64(2), got.Counters.Documents)
	require.Equal(t, int64(2), got.Counters.Pairs)
	require.Equal(t, int64(5), got.Latency.ApplicableCalls)
	require.Equal(t, int64(3), got.Latency.ObservedCalls)
	require.NotNil(t, got.Latency.SumMS)
	require.Equal(t, int64(200), *got.Latency.SumMS)
	require.Equal(t, int64(100), *got.Latency.MaxMS)
	require.InDelta(t, 200.0/3.0, *got.Latency.AverageMS, 0.000001)

	wrongTenant, err := repo.AggregateEvaluationRun(ctx, 52, runID)
	require.NoError(t, err)
	require.Zero(t, wrongTenant.Calls.Total)
	require.Empty(t, wrongTenant.CostByCurrency)
}

func TestAggregateEvaluationRunCostsAreExactAndCurrencyGrouped(t *testing.T) {
	db := newModelUsageFKTestDB(t)
	repo := NewModelUsageRepository(db)
	ctx := context.Background()
	runID := uuid.NewString()
	insertEvaluationRun(t, db, runID, 61)

	chatOne := aggregationUsage(61, runID, types.CallTypeChat)
	chatTwo := aggregationUsage(61, runID, types.CallTypeChat)
	embedding := aggregationUsage(61, runID, types.CallTypeEmbedding)
	rerank := aggregationUsage(61, runID, types.CallTypeRerank)
	chatUnpriced := aggregationUsage(61, runID, types.CallTypeChat)
	for _, usage := range []*types.ModelUsage{chatOne, chatTwo, embedding, rerank, chatUnpriced} {
		require.NoError(t, repo.Create(ctx, usage))
	}
	insertAggregationCost(t, db, chatOne.ID, types.CostStatusPriced, "TEST", decPtr("0.100000000000000001"), decPtr("0.100000000000000001"))
	insertAggregationCost(t, db, chatTwo.ID, types.CostStatusPriced, "TEST", decPtr("0.200000000000000002"), decPtr("0.200000000000000002"))
	insertAggregationCost(t, db, rerank.ID, types.CostStatusPartial, "TEST", nil, decPtr("0.3"))
	insertAggregationCost(t, db, chatUnpriced.ID, types.CostStatusUnpriced, "CNY", nil, nil)

	got, err := repo.AggregateEvaluationRun(ctx, 61, runID)
	require.NoError(t, err)
	require.Equal(t, types.CallCounts{Total: 1, Embedding: 1}, got.NoCostRowCalls)
	require.Len(t, got.CostByCurrency, 2)
	require.Equal(t, "CNY", got.CostByCurrency[0].Currency, "currency ordering must be deterministic")
	require.Equal(t, int64(1), got.CostByCurrency[0].Chat.UnpricedCalls)
	require.Equal(t, types.Decimal("0"), got.CostByCurrency[0].Total.PricedCost)

	testCost := got.CostByCurrency[1]
	require.Equal(t, "TEST", testCost.Currency)
	require.Equal(t, types.Decimal("0.300000000000000003"), testCost.Chat.PricedCost)
	require.Equal(t, types.Decimal("0.300000000000000003"), testCost.Chat.KnownCost)
	require.Equal(t, int64(2), testCost.Chat.PricedCalls)
	require.Equal(t, types.Decimal("0.3"), testCost.Rerank.KnownCost)
	require.Equal(t, int64(1), testCost.Rerank.PartialCalls)
	require.Equal(t, types.Decimal("0.600000000000000003"), testCost.Total.KnownCost)
	require.Equal(t, types.Decimal("0.300000000000000003"), testCost.Total.PricedCost)
}

func TestAggregateEvaluationRunEmpty(t *testing.T) {
	db := newModelUsageFKTestDB(t)
	runID := uuid.NewString()
	insertEvaluationRun(t, db, runID, 71)
	got, err := NewModelUsageRepository(db).AggregateEvaluationRun(context.Background(), 71, runID)
	require.NoError(t, err)
	require.Zero(t, got.Calls.Total)
	require.Nil(t, got.InputTokens.Sum)
	require.Nil(t, got.Latency.SumMS)
	require.Empty(t, got.CostByCurrency)
}

func TestAggregateEvaluationRunPostgreSQLQueryAndExactScan(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
	})
	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	require.NoError(t, err)

	columns := []string{
		"call_type", "input_tokens", "output_tokens", "total_tokens", "latency_ms",
		"prompt_cache_status", "cache_read_tokens", "cache_write_tokens", "cache_miss_tokens",
		"embedding_cache_status", "logical_requests", "provider_requests", "provider_inputs",
		"provider_pairs", "embedding_inputs", "cache_hits", "cache_misses", "cache_read_errors",
		"cache_write_errors", "queries", "documents", "pairs", "cost_id", "cost_status",
		"cost_currency", "total_cost", "known_cost",
	}
	rows := sqlmock.NewRows(columns).AddRow(
		"chat", 1, 2, 3, 4,
		nil, nil, nil, nil,
		nil, 1, 1, 0,
		0, 0, 0, 0, 0,
		0, 0, 0, 0, "cost-pg", "priced",
		"TEST", "0.100000000000000001", "0.100000000000000001",
	)
	mock.ExpectQuery(`(?s)SELECT u\.call_type,.*LEFT JOIN model_usage_cost AS c ON c\.usage_id = u\.id.*u\.tenant_id = \$1 AND u\.evaluation_run_id = \$2.*ORDER BY u\.id ASC`).
		WithArgs(int64(91), "run-pg").WillReturnRows(rows)

	got, err := NewModelUsageRepository(db).AggregateEvaluationRun(context.Background(), 91, "run-pg")
	require.NoError(t, err)
	require.Equal(t, types.CallCounts{Total: 1, Chat: 1}, got.Calls)
	require.Equal(t, types.Decimal("0.100000000000000001"), got.CostByCurrency[0].Total.PricedCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func aggregationUsage(tenantID uint64, runID string, callType types.CallType) *types.ModelUsage {
	usage := testModelUsage(tenantID, uuid.NewString())
	usage.EvaluationRunID = &runID
	usage.CallType = callType
	usage.LatencyMS = int64Ptr(100)
	switch callType {
	case types.CallTypeEmbedding:
		usage.ModelType = string(types.ModelTypeEmbedding)
	case types.CallTypeRerank:
		usage.ModelType = string(types.ModelTypeRerank)
	}
	return usage
}

func callCountFor(counts types.CallCounts, callType types.CallType) int64 {
	switch callType {
	case types.CallTypeChat:
		return counts.Chat
	case types.CallTypeEmbedding:
		return counts.Embedding
	default:
		return counts.Rerank
	}
}

func decPtr(value string) *types.Decimal {
	decimal := types.Decimal(value)
	return &decimal
}

func insertAggregationCost(t *testing.T, db *gorm.DB, usageID string, status types.CostStatus, currency string, total, known *types.Decimal) {
	t.Helper()
	require.NoError(t, db.Create(&types.ModelUsageCost{
		UsageID: usageID, Status: status, Currency: &currency, TotalCost: total, KnownCost: known,
		PricingSnapshot: types.JSON(`{}`), CalculatorVersion: "test", CalculatedAt: time.Now().UTC(),
	}).Error)
}
