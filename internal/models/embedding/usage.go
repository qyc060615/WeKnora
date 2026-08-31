package embedding

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/Tencent/WeKnora/internal/models/usage"
	"github.com/Tencent/WeKnora/internal/types"
)

// usageSpan is the per-logical-invocation accounting threaded through the
// embedder decorator chain via context. The outermost usage wrapper seeds it;
// the innermost concurrency wrapper counts provider round-trips; the caching
// wrapper records its cache accounting into it; providers that return native
// token usage accumulate it here.
type usageSpan struct {
	providerRequests atomic.Int64
	cacheSummary     *cacheRequestSummary
	inputTokens      atomic.Int64
	totalTokens      atomic.Int64
	inputReported    atomic.Bool
	totalReported    atomic.Bool
}

type usageSpanKey struct{}

func withUsageSpan(ctx context.Context) (context.Context, *usageSpan) {
	span := &usageSpan{}
	return context.WithValue(ctx, usageSpanKey{}, span), span
}

func spanFromContext(ctx context.Context) *usageSpan {
	span, _ := ctx.Value(usageSpanKey{}).(*usageSpan)
	return span
}

// noteEmbeddingProviderRequest records one outbound HTTP attempt on the
// per-invocation span. The shared transport calls it for every httpClient.Do
// (retries included); the Ollama local embedder, which bypasses the shared
// transport, calls it directly before its SDK round-trip.
func noteEmbeddingProviderRequest(ctx context.Context) {
	if span := spanFromContext(ctx); span != nil {
		span.providerRequests.Add(1)
	}
}

// noteEmbeddingTokens records provider-reported token usage. A nil pointer means
// that token field was absent from the provider response; a non-nil pointer is
// provider_reported even when its value is zero. The value is never used to
// infer presence.
func noteEmbeddingTokens(ctx context.Context, inputTokens, totalTokens *int) {
	span := spanFromContext(ctx)
	if span == nil {
		return
	}
	if inputTokens != nil {
		span.inputTokens.Add(int64(*inputTokens))
		span.inputReported.Store(true)
	}
	if totalTokens != nil {
		span.totalTokens.Add(int64(*totalTokens))
		span.totalReported.Store(true)
	}
}

// usageEmbedder records one model_usage row per logical embedding invocation
// (Embed / BatchEmbed / BatchEmbedWithPool). It is the OUTERMOST decorator, so
// latency includes cache read/write and the provider round-trips.
type usageEmbedder struct {
	inner             Embedder
	config            Config
	resolvedModelName *string
}

func wrapEmbeddingUsage(e Embedder, config Config, resolvedModelName *string) Embedder {
	if e == nil {
		return e
	}
	return &usageEmbedder{inner: e, config: config, resolvedModelName: resolvedModelName}
}

func (w *usageEmbedder) GetModelName() string { return w.inner.GetModelName() }
func (w *usageEmbedder) GetDimensions() int   { return w.inner.GetDimensions() }
func (w *usageEmbedder) GetModelID() string   { return w.inner.GetModelID() }

func (w *usageEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	ctx, span := withUsageSpan(ctx)
	start := time.Now()
	vec, err := w.inner.Embed(ctx, text)
	w.record(ctx, span, start, 1, err, true)
	return vec, err
}

func (w *usageEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, span := withUsageSpan(ctx)
	start := time.Now()
	vecs, err := w.inner.BatchEmbed(ctx, texts)
	w.record(ctx, span, start, len(texts), err, true)
	return vecs, err
}

func (w *usageEmbedder) BatchEmbedWithPool(ctx context.Context, model Embedder, texts []string) ([][]float32, error) {
	ctx, span := withUsageSpan(ctx)
	start := time.Now()
	vecs, err := w.inner.BatchEmbedWithPool(ctx, model, texts)
	// A failed pooled call may have sent only some sub-batches. Without
	// per-sub-batch input accounting, the total cannot be stated reliably.
	w.record(ctx, span, start, len(texts), err, false)
	return vecs, err
}

func (w *usageEmbedder) record(
	ctx context.Context, span *usageSpan, start time.Time, inputs int, err error, failedInputsKnown bool,
) {
	latencyMS := time.Since(start).Milliseconds()
	mu := &types.ModelUsage{
		ModelTenantID:     w.config.TenantID,
		ModelID:           w.config.ModelID,
		ModelName:         w.config.ModelName,
		ModelType:         string(w.config.Type),
		ModelSource:       string(w.config.Source),
		ResolvedProvider:  usage.ResolveProvider(w.config.Provider, w.config.BaseURL),
		ResolvedModelName: w.resolvedModelName,
		CallType:          types.CallTypeEmbedding,
		// Purpose is deliberately left empty here: the recorder fills it from a
		// context-carried purpose ("query_embedding" / "index_embedding") set by
		// the caller, so unknown embedding paths are never mislabelled as index.
		Status:           usage.StatusFromError(err),
		TokenProvenance:  types.TokenProvenanceUnreported,
		LatencyMS:        &latencyMS,
		StartedAt:        &start,
		LogicalRequests:  1,
		EmbeddingInputs:  inputs,
		ProviderRequests: int(span.providerRequests.Load()),
	}

	if s := span.cacheSummary; s != nil {
		status := cacheStatusFromSummary(s)
		mu.EmbeddingCacheStatus = &status
		mu.EmbeddingInputs = s.inputs
		mu.CacheHits = s.hits
		mu.CacheMisses = s.misses
		mu.ProviderInputs = s.providerInputs
		if s.readError {
			mu.CacheReadErrors = 1
		}
		if s.writeError {
			mu.CacheWriteErrors = 1
		}
	} else {
		// Cache disabled or bypassed: no cache accounting exists, so every
		// input (on success) reached the provider and no cache counter applies.
		status := types.EmbeddingCacheStatusDisabled
		mu.EmbeddingCacheStatus = &status
		// Successful calls sent the full logical input set. For failed direct
		// Embed/BatchEmbed calls, an observed outbound attempt proves the single
		// provider batch carried all inputs. Failed pooled fan-out is left at 0
		// because only a subset of sub-batches may have reached the provider.
		if err == nil || (failedInputsKnown && span.providerRequests.Load() > 0) {
			mu.ProviderInputs = inputs
		}
	}
	if err != nil && !failedInputsKnown && span.providerRequests.Load() > 0 {
		// A failed pooled fan-out exposes request attempts but not which
		// sub-batches were sent. Preserve unknown as the conservative zero fact;
		// the Pricing v1 calculator recognizes this shape as an unknown meter.
		mu.ProviderInputs = 0
	}

	// Provider-reported native token usage (Aliyun total_tokens, Volcengine
	// prompt_tokens + total_tokens). output_tokens stays NULL.
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

func cacheStatusFromSummary(s *cacheRequestSummary) types.EmbeddingCacheStatus {
	if s.hits == s.inputs && s.misses == 0 {
		return types.EmbeddingCacheStatusFullHit
	}
	if s.hits > 0 {
		return types.EmbeddingCacheStatusPartial
	}
	return types.EmbeddingCacheStatusMiss
}
