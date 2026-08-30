package embedding

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

func testEmbeddingConfig() Config {
	return Config{
		ModelID: "embed-model", ModelName: "embed-safe", Type: types.ModelTypeEmbedding,
		Source: types.ModelSourceOpenAI, Provider: "openai", TenantID: 10000,
	}
}

func tenantCtx(id uint64) context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, id)
}

func TestCacheStatusFromSummary(t *testing.T) {
	require.Equal(t, types.EmbeddingCacheStatusFullHit,
		cacheStatusFromSummary(&cacheRequestSummary{inputs: 5, hits: 5, misses: 0}))
	require.Equal(t, types.EmbeddingCacheStatusPartial,
		cacheStatusFromSummary(&cacheRequestSummary{inputs: 5, hits: 3, misses: 2}))
	require.Equal(t, types.EmbeddingCacheStatusMiss,
		cacheStatusFromSummary(&cacheRequestSummary{inputs: 5, hits: 0, misses: 5}))
}

func TestEmbeddingUsageCacheDisabled(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)

	w := wrapEmbeddingUsage(&usageFakeEmbedder{}, testEmbeddingConfig())
	_, err := w.BatchEmbed(tenantCtx(1), []string{"a", "b", "c"})
	require.NoError(t, err)

	u := repo.last()
	require.NotNil(t, u)
	require.Equal(t, types.CallTypeEmbedding, u.CallType)
	require.Equal(t, uint64(1), u.TenantID)
	require.Equal(t, uint64(10000), u.ModelTenantID)
	require.Equal(t, 3, u.EmbeddingInputs)
	require.Equal(t, 3, u.ProviderInputs, "cache disabled: every input reaches the provider")
	require.Equal(t, 0, u.CacheHits)
	require.Equal(t, 0, u.CacheMisses)
	require.NotNil(t, u.EmbeddingCacheStatus)
	require.Equal(t, types.EmbeddingCacheStatusDisabled, *u.EmbeddingCacheStatus)
	require.Equal(t, types.TokenProvenanceUnreported, u.TokenProvenance)
	require.Equal(t, 1, u.LogicalRequests)
}

func TestEmbeddingUsageCacheAccounting(t *testing.T) {
	for _, tc := range []struct {
		name     string
		summary  *cacheRequestSummary
		status   types.EmbeddingCacheStatus
		hits     int
		misses   int
		provIn   int
		readErr  int
		writeErr int
	}{
		{"full hit", &cacheRequestSummary{inputs: 4, hits: 4, misses: 0, providerInputs: 0}, types.EmbeddingCacheStatusFullHit, 4, 0, 0, 0, 0},
		{"partial", &cacheRequestSummary{inputs: 4, hits: 1, misses: 3, providerInputs: 3}, types.EmbeddingCacheStatusPartial, 1, 3, 3, 0, 0},
		{"miss with errors", &cacheRequestSummary{inputs: 4, hits: 0, misses: 4, providerInputs: 4, readError: true, writeError: true}, types.EmbeddingCacheStatusMiss, 0, 4, 4, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeUsageRepo{}
			usage.SetRecorder(usage.NewRecorder(repo))
			defer usage.SetRecorder(nil)

			w := wrapEmbeddingUsage(&usageFakeEmbedder{}, testEmbeddingConfig()).(*usageEmbedder)
			span := &usageSpan{}
			span.cacheSummary = tc.summary
			w.record(tenantCtx(1), span, time.Now(), tc.summary.inputs, nil)

			u := repo.last()
			require.NotNil(t, u)
			require.Equal(t, tc.status, *u.EmbeddingCacheStatus)
			require.Equal(t, tc.hits, u.CacheHits)
			require.Equal(t, tc.misses, u.CacheMisses)
			require.Equal(t, tc.provIn, u.ProviderInputs)
			require.Equal(t, tc.readErr, u.CacheReadErrors)
			require.Equal(t, tc.writeErr, u.CacheWriteErrors)
		})
	}
}

func TestEmbeddingUsageProviderRequestCounter(t *testing.T) {
	span := &usageSpan{}
	ctx := context.WithValue(context.Background(), usageSpanKey{}, span)
	span.providerRequests.Add(1)
	span.providerRequests.Add(1)
	require.Equal(t, int64(2), span.providerRequests.Load())
	require.Equal(t, span, spanFromContext(ctx))
}

func TestEmbeddingUsageNativeTokens(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)

	w := wrapEmbeddingUsage(&usageFakeEmbedder{}, testEmbeddingConfig()).(*usageEmbedder)
	ctx, span := withUsageSpan(tenantCtx(1))
	noteEmbeddingTokens(ctx, 20, 30) // Volcengine-style prompt + total tokens
	w.record(ctx, span, time.Now(), 2, nil)

	u := repo.last()
	require.Equal(t, types.TokenProvenanceProviderReported, u.TokenProvenance)
	require.Equal(t, 20, *u.InputTokens)
	require.Equal(t, 30, *u.TotalTokens)
	require.Nil(t, u.OutputTokens, "embedding has no output tokens")
}

func TestEmbeddingUsageEvaluationAttributionAndPurpose(t *testing.T) {
	repo := &fakeUsageRepo{}
	usage.SetRecorder(usage.NewRecorder(repo))
	defer usage.SetRecorder(nil)

	w := wrapEmbeddingUsage(&usageFakeEmbedder{}, testEmbeddingConfig())
	ctx := types.WithEvaluationRunID(tenantCtx(1), "run-7")
	_, err := w.BatchEmbed(ctx, []string{"a"})
	require.NoError(t, err)

	u := repo.last()
	require.NotNil(t, u.EvaluationRunID)
	require.Equal(t, "run-7", *u.EvaluationRunID)
	require.Equal(t, "", u.Purpose, "unknown embedding must not be defaulted to index_embedding")
}

type usageFakeEmbedder struct{}

func (f *usageFakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{1}, nil
}
func (f *usageFakeEmbedder) BatchEmbed(context.Context, []string) ([][]float32, error) {
	return [][]float32{{1}}, nil
}
func (f *usageFakeEmbedder) BatchEmbedWithPool(_ context.Context, _ Embedder, _ []string) ([][]float32, error) {
	return [][]float32{{1}}, nil
}
func (f *usageFakeEmbedder) GetModelName() string { return "fake" }
func (f *usageFakeEmbedder) GetDimensions() int   { return 1 }
func (f *usageFakeEmbedder) GetModelID() string   { return "fake-id" }
