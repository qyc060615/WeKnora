package repository

import (
	"context"
	"os"
	"testing"
	"time"

	modelpricing "github.com/Tencent/WeKnora/internal/models/pricing"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPricingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + uuid.NewString() + "?mode=memory&cache=shared&_foreign_keys=on"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, path := range []string{
		"../../../migrations/sqlite/000013_evaluation_runs.up.sql",
		"../../../migrations/sqlite/000014_model_usage.up.sql",
		"../../../migrations/sqlite/000015_model_pricing.up.sql",
	} {
		ddl, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(ddl)).Error)
	}
	return db
}

func fakePricingRule(from time.Time, to *time.Time, version string) *types.ModelPricing {
	zero := types.Decimal("0")
	return &types.ModelPricing{
		ResolvedProvider: "fake", ResolvedModelName: "fake-model", CallType: types.CallTypeChat,
		Currency: "USD", BillingMode: types.BillingModeChatStandardTokens,
		InputTokenPrice: &zero, OutputTokenPrice: &zero, UnitScale: "1000",
		EffectiveFrom: from, EffectiveTo: to, PricingVersion: version, SourceName: "deterministic-test",
	}
}

func TestPricingResolverBoundariesAndOverlap(t *testing.T) {
	db := newPricingTestDB(t)
	repo := NewPricingRepository(db)
	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(24 * time.Hour)
	r1 := fakePricingRule(t0, &t1, "v1")
	r2 := fakePricingRule(t1, nil, "v2")
	require.NoError(t, repo.CreatePricing(ctx, r1))
	require.NoError(t, repo.CreatePricing(ctx, r2), "touching half-open intervals must be allowed")

	got, err := repo.ResolvePricing(ctx, "fake", "fake-model", types.CallTypeChat, t0)
	require.NoError(t, err)
	require.Equal(t, "v1", got.PricingVersion, "effective_from is inclusive")
	got, err = repo.ResolvePricing(ctx, "fake", "fake-model", types.CallTypeChat, t1)
	require.NoError(t, err)
	require.Equal(t, "v2", got.PricingVersion, "effective_to is exclusive")
	got, err = repo.ResolvePricing(ctx, "fake", "fake-model", types.CallTypeChat, t0.Add(-time.Nanosecond))
	require.NoError(t, err)
	require.Nil(t, got)

	overlapEnd := t1.Add(time.Hour)
	require.Error(t, repo.CreatePricing(ctx, fakePricingRule(t0.Add(time.Hour), &overlapEnd, "overlap")))
	require.Error(t, db.Create(fakePricingRule(t0.Add(2*time.Hour), &overlapEnd, "trigger-overlap")).Error,
		"database trigger must reject overlaps even when repository validation is bypassed")
}

func TestPricingPersistsNullAndExplicitZeroDistinctly(t *testing.T) {
	db := newPricingTestDB(t)
	repo := NewPricingRepository(db)
	rule := fakePricingRule(time.Now().UTC(), nil, "zero")
	rule.TotalTokenPrice = nil
	require.NoError(t, repo.CreatePricing(context.Background(), rule))
	var got types.ModelPricing
	require.NoError(t, db.First(&got, "id = ?", rule.ID).Error)
	require.NotNil(t, got.InputTokenPrice)
	require.Equal(t, types.Decimal("0"), *got.InputTokenPrice)
	require.Nil(t, got.TotalTokenPrice)
}

func TestSQLitePricingPreservesFixedPrecisionText(t *testing.T) {
	db := newPricingTestDB(t)
	repo := NewPricingRepository(db)
	rule := fakePricingRule(time.Now().UTC(), nil, "precise")
	tiny := types.Decimal("0.000000000000000001")
	rule.InputTokenPrice = &tiny
	require.NoError(t, repo.CreatePricing(context.Background(), rule))
	var got types.ModelPricing
	require.NoError(t, db.First(&got, "id = ?", rule.ID).Error)
	require.Equal(t, tiny, *got.InputTokenPrice)
}

func TestMissingResolvedIdentityPersistsUnpricedCost(t *testing.T) {
	db := newPricingTestDB(t)
	repo := NewPricingRepository(db)
	ctx := context.Background()
	usage := testModelUsage(1, "legacy-unknown")
	usage.ID = uuid.NewString()
	usage.ResolvedModelName = nil
	require.NoError(t, db.Create(usage).Error)
	require.NoError(t, modelpricing.NewProcessor(repo).Process(ctx, usage))
	cost, err := repo.GetCostByUsageID(ctx, usage.ID)
	require.NoError(t, err)
	require.Equal(t, types.CostStatusUnpriced, cost.Status)
	require.Nil(t, cost.TotalCost)
	require.Nil(t, cost.PricingRuleID)
}

func TestPersistedCostKeepsHistoricalSnapshot(t *testing.T) {
	db := newPricingTestDB(t)
	repo := NewPricingRepository(db)
	ctx := context.Background()
	start := time.Now().UTC().Truncate(time.Millisecond)
	one := types.Decimal("1")
	rule := fakePricingRule(start.Add(-time.Hour), nil, "v1")
	rule.InputTokenPrice, rule.OutputTokenPrice, rule.UnitScale = &one, &one, "1"
	require.NoError(t, repo.CreatePricing(ctx, rule))
	resolved := "fake-model"
	usage := testModelUsage(1, "priced")
	usage.ID, usage.ResolvedModelName, usage.StartedAt = uuid.NewString(), &resolved, &start
	usage.ResolvedProvider = "fake"
	usage.InputTokens, usage.OutputTokens = intPtr(2), intPtr(3)
	require.NoError(t, db.Create(usage).Error)
	require.NoError(t, modelpricing.NewProcessor(repo).Process(ctx, usage))

	cost, err := repo.GetCostByUsageID(ctx, usage.ID)
	require.NoError(t, err)
	require.Equal(t, types.Decimal("5"), *cost.TotalCost)
	require.Contains(t, string(cost.PricingSnapshot), `"pricing_version":"v1"`)

	ten := types.Decimal("10")
	require.NoError(t, db.Model(&types.ModelPricing{}).Where("id = ?", rule.ID).Update("input_token_price", ten).Error)
	still, err := repo.GetCostByUsageID(ctx, usage.ID)
	require.NoError(t, err)
	require.Equal(t, types.Decimal("5"), *still.TotalCost)
	require.Equal(t, cost.PricingSnapshot, still.PricingSnapshot)
}

func TestPricingMigrationParity(t *testing.T) {
	pg, err := os.ReadFile("../../../migrations/versioned/000093_model_pricing.up.sql")
	require.NoError(t, err)
	sq, err := os.ReadFile("../../../migrations/sqlite/000015_model_pricing.up.sql")
	require.NoError(t, err)
	for _, column := range []string{
		"resolved_provider", "resolved_model_name", "call_type", "currency", "billing_mode",
		"input_token_price", "output_token_price", "total_token_price", "cache_read_token_price",
		"cache_write_token_price", "per_request_price", "per_input_price", "per_pair_price",
		"unit_scale", "effective_from", "effective_to", "pricing_version", "source_name",
		"source_reference", "source_retrieved_at", "created_at",
	} {
		require.Contains(t, string(pg), column)
		require.Contains(t, string(sq), column)
	}
	for _, column := range []string{
		"usage_id", "status", "currency", "total_cost", "known_cost", "input_cost", "output_cost",
		"cache_read_cost", "cache_write_cost", "request_cost", "provider_input_cost",
		"provider_pair_cost", "pricing_rule_id", "pricing_version", "pricing_snapshot",
		"calculator_version", "calculated_at",
	} {
		require.Contains(t, string(pg), column)
		require.Contains(t, string(sq), column)
	}
	require.Contains(t, string(pg), "trg_model_pricing_no_overlap")
	require.Contains(t, string(sq), "trg_model_pricing_no_overlap_insert")
	require.NotContains(t, string(pg), "UPDATE model_usage SET resolved_model_name")
	require.NotContains(t, string(sq), "UPDATE model_usage SET resolved_model_name")
}
