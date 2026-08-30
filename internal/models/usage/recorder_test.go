package usage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	mu     sync.Mutex
	usages []*types.ModelUsage
	err    error
}

func (f *fakeRepo) Create(_ context.Context, u *types.ModelUsage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.usages = append(f.usages, u)
	return nil
}

func (f *fakeRepo) last() *types.ModelUsage {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.usages) == 0 {
		return nil
	}
	return f.usages[len(f.usages)-1]
}

func (f *fakeRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.usages)
}

func TestRecordFillsAttribution(t *testing.T) {
	repo := &fakeRepo{}
	SetRecorder(NewRecorder(repo))
	defer SetRecorder(nil)

	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	ctx = types.WithEvaluationRunID(ctx, "run-9")
	ctx = types.WithLLMCallMetadata(ctx, "test_purpose", "")

	Record(ctx, &types.ModelUsage{Purpose: "default"})

	got := repo.last()
	require.NotNil(t, got)
	require.Equal(t, uint64(7), got.TenantID)
	require.NotNil(t, got.EvaluationRunID)
	require.Equal(t, "run-9", *got.EvaluationRunID)
	require.Equal(t, "test_purpose", got.Purpose, "context purpose must override the wrapper default")
}

func TestRecordSkipsWithoutTenant(t *testing.T) {
	repo := &fakeRepo{}
	SetRecorder(NewRecorder(repo))
	defer SetRecorder(nil)

	Record(context.Background(), &types.ModelUsage{})
	require.Equal(t, 0, repo.count())
}

func TestRecordSwallowsPersistenceError(t *testing.T) {
	repo := &fakeRepo{err: errors.New("db unavailable")}
	SetRecorder(NewRecorder(repo))
	defer SetRecorder(nil)

	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	// Must not panic and must not surface the error to the caller.
	Record(ctx, &types.ModelUsage{})
	require.Equal(t, 0, repo.count())
}

func TestResolveProvider(t *testing.T) {
	require.Equal(t, "openai", ResolveProvider("openai", ""))
	// A configured provider wins over base URL detection.
	require.Equal(t, "zhipu", ResolveProvider("zhipu", "https://api.openai.com/v1"))
}

func TestStatusFromError(t *testing.T) {
	require.Equal(t, types.UsageStatusSuccess, StatusFromError(nil))
	require.Equal(t, types.UsageStatusCancelled, StatusFromError(context.Canceled))
	require.Equal(t, types.UsageStatusTimeout, StatusFromError(context.DeadlineExceeded))
	require.Equal(t, types.UsageStatusError, StatusFromError(errors.New("provider 500")))
}

// ctxAwareRepo rejects a cancelled/deadline-exceeded persistence context, so
// the tests below genuinely prove the recorder detaches the model-call context
// before persisting.
type ctxAwareRepo struct {
	mu          sync.Mutex
	usages      []*types.ModelUsage
	gotCtxErr   error
	gotDeadline bool
}

func (f *ctxAwareRepo) Create(ctx context.Context, u *types.ModelUsage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotCtxErr = ctx.Err()
	if _, ok := ctx.Deadline(); ok {
		f.gotDeadline = true
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.usages = append(f.usages, u)
	return nil
}

func (f *ctxAwareRepo) last() *types.ModelUsage {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.usages) == 0 {
		return nil
	}
	return f.usages[len(f.usages)-1]
}

func (f *ctxAwareRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.usages)
}

func TestRecordPersistsDespiteCancelledContext(t *testing.T) {
	repo := &ctxAwareRepo{}
	SetRecorder(NewRecorder(repo))
	defer SetRecorder(nil)

	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	ctx = types.WithEvaluationRunID(ctx, "run-9")
	ctx = types.WithLLMCallMetadata(ctx, "test_purpose", "")
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	Record(ctx, &types.ModelUsage{Status: types.UsageStatusCancelled})

	require.Nil(t, repo.gotCtxErr, "persistence ctx must not inherit the model cancel")
	require.Equal(t, 1, repo.count())
	got := repo.last()
	require.Equal(t, uint64(7), got.TenantID)
	require.NotNil(t, got.EvaluationRunID)
	require.Equal(t, "run-9", *got.EvaluationRunID)
	require.Equal(t, "test_purpose", got.Purpose)
	require.Equal(t, types.UsageStatusCancelled, got.Status, "status must stay cancelled")
}

func TestRecordPersistsDespiteDeadlineExceeded(t *testing.T) {
	repo := &ctxAwareRepo{}
	SetRecorder(NewRecorder(repo))
	defer SetRecorder(nil)

	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(8))
	ctx = types.WithEvaluationRunID(ctx, "run-10")
	ctx = types.WithLLMCallMetadata(ctx, "p", "")
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer cancel()

	Record(ctx, &types.ModelUsage{Status: types.UsageStatusTimeout})

	require.Nil(t, repo.gotCtxErr)
	require.Equal(t, 1, repo.count())
	got := repo.last()
	require.Equal(t, uint64(8), got.TenantID)
	require.Equal(t, "run-10", *got.EvaluationRunID)
	require.Equal(t, "p", got.Purpose)
	require.Equal(t, types.UsageStatusTimeout, got.Status, "status must stay timeout")
}

func TestRecordPersistenceContextHasTimeout(t *testing.T) {
	repo := &ctxAwareRepo{}
	SetRecorder(NewRecorder(repo))
	defer SetRecorder(nil)

	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	Record(ctx, &types.ModelUsage{Status: types.UsageStatusSuccess})

	require.Nil(t, repo.gotCtxErr, "persistence ctx must be valid")
	require.True(t, repo.gotDeadline, "persistence ctx must carry an independent timeout")
	require.Equal(t, 1, repo.count())
}
