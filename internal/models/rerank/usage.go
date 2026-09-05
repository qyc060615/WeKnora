package rerank

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/Tencent/WeKnora/internal/models/usage"
	"github.com/Tencent/WeKnora/internal/types"
)

// rerankUsageSpan is the per-logical-invocation accounting threaded through the
// reranker decorator chain via context. The outermost usage wrapper seeds it;
// each concrete provider notes its outbound requests (and the pairs they carry)
// on it, so internal batching and retries are counted near the request instead
// of being guessed from document counts.
type rerankUsageSpan struct {
	providerRequests atomic.Int64
	providerPairs    atomic.Int64
	inputTokens      atomic.Int64
	totalTokens      atomic.Int64
	inputReported    atomic.Bool
	totalReported    atomic.Bool
}

type rerankUsageSpanKey struct{}

func withRerankSpan(ctx context.Context) (context.Context, *rerankUsageSpan) {
	span := &rerankUsageSpan{}
	return context.WithValue(ctx, rerankUsageSpanKey{}, span), span
}

func rerankSpanFromContext(ctx context.Context) *rerankUsageSpan {
	span, _ := ctx.Value(rerankUsageSpanKey{}).(*rerankUsageSpan)
	return span
}

// noteProviderRequest records one outbound provider request carrying `pairs`
// query/document pairs. Providers call it immediately before an HTTP round-trip.
func noteProviderRequest(ctx context.Context, pairs int) {
	if span := rerankSpanFromContext(ctx); span != nil {
		span.providerRequests.Add(1)
		span.providerPairs.Add(int64(pairs))
	}
}

// noteRerankTokens records provider-reported token usage for the current
// logical rerank invocation. A non-nil zero is reported data; nil is absent.
func noteRerankTokens(ctx context.Context, promptTokens, totalTokens *int) {
	span := rerankSpanFromContext(ctx)
	if span == nil {
		return
	}
	if promptTokens != nil {
		span.inputTokens.Add(int64(*promptTokens))
		span.inputReported.Store(true)
	}
	if totalTokens != nil {
		span.totalTokens.Add(int64(*totalTokens))
		span.totalReported.Store(true)
	}
}

// usageReranker records one model_usage row per logical Rerank call. A second
// Rerank (e.g. threshold-degradation re-run) is a second logical invocation and
// therefore a second row.
type usageReranker struct {
	inner             Reranker
	config            RerankerConfig
	resolvedModelName *string
}

func wrapRerankUsage(r Reranker, config *RerankerConfig, resolvedModelName *string, err error) (Reranker, error) {
	if err != nil || r == nil || config == nil {
		return r, err
	}
	return &usageReranker{inner: r, config: *config, resolvedModelName: resolvedModelName}, nil
}

func (w *usageReranker) GetModelName() string { return w.inner.GetModelName() }
func (w *usageReranker) GetModelID() string   { return w.inner.GetModelID() }

func (w *usageReranker) Rerank(ctx context.Context, query string, documents []string) ([]RankResult, error) {
	ctx, span := withRerankSpan(ctx)
	start := time.Now()
	results, err := w.inner.Rerank(ctx, query, documents)
	w.record(ctx, span, start, len(documents), err)
	return results, err
}

func (w *usageReranker) record(ctx context.Context, span *rerankUsageSpan, start time.Time, docCount int, err error) {
	latencyMS := time.Since(start).Milliseconds()
	mu := &types.ModelUsage{
		ModelTenantID:     w.config.TenantID,
		ModelID:           w.config.ModelID,
		ModelName:         w.config.ModelName,
		ModelType:         string(w.config.Type),
		ModelSource:       string(w.config.Source),
		ResolvedProvider:  usage.ResolveProvider(w.config.Provider, w.config.BaseURL),
		ResolvedModelName: w.resolvedModelName,
		CallType:          types.CallTypeRerank,
		Purpose:           "rerank",
		Status:            usage.StatusFromError(err),
		TokenProvenance:   types.TokenProvenanceUnreported,
		LatencyMS:         &latencyMS,
		StartedAt:         &start,
		LogicalRequests:   1,
		Queries:           1,
		Documents:         docCount,
		Pairs:             docCount, // 1 query × N documents
		ProviderRequests:  int(span.providerRequests.Load()),
		ProviderPairs:     int(span.providerPairs.Load()),
	}
	// Provider-reported native token usage (Jina/Aliyun/OpenAI total_tokens,
	// Zhipu prompt_tokens + total_tokens). output_tokens stays NULL.
	if span.inputReported.Load() {
		mu.InputTokens = usage.IntPtr(int(span.inputTokens.Load()))
	}
	if span.totalReported.Load() {
		mu.TotalTokens = usage.IntPtr(int(span.totalTokens.Load()))
	}
	if span.inputReported.Load() || span.totalReported.Load() {
		mu.TokenProvenance = types.TokenProvenanceProviderReported
	}
	usage.Record(ctx, mu)
}
