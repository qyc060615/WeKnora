package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type benchmarkManifest struct {
	Version         string                     `json:"version"`
	SourceDirectory string                     `json:"source_directory"`
	Passages        []benchmarkManifestPassage `json:"passages"`
	Queries         []benchmarkManifestQuery   `json:"queries"`
}

type benchmarkManifestPassage struct {
	PID     int64  `json:"pid"`
	Source  string `json:"source"`
	Section string `json:"section"`
	Text    string `json:"text"`
}

type benchmarkManifestQuery struct {
	QID              int64   `json:"qid"`
	Type             string  `json:"type"`
	Question         string  `json:"question"`
	AID              int64   `json:"aid"`
	Answer           string  `json:"answer"`
	RelevantPIDs     []int64 `json:"relevant_pids"`
	HardNegativePIDs []int64 `json:"hard_negative_pids"`
	DesignReason     string  `json:"design_reason"`
}

func TestBenchmarkV1Integrity(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..", "..")
	directory := filepath.Join(repositoryRoot, "dataset", "benchmark_v1")
	manifestData, err := os.ReadFile(filepath.Join(directory, "benchmark.json"))
	require.NoError(t, err)

	var manifest benchmarkManifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	require.Equal(t, "benchmark_v1", manifest.Version)
	require.GreaterOrEqual(t, len(manifest.Queries), 12)
	require.LessOrEqual(t, len(manifest.Queries), 15)
	require.NotEmpty(t, manifest.Passages)

	corpus, err := loadParquet[TextInfo](filepath.Join(directory, "corpus.parquet"))
	require.NoError(t, err)
	queries, err := loadParquet[TextInfo](filepath.Join(directory, "queries.parquet"))
	require.NoError(t, err)
	answers, err := loadParquet[TextInfo](filepath.Join(directory, "answers.parquet"))
	require.NoError(t, err)
	qrels, err := loadParquet[RelsInfo](filepath.Join(directory, "qrels.parquet"))
	require.NoError(t, err)
	qas, err := loadParquet[QaInfo](filepath.Join(directory, "qas.parquet"))
	require.NoError(t, err)

	require.Len(t, queries, len(manifest.Queries))
	require.Len(t, answers, len(manifest.Queries))
	require.Len(t, qas, len(manifest.Queries))
	require.Len(t, corpus, len(manifest.Passages))

	corpusByID := uniqueTextRows(t, corpus, "corpus")
	queriesByID := uniqueTextRows(t, queries, "queries")
	answersByID := uniqueTextRows(t, answers, "answers")
	qasByQID := make(map[int64]int64, len(qas))
	for _, row := range qas {
		_, duplicate := qasByQID[row.QID]
		require.False(t, duplicate, "duplicate qas QID %d", row.QID)
		_, queryExists := queriesByID[row.QID]
		_, answerExists := answersByID[row.AID]
		require.True(t, queryExists, "qas QID %d does not exist", row.QID)
		require.True(t, answerExists, "qas AID %d does not exist", row.AID)
		qasByQID[row.QID] = row.AID
	}
	qrelsByQID := make(map[int64][]int64)
	qrelPairs := make(map[[2]int64]struct{}, len(qrels))
	for _, row := range qrels {
		_, queryExists := queriesByID[row.QID]
		_, passageExists := corpusByID[row.PID]
		require.True(t, queryExists, "qrels QID %d does not exist", row.QID)
		require.True(t, passageExists, "qrels PID %d does not exist", row.PID)
		pair := [2]int64{row.QID, row.PID}
		_, duplicate := qrelPairs[pair]
		require.False(t, duplicate, "duplicate qrels pair QID=%d PID=%d", row.QID, row.PID)
		qrelPairs[pair] = struct{}{}
		qrelsByQID[row.QID] = append(qrelsByQID[row.QID], row.PID)
	}

	typeCounts := make(map[string]int)
	manifestPIDs := make(map[int64]struct{}, len(manifest.Passages))
	sourceFiles := make(map[string]struct{})
	for index, passage := range manifest.Passages {
		require.Equal(t, int64(index), passage.PID, "benchmark PIDs must be contiguous")
		_, duplicate := manifestPIDs[passage.PID]
		require.False(t, duplicate, "duplicate manifest PID %d", passage.PID)
		manifestPIDs[passage.PID] = struct{}{}
		require.NotEmpty(t, passage.Source)
		require.NotEmpty(t, passage.Section)
		require.NotEmpty(t, passage.Text)
		require.Equal(t, passage.Text, corpusByID[passage.PID])
		require.FileExists(t, filepath.Join(repositoryRoot, manifest.SourceDirectory, passage.Source))
		sourceFiles[passage.Source] = struct{}{}
	}
	require.Len(t, sourceFiles, 10)

	manifestQIDs := make(map[int64]struct{}, len(manifest.Queries))
	for index, query := range manifest.Queries {
		require.Equal(t, int64(index), query.QID, "benchmark QIDs must be contiguous")
		_, duplicate := manifestQIDs[query.QID]
		require.False(t, duplicate, "duplicate manifest QID %d", query.QID)
		manifestQIDs[query.QID] = struct{}{}
		typeCounts[query.Type]++

		require.NotEmpty(t, query.Question)
		require.NotEmpty(t, query.Answer)
		require.NotEmpty(t, query.RelevantPIDs)
		require.NotEmpty(t, query.DesignReason)
		require.Equal(t, query.Question, queriesByID[query.QID])
		require.Equal(t, query.Answer, answersByID[query.AID])
		require.Equal(t, query.AID, qasByQID[query.QID])
		require.ElementsMatch(t, query.RelevantPIDs, qrelsByQID[query.QID])

		relevant := make(map[int64]struct{}, len(query.RelevantPIDs))
		for _, pid := range query.RelevantPIDs {
			_, exists := corpusByID[pid]
			require.True(t, exists, "query %d relevant PID %d does not exist", query.QID, pid)
			relevant[pid] = struct{}{}
		}
		for _, pid := range query.HardNegativePIDs {
			_, exists := corpusByID[pid]
			require.True(t, exists, "query %d hard-negative PID %d does not exist", query.QID, pid)
			_, isRelevant := relevant[pid]
			require.False(t, isRelevant, "query %d PID %d cannot be both relevant and negative", query.QID, pid)
		}
		if query.Type == "hard-negative" {
			require.NotEmpty(t, query.HardNegativePIDs)
		}
	}

	require.Equal(t, map[string]int{"single-hop": 8, "hard-negative": 4, "boundary": 3}, typeCounts)
	t.Logf("benchmark_v1 stats: queries=%d corpus=%d answers=%d qrels=%d avg_relevant=%.2f types=%v",
		len(queries), len(corpus), len(answers), len(qrels), float64(len(qrels))/float64(len(queries)), typeCounts)
}

func uniqueTextRows(t *testing.T, rows []TextInfo, name string) map[int64]string {
	t.Helper()
	result := make(map[int64]string, len(rows))
	for _, row := range rows {
		_, duplicate := result[row.ID]
		require.False(t, duplicate, "duplicate %s ID %d", name, row.ID)
		require.NotEmpty(t, row.Text, "%s ID %d has empty text", name, row.ID)
		result[row.ID] = row.Text
	}
	return result
}
