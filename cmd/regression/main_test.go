package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func resultJSON(mutate func(*types.BenchmarkResult)) []byte {
	precision, recall, ndcg3, ndcg10 := 0.90, 0.85, 0.88, 0.91
	mrr, mapv := 0.86, 0.83
	bleu1, bleu2, bleu4 := 0.35, 0.30, 0.25
	rouge1, rouge2, rougel := 0.45, 0.38, 0.42
	r := &types.BenchmarkResult{
		BenchmarkVersion: types.BenchmarkContractVersionV11,
		Run:              types.BenchmarkRunSummary{TaskID: "task-fixture", EvaluationRunID: "run-fixture"},
		Quality: types.BenchmarkQuality{
			State: types.BenchmarkQualityStateComplete,
			Retrieval: &types.BenchmarkRetrievalQuality{
				Precision: &precision, Recall: &recall, NDCG3: &ndcg3,
				NDCG10: &ndcg10, MRR: &mrr, MAP: &mapv,
			},
			Answer: &types.BenchmarkAnswerQuality{
				BLEU1: &bleu1, BLEU2: &bleu2, BLEU4: &bleu4,
				ROUGE1: &rouge1, ROUGE2: &rouge2, ROUGEL: &rougel,
			},
		},
		Reproducibility: types.BenchmarkReproducibilityComplete,
	}
	if mutate != nil {
		mutate(r)
	}
	data, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return data
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

func TestCLIPassExitZero(t *testing.T) {
	dir := t.TempDir()
	baseline := writeFile(t, dir, "baseline.json", resultJSON(nil))
	current := writeFile(t, dir, "current.json", resultJSON(nil))

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--baseline", baseline, "--current", current}, &stdout, &stderr)
	require.Equal(t, exitPass, code)
	require.Contains(t, stdout.String(), "Overall: PASS")
}

func TestCLIRegressionExitOne(t *testing.T) {
	dir := t.TempDir()
	baseline := writeFile(t, dir, "baseline.json", resultJSON(nil))
	current := writeFile(t, dir, "current.json", resultJSON(func(r *types.BenchmarkResult) {
		v := 0.70
		r.Quality.Retrieval.Recall = &v
	}))

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--baseline", baseline, "--current", current}, &stdout, &stderr)
	require.Equal(t, exitRegression, code)
	require.Contains(t, stdout.String(), "Overall: FAIL")
	require.Contains(t, stdout.String(), "Recall")
}

// Test 7: an unparseable baseline is an execution error with a distinct exit
// code and a clear message.
func TestCLIInvalidBaselineExitTwo(t *testing.T) {
	dir := t.TempDir()
	baseline := writeFile(t, dir, "baseline.json", []byte("{not valid json"))
	current := writeFile(t, dir, "current.json", resultJSON(nil))

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--baseline", baseline, "--current", current}, &stdout, &stderr)
	require.Equal(t, exitError, code)
	require.Contains(t, stderr.String(), "load baseline")
}

func TestCLIMissingRequiredFlagsExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCLI(nil, &stdout, &stderr)
	require.Equal(t, exitError, code)
	require.Contains(t, stderr.String(), "--baseline and --current are required")
}

func TestCLIReportFileWrittenOnFail(t *testing.T) {
	dir := t.TempDir()
	baseline := writeFile(t, dir, "baseline.json", resultJSON(nil))
	current := writeFile(t, dir, "current.json", resultJSON(func(r *types.BenchmarkResult) {
		v := 0.70
		r.Quality.Retrieval.Recall = &v
	}))
	reportPath := filepath.Join(dir, "report.json")

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"--baseline", baseline, "--current", current, "--report", reportPath,
	}, &stdout, &stderr)
	require.Equal(t, exitRegression, code)

	data, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	require.Contains(t, string(data), `"overall_status": "FAIL"`)
}

func TestCLIPolicyFileOverride(t *testing.T) {
	dir := t.TempDir()
	baseline := writeFile(t, dir, "baseline.json", resultJSON(nil))
	// Recall drops by 0.15; a generous 0.20 policy must still pass.
	current := writeFile(t, dir, "current.json", resultJSON(func(r *types.BenchmarkResult) {
		v := 0.70
		r.Quality.Retrieval.Recall = &v
	}))
	policyPath := writeFile(t, dir, "policy.json", []byte(
		`{"default_allowed_drop": 0.20, "allowed_drop": {}}`,
	))

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"--baseline", baseline, "--current", current, "--policy", policyPath,
	}, &stdout, &stderr)
	require.Equal(t, exitPass, code)
	require.Contains(t, stdout.String(), "Overall: PASS")
}

func TestCLIEmptyResultFailsClosed(t *testing.T) {
	dir := t.TempDir()
	baseline := writeFile(t, dir, "baseline.json", []byte(`{}`))
	current := writeFile(t, dir, "current.json", resultJSON(nil))

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--baseline", baseline, "--current", current}, &stdout, &stderr)
	require.Equal(t, exitRegression, code)
	require.Contains(t, stdout.String(), "Overall: FAIL")
	require.True(t, strings.Contains(stdout.String(), "missing"), "empty result must fail closed as missing metrics")
}
