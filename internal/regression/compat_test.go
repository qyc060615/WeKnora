package regression

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// contractResult returns a BenchmarkResult whose config snapshot carries the
// frozen Benchmark v1.1 contract (mirroring baseline_cache_off_v1_1.json).
func contractResult() *types.BenchmarkResult {
	return &types.BenchmarkResult{
		BenchmarkVersion: "v1.1",
		Config: types.EvaluationConfigSnapshotV1{
			BenchmarkContractVersion: "v1.1",
			Dataset: types.EvaluationDatasetSnapshot{
				DatasetID:             "benchmark_v1",
				DatasetSemanticSHA256: "56fd363d797ee4c1524a5a1a2517b3b30ce955229c37784cf730c0d1dc47fd0d",
				CorpusCount:           32,
				QuestionCount:         15,
			},
			Retrieval: types.EvaluationRetrievalSnapshot{
				VectorThreshold:  0.2,
				KeywordThreshold: 0.3,
				EmbeddingTopK:    30,
				RerankTopK:       30,
				RerankThreshold:  0.3,
				RetrieveDriver:   "postgres",
			},
			Models: types.EvaluationModelsSnapshot{
				Embedding: &types.EvaluationConfiguredModelSnapshot{
					Name:      "text-embedding-v4",
					Provider:  "generic",
					Embedding: &types.EvaluationEmbeddingSnapshot{Dimension: 1024},
				},
				Chat:   &types.EvaluationConfiguredModelSnapshot{Name: "deepseek-v4-pro", Provider: "generic"},
				Rerank: &types.EvaluationConfiguredModelSnapshot{Name: "qwen3-rerank", Provider: "aliyun"},
			},
			Execution: types.EvaluationExecutionSnapshot{WorkerLimit: 27},
		},
		Quality: types.BenchmarkQuality{
			State:     types.BenchmarkQualityStateComplete,
			Retrieval: &types.BenchmarkRetrievalQuality{},
			Answer:    &types.BenchmarkAnswerQuality{},
		},
	}
}

func TestCheckCompatibilityPass(t *testing.T) {
	require.Empty(t, CheckCompatibility(contractResult(), contractResult()))
}

func TestCheckCompatibilityModelMismatch(t *testing.T) {
	current := contractResult()
	current.Config.Models.Rerank.Provider = "generic" // baseline is aliyun

	mismatches := CheckCompatibility(contractResult(), current)
	require.Len(t, mismatches, 1)
	require.Equal(t, "models.rerank.provider", mismatches[0].Field)
	require.Equal(t, "aliyun", mismatches[0].Baseline)
	require.Equal(t, "generic", mismatches[0].Current)
}

func TestCheckCompatibilityDatasetMismatch(t *testing.T) {
	current := contractResult()
	current.Config.Dataset.DatasetSemanticSHA256 = "1111111111111111111111111111111111111111111111111111111111111111"

	mismatches := CheckCompatibility(contractResult(), current)
	require.Len(t, mismatches, 1)
	require.Equal(t, "dataset.dataset_semantic_sha256", mismatches[0].Field)
}

func TestCheckCompatibilityMultipleMismatch(t *testing.T) {
	current := contractResult()
	current.Config.Models.Rerank.Provider = "generic"
	current.Config.Models.Chat.Name = "some-other-chat"
	current.Config.Retrieval.EmbeddingTopK = 10
	current.Config.Execution.WorkerLimit = 8

	mismatches := CheckCompatibility(contractResult(), current)

	fields := make(map[string]bool)
	for _, m := range mismatches {
		fields[m.Field] = true
	}
	require.True(t, fields["models.rerank.provider"], "rerank provider mismatch must be reported")
	require.True(t, fields["models.chat.name"], "chat name mismatch must be reported")
	require.True(t, fields["retrieval.embedding_top_k"], "embedding_top_k mismatch must be reported")
	require.True(t, fields["execution.worker_limit"], "worker_limit mismatch must be reported")
	require.Len(t, mismatches, 4, "all four mismatches must be reported, not just the first")
}

func TestCheckCompatibilityNilInputs(t *testing.T) {
	require.Empty(t, CheckCompatibility(nil, contractResult()))
	require.Empty(t, CheckCompatibility(contractResult(), nil))
}

func TestFormatMismatches(t *testing.T) {
	mismatches := []Mismatch{
		{Field: "models.rerank.provider", Baseline: "aliyun", Current: "generic"},
		{Field: "dataset.dataset_semantic_sha256", Baseline: "aaa", Current: "bbb"},
	}
	text := FormatMismatches(mismatches)
	require.Contains(t, text, "Compatibility: FAIL")
	require.Contains(t, text, "models.rerank.provider")
	require.Contains(t, text, "baseline = aliyun")
	require.Contains(t, text, "current  = generic")
	require.Contains(t, text, "dataset.dataset_semantic_sha256")
}
