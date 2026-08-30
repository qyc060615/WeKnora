package usage

import (
	"context"
	"errors"
	"sync"
	"testing"

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
