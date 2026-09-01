package service

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type benchmarkResultService struct {
	evaluationRuns interfaces.EvaluationRunRepository
	modelUsage     interfaces.ModelUsageRepository
}

func NewBenchmarkResultService(
	evaluationRuns interfaces.EvaluationRunRepository,
	modelUsage interfaces.ModelUsageRepository,
) interfaces.BenchmarkResultService {
	return &benchmarkResultService{evaluationRuns: evaluationRuns, modelUsage: modelUsage}
}

func (s *benchmarkResultService) GetBenchmarkResult(ctx context.Context, taskID string) (*types.BenchmarkResult, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	run, err := s.evaluationRuns.GetByTaskID(ctx, tenantID, taskID)
	if err != nil {
		return nil, fmt.Errorf("get evaluation run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("task not found")
	}

	modelFacts, err := s.modelUsage.AggregateEvaluationRun(ctx, tenantID, run.ID)
	if err != nil {
		return nil, fmt.Errorf("aggregate benchmark model facts: %w", err)
	}
	quality, err := benchmarkQuality(run)
	if err != nil {
		return nil, err
	}

	return &types.BenchmarkResult{
		BenchmarkVersion: run.ConfigSnapshot.BenchmarkContractVersion,
		Run: types.BenchmarkRunSummary{
			EvaluationRunID: run.ID, TaskID: run.TaskID, Status: run.Status,
			Total: run.Total, Finished: run.Finished, CreatedAt: run.CreatedAt,
			StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, ErrorMessage: run.ErrorMessage,
		},
		Config: run.ConfigSnapshot, Quality: quality,
		RunWallClockDurationMS: run.DurationMS, ModelFacts: modelFacts,
		Reproducibility: benchmarkReproducibility(run.ConfigSnapshot),
	}, nil
}

func benchmarkQuality(run *types.EvaluationRun) (types.BenchmarkQuality, error) {
	switch run.Status {
	case types.EvaluationStatuePending, types.EvaluationStatueRunning:
		return types.BenchmarkQuality{State: types.BenchmarkQualityStatePending}, nil
	case types.EvaluationStatueFailed:
		return types.BenchmarkQuality{State: types.BenchmarkQualityStateUnavailable}, nil
	case types.EvaluationStatueSuccess:
		if evaluationRunMetric(run) == nil {
			return types.BenchmarkQuality{}, fmt.Errorf(
				"benchmark invariant violation: successful run %q does not have all quality metrics", run.TaskID,
			)
		}
		return types.BenchmarkQuality{
			State: types.BenchmarkQualityStateComplete,
			Retrieval: &types.BenchmarkRetrievalQuality{
				Precision: run.Precision, Recall: run.Recall, NDCG3: run.NDCG3,
				NDCG10: run.NDCG10, MRR: run.MRR, MAP: run.MAP,
			},
			Answer: &types.BenchmarkAnswerQuality{
				BLEU1: run.BLEU1, BLEU2: run.BLEU2, BLEU4: run.BLEU4,
				ROUGE1: run.ROUGE1, ROUGE2: run.ROUGE2, ROUGEL: run.ROUGEL,
			},
		}, nil
	default:
		return types.BenchmarkQuality{}, fmt.Errorf("benchmark invariant violation: unknown evaluation status %d", run.Status)
	}
}

func benchmarkReproducibility(snapshot types.EvaluationConfigSnapshotV1) types.BenchmarkReproducibilityState {
	modelsComplete := snapshot.Models.Embedding != nil && snapshot.Models.Chat != nil && snapshot.Models.Summary != nil
	if snapshot.Models.RerankModelID != nil {
		modelsComplete = modelsComplete && snapshot.Models.Rerank != nil
	}
	if snapshot.BenchmarkContractVersion != types.BenchmarkContractVersionV11 ||
		snapshot.Dataset.DatasetSemanticSHA256 == "" || snapshot.Dataset.CorpusCount <= 0 ||
		snapshot.Dataset.QuestionCount <= 0 || snapshot.Dataset.CorpusMode != "pre_chunked_passages" ||
		snapshot.Dataset.ChunkingApplied || snapshot.Execution.WorkerLimit <= 0 ||
		snapshot.Pipeline.Tokenizer.Name != "jieba" || snapshot.Pipeline.Tokenizer.DictionaryMode == "" ||
		len(snapshot.Pipeline.Metrics) == 0 || len(snapshot.Pipeline.NDCGCutoffs) == 0 || !modelsComplete {
		return types.BenchmarkReproducibilityLegacyUnknown
	}
	return types.BenchmarkReproducibilityComplete
}
