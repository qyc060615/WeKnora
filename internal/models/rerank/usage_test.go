package rerank

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/models/usage"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type fakeUsageRepo struct {
	mu     sync.Mutex
	usages []*types.ModelUsage
}

func (f *fakeUsageRepo) Create(_ context.Context, u *types.ModelUsage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.usages = append(f.usages, u)
	return nil
}

func (f *fakeUsageRepo) last() *types.ModelUsage {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.usages) == 0 {
		return nil
	}
	return f.usages[len(f.usages)-1]
}

func (f *fakeUsageRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.usages)
}

func testRerankConfig() RerankerConfig {
	return RerankerConfig{
		ModelID: "rerank-model", ModelName: "rerank-safe", Type: types.ModelTypeRerank,
		Source: types.ModelSourceOpenAI, Provider: "openai", TenantID: 10000,
	}
}

func tenantCtx(id uint64) context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, id)
}

// fakeReranker notes one provider request carrying the full document set.
type fakeReranker struct {
	batchCalls int
}

func (f *fakeReranker) Rerank(ctx context.Context, _ string, documents []string) ([]RankResult, error) {
	// Simulate internal batching: this provider splits into f.batchCalls
	// outbound requests, each carrying the documents evenly.
	per := len(documents)
	if f.batchCalls > 1 {
		per = len(documents) / f.batchCalls
	}
	for i := 0; i < f.batchCalls; i++ {
		noteProviderRequest(ctx, per)
	}
	return []RankResult{{Index: 0, RelevanceScore: 0.9}}, nil
}

func (f *fakeReranker) GetModelName() string { return "fake" }
func (f *fakeReranker) GetModelID() string   { return "fake-id" }

func TestRerankUsageSingleRequest(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)

	resolved := "provider-rerank"
	w, err := wrapRerankUsage(&fakeReranker{batchCalls: 1}, ptr(testRerankConfig()), &resolved, nil)
	require.NoError(t, err)

	docs := []string{"d1", "d2", "d3", "d4"}
	before := time.Now()
	_, err = w.Rerank(tenantCtx(1), "q", docs)
	after := time.Now()
	require.NoError(t, err)

	u := repo.last()
	require.NotNil(t, u)
	require.Equal(t, types.CallTypeRerank, u.CallType)
	require.Equal(t, uint64(1), u.TenantID)
	require.Equal(t, uint64(10000), u.ModelTenantID)
	require.Equal(t, "provider-rerank", *u.ResolvedModelName)
	require.False(t, u.StartedAt.Before(before))
	require.False(t, u.StartedAt.After(after))
	require.Equal(t, 1, u.Queries)
	require.Equal(t, 4, u.Documents)
	require.Equal(t, 4, u.Pairs)
	require.Equal(t, 1, u.ProviderRequests)
	require.Equal(t, 4, u.ProviderPairs)
	require.Equal(t, "rerank", u.Purpose)
	require.Equal(t, types.TokenProvenanceUnreported, u.TokenProvenance)
	require.Equal(t, 1, u.LogicalRequests)
}

func TestRerankUsageInternalBatching(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)

	w, _ := wrapRerankUsage(&fakeReranker{batchCalls: 2}, ptr(testRerankConfig()), nil, nil)
	docs := []string{"d1", "d2", "d3", "d4"}
	_, err := w.Rerank(tenantCtx(1), "q", docs)
	require.NoError(t, err)

	u := repo.last()
	require.Equal(t, 2, u.ProviderRequests)
	require.Equal(t, 4, u.ProviderPairs)
}

func TestRerankUsageSecondInvocationIsSecondRow(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)

	w, _ := wrapRerankUsage(&fakeReranker{batchCalls: 1}, ptr(testRerankConfig()), nil, nil)
	_, err := w.Rerank(tenantCtx(1), "q", []string{"a"})
	require.NoError(t, err)
	_, err = w.Rerank(tenantCtx(1), "q", []string{"a"})
	require.NoError(t, err)

	require.Equal(t, 2, repo.count(), "a threshold-degradation re-run is a second logical row")
}

func TestRerankUsageNativeTokens(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)

	w, err := wrapRerankUsage(&fakeReranker{batchCalls: 1}, ptr(testRerankConfig()), nil, nil)
	require.NoError(t, err)
	ctx, span := withRerankSpan(tenantCtx(1))
	prompt, total := 10, 25
	noteRerankTokens(ctx, &prompt, &total) // Zhipu-style prompt + total tokens
	start := time.Date(2026, 8, 31, 1, 2, 3, 4, time.UTC)
	w.(*usageReranker).record(ctx, span, start, 2, nil)

	u := repo.last()
	require.Equal(t, start, *u.StartedAt)
	require.Equal(t, types.TokenProvenanceProviderReported, u.TokenProvenance)
	require.Equal(t, 10, *u.InputTokens)
	require.Equal(t, 25, *u.TotalTokens)
	require.Nil(t, u.OutputTokens, "rerank has no output tokens")
}

func TestRerankUsageReportedZeroIsPresent(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)
	w, _ := wrapRerankUsage(&fakeReranker{batchCalls: 1}, ptr(testRerankConfig()), nil, nil)
	ctx, span := withRerankSpan(tenantCtx(1))
	zero := 0
	noteRerankTokens(ctx, &zero, &zero)
	w.(*usageReranker).record(ctx, span, time.Now(), 1, nil)
	require.Equal(t, types.TokenProvenanceProviderReported, repo.last().TokenProvenance)
	require.Equal(t, 0, *repo.last().InputTokens)
	require.Equal(t, 0, *repo.last().TotalTokens)
}

func TestEffectiveRerankModelNameUsesProviderDefault(t *testing.T) {
	r := &LKEAPReranker{modelName: LKEAPDefaultRerankModel}
	require.Equal(t, LKEAPDefaultRerankModel, *effectiveRerankModelName(r))
}

func TestRerankUsageEvaluationAttribution(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)

	w, _ := wrapRerankUsage(&fakeReranker{batchCalls: 1}, ptr(testRerankConfig()), nil, nil)
	ctx := types.WithEvaluationRunID(tenantCtx(1), "run-3")
	_, err := w.Rerank(ctx, "q", []string{"a"})
	require.NoError(t, err)

	u := repo.last()
	require.NotNil(t, u.EvaluationRunID)
	require.Equal(t, "run-3", *u.EvaluationRunID)
}

func ptr[T any](v T) *T { return &v }
