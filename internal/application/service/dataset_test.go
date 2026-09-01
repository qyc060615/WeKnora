package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDatasetServiceGetDatasetByID(t *testing.T) {
	service := &DatasetService{datasetRoot: filepath.Join("..", "..", "..", "dataset")}

	t.Run("default", func(t *testing.T) {
		dataset, err := service.GetDatasetByID(context.Background(), "default")
		require.NoError(t, err)
		require.NotEmpty(t, dataset.Corpus)
		require.NotEmpty(t, dataset.QAPairs)
	})

	t.Run("benchmark v1", func(t *testing.T) {
		dataset, err := service.GetDatasetByID(context.Background(), "benchmark_v1")
		require.NoError(t, err)
		require.Len(t, dataset.Corpus, 32)
		require.Len(t, dataset.QAPairs, 15)
		require.Equal(t, 32, dataset.Identity.CorpusCount)
		require.Equal(t, 15, dataset.Identity.QuestionCount)
		require.Equal(t, 15, dataset.Identity.QrelsCount)
		require.Equal(t, 15, dataset.Identity.AnswerCount)
		require.Len(t, dataset.Identity.DatasetSemanticSHA256, 64)
		for index, passage := range dataset.Corpus {
			require.Equal(t, index, passage.PID, "benchmark corpus must preserve PID order")
			require.NotEmpty(t, passage.Text)
		}
		for index, pair := range dataset.QAPairs {
			require.Equal(t, index, pair.QID, "benchmark iteration must be deterministic by QID")
			require.NotEmpty(t, pair.Question)
			require.NotEmpty(t, pair.Answer)
			require.NotEmpty(t, pair.PIDs)
		}

		// Query 0 marks PID 1 as a hard negative in benchmark.json. It must
		// exist in the candidate corpus without becoming qrels ground truth.
		require.NotContains(t, dataset.QAPairs[0].PIDs, 1)
		require.Equal(t, 1, dataset.Corpus[1].PID)
		require.NotEmpty(t, dataset.Corpus[1].Text)
	})

	for _, datasetID := range []string{"not-exist", "../samples", "benchmark_v1/../samples", ""} {
		t.Run("reject "+datasetID, func(t *testing.T) {
			_, err := service.GetDatasetByID(context.Background(), datasetID)
			require.ErrorContains(t, err, "unsupported dataset ID")
		})
	}
}

func TestDatasetSemanticIdentityDeterministicAndContentSensitive(t *testing.T) {
	build := func(reverse bool) dataset {
		result := dataset{
			queries: make(map[int64]string), corpus: make(map[int64]string), answers: make(map[int64]string),
			qrels: make(map[int64][]int64), qas: make(map[int64]int64),
		}
		if reverse {
			result.corpus[2], result.corpus[1] = "two", "one"
			result.queries[2], result.queries[1] = "q2", "q1"
			result.answers[2], result.answers[1] = "a2", "a1"
			result.qrels[2], result.qrels[1] = []int64{2}, []int64{2, 1}
			result.qas[2], result.qas[1] = 2, 1
		} else {
			result.corpus[1], result.corpus[2] = "one", "two"
			result.queries[1], result.queries[2] = "q1", "q2"
			result.answers[1], result.answers[2] = "a1", "a2"
			result.qrels[1], result.qrels[2] = []int64{1, 2}, []int64{2}
			result.qas[1], result.qas[2] = 1, 2
		}
		return result
	}

	firstDataset := build(false)
	first, err := firstDataset.describe("same-id")
	require.NoError(t, err)
	secondDataset := build(true)
	second, err := secondDataset.describe("same-id")
	require.NoError(t, err)
	require.Equal(t, first.Identity.DatasetSemanticSHA256, second.Identity.DatasetSemanticSHA256)

	changedCorpus := build(false)
	changedCorpus.corpus[1] = "changed"
	corpusIdentity, err := changedCorpus.describe("same-id")
	require.NoError(t, err)
	require.NotEqual(t, first.Identity.DatasetSemanticSHA256, corpusIdentity.Identity.DatasetSemanticSHA256)

	changedQrels := build(false)
	changedQrels.qrels[1] = []int64{1}
	qrelsIdentity, err := changedQrels.describe("same-id")
	require.NoError(t, err)
	require.NotEqual(t, first.Identity.DatasetSemanticSHA256, qrelsIdentity.Identity.DatasetSemanticSHA256)
}

func TestDatasetServiceReturnsLoadError(t *testing.T) {
	service := &DatasetService{datasetRoot: t.TempDir()}

	_, err := service.GetDatasetByID(context.Background(), "default")
	require.ErrorContains(t, err, `load dataset "default"`)
	require.ErrorContains(t, err, "load queries")
}
