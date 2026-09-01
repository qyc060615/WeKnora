package service

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type benchmarkEvaluationRepositoryStub struct {
	tenantID uint64
	taskID   string
	run      *types.EvaluationRun
}

func (s *benchmarkEvaluationRepositoryStub) Create(context.Context, *types.EvaluationRun) error {
	return nil
}
func (s *benchmarkEvaluationRepositoryStub) GetByTaskID(_ context.Context, tenantID uint64, taskID string) (*types.EvaluationRun, error) {
	if tenantID != s.tenantID || taskID != s.taskID {
		return nil, nil
	}
	return s.run, nil
}
func (s *benchmarkEvaluationRepositoryStub) MarkRunning(context.Context, uint64, string, time.Time) error {
	return nil
}
func (s *benchmarkEvaluationRepositoryStub) UpdateTotal(context.Context, uint64, string, int) error {
	return nil
}
func (s *benchmarkEvaluationRepositoryStub) IncrementFinished(context.Context, uint64, string) error {
	return nil
}
func (s *benchmarkEvaluationRepositoryStub) MarkSuccess(context.Context, uint64, string, *types.MetricResult, time.Time) error {
	return nil
}
func (s *benchmarkEvaluationRepositoryStub) MarkFailed(context.Context, uint64, string, *types.MetricResult, string, time.Time) error {
	return nil
}

type benchmarkUsageRepositoryStub struct {
	tenantID uint64
	runID    string
	result   *types.EvaluationModelUsageAggregate
}

func (s *benchmarkUsageRepositoryStub) Create(context.Context, *types.ModelUsage) error { return nil }
func (s *benchmarkUsageRepositoryStub) GetByID(context.Context, uint64, string) (*types.ModelUsage, error) {
	return nil, nil
}
func (s *benchmarkUsageRepositoryStub) AggregateEvaluationRun(_ context.Context, tenantID uint64, runID string) (*types.EvaluationModelUsageAggregate, error) {
	s.tenantID, s.runID = tenantID, runID
	return s.result, nil
}

func completeBenchmarkSnapshot() types.EvaluationConfigSnapshotV1 {
	model := func(id string) *types.EvaluationConfiguredModelSnapshot {
		return &types.EvaluationConfiguredModelSnapshot{ID: id, Name: id, Type: "test", Source: "test"}
	}
	return types.EvaluationConfigSnapshotV1{
		SnapshotSchemaVersion: 1, BenchmarkContractVersion: types.BenchmarkContractVersionV11,
		Dataset: types.EvaluationDatasetSnapshot{
			DatasetID: "benchmark_v1", DatasetSemanticSHA256: "dataset-hash",
			CorpusCount: 32, QuestionCount: 15, QrelsCount: 15, AnswerCount: 15,
			CorpusMode: "pre_chunked_passages", ChunkingApplied: false,
		},
		Pipeline: types.EvaluationPipelineSnapshot{
			Name: "rag", Metrics: []string{"recall"}, NDCGCutoffs: []int{3, 10},
			Tokenizer: types.EvaluationTokenizerSnapshot{Name: "jieba", DictionaryMode: "builtin"},
		},
		Models: types.EvaluationModelsSnapshot{
			EmbeddingModelID: "embedding", ChatModelID: "chat", SummaryModelID: "summary",
			Embedding: model("embedding"), Chat: model("chat"), Summary: model("summary"),
		},
		Execution: types.EvaluationExecutionSnapshot{WorkerLimit: 2},
	}
}

func benchmarkRun(status types.EvaluationStatue) *types.EvaluationRun {
	now := time.Now().UTC()
	duration := int64(1234)
	run := &types.EvaluationRun{
		ID: "run-1", TaskID: "task-1", TenantID: 7, DatasetID: "benchmark_v1",
		Status: status, Total: 15, Finished: 15, ConfigSnapshot: completeBenchmarkSnapshot(),
		CreatedAt: now, StartedAt: &now, FinishedAt: &now, DurationMS: &duration,
	}
	if status == types.EvaluationStatueSuccess {
		value := 0.5
		run.Precision, run.Recall, run.NDCG3, run.NDCG10, run.MRR, run.MAP =
			&value, &value, &value, &value, &value, &value
		run.BLEU1, run.BLEU2, run.BLEU4, run.ROUGE1, run.ROUGE2, run.ROUGEL =
			&value, &value, &value, &value, &value, &value
	}
	return run
}

func TestBenchmarkResultServiceQualityAndOperationalFacts(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      types.EvaluationStatue
		wantState   types.BenchmarkQualityState
		wantQuality bool
	}{
		{name: "pending", status: types.EvaluationStatuePending, wantState: types.BenchmarkQualityStatePending},
		{name: "running", status: types.EvaluationStatueRunning, wantState: types.BenchmarkQualityStatePending},
		{name: "success", status: types.EvaluationStatueSuccess, wantState: types.BenchmarkQualityStateComplete, wantQuality: true},
		{name: "failed", status: types.EvaluationStatueFailed, wantState: types.BenchmarkQualityStateUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := benchmarkRun(tc.status)
			if tc.status != types.EvaluationStatueSuccess {
				run.Finished = 4
			}
			facts := &types.EvaluationModelUsageAggregate{
				Calls:                   types.CallCounts{Total: 2, Chat: 1, Embedding: 1},
				ObservedModels:          []types.ObservedModelIdentity{{CallType: types.CallTypeChat, Calls: 1}},
				CostByCurrency:          []types.CurrencyCostAggregate{{Currency: "CNY"}, {Currency: "USD"}},
				CostRowsWithoutCurrency: types.CallCounts{Total: 1, Chat: 1},
			}
			evaluationRepo := &benchmarkEvaluationRepositoryStub{tenantID: 7, taskID: "task-1", run: run}
			usageRepo := &benchmarkUsageRepositoryStub{result: facts}
			svc := NewBenchmarkResultService(evaluationRepo, usageRepo)
			ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))

			got, err := svc.GetBenchmarkResult(ctx, "task-1")
			require.NoError(t, err)
			require.Equal(t, tc.wantState, got.Quality.State)
			require.Equal(t, tc.wantQuality, got.Quality.Retrieval != nil)
			require.Equal(t, tc.wantQuality, got.Quality.Answer != nil)
			require.Same(t, facts, got.ModelFacts)
			require.Equal(t, int64(2), got.ModelFacts.Calls.Total)
			require.Len(t, got.ModelFacts.CostByCurrency, 2)
			require.Equal(t, types.CallCounts{Total: 1, Chat: 1}, got.ModelFacts.CostRowsWithoutCurrency)
			require.Equal(t, run.ID, usageRepo.runID)
			require.Equal(t, uint64(7), usageRepo.tenantID)
			require.Equal(t, types.BenchmarkReproducibilityComplete, got.Reproducibility)
			require.Equal(t, int64(1234), *got.RunWallClockDurationMS)
		})
	}
}

func TestBenchmarkResultServiceSuccessfulRunRequiresAllMetrics(t *testing.T) {
	run := benchmarkRun(types.EvaluationStatueSuccess)
	run.ROUGEL = nil
	svc := NewBenchmarkResultService(
		&benchmarkEvaluationRepositoryStub{tenantID: 7, taskID: "task-1", run: run},
		&benchmarkUsageRepositoryStub{result: &types.EvaluationModelUsageAggregate{}},
	)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	_, err := svc.GetBenchmarkResult(ctx, "task-1")
	require.ErrorContains(t, err, "does not have all quality metrics")
}

func TestBenchmarkResultServiceLegacyAndTenantIsolation(t *testing.T) {
	run := benchmarkRun(types.EvaluationStatueFailed)
	run.ConfigSnapshot = types.EvaluationConfigSnapshotV1{
		SnapshotSchemaVersion: 1,
		Dataset:               types.EvaluationDatasetSnapshot{DatasetID: "benchmark_v1"},
	}
	repo := &benchmarkEvaluationRepositoryStub{tenantID: 7, taskID: "task-1", run: run}
	svc := NewBenchmarkResultService(repo, &benchmarkUsageRepositoryStub{result: &types.EvaluationModelUsageAggregate{}})

	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	got, err := svc.GetBenchmarkResult(ctx, "task-1")
	require.NoError(t, err)
	require.Equal(t, types.BenchmarkReproducibilityLegacyUnknown, got.Reproducibility)
	require.Empty(t, got.BenchmarkVersion)

	wrongTenant := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(8))
	_, err = svc.GetBenchmarkResult(wrongTenant, "task-1")
	require.ErrorContains(t, err, "task not found")
	_, err = svc.GetBenchmarkResult(ctx, "wrong-task")
	require.ErrorContains(t, err, "task not found")
}
