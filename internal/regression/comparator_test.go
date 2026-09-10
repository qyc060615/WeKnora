package regression

import (
	"math"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// fullResult returns a Benchmark v1.1 result with all twelve quality metrics
// populated. It is the deterministic fixture basis for comparator tests.
func fullResult(taskID string) *types.BenchmarkResult {
	precision, recall, ndcg3, ndcg10 := 0.90, 0.85, 0.88, 0.91
	mrr, mapv := 0.86, 0.83
	bleu1, bleu2, bleu4 := 0.35, 0.30, 0.25
	rouge1, rouge2, rougel := 0.45, 0.38, 0.42
	return &types.BenchmarkResult{
		BenchmarkVersion: types.BenchmarkContractVersionV11,
		Run:              types.BenchmarkRunSummary{TaskID: taskID, EvaluationRunID: "run-" + taskID},
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
}

func setMetric(r *types.BenchmarkResult, key MetricKey, v float64) {
	switch key {
	case MetricPrecision:
		r.Quality.Retrieval.Precision = &v
	case MetricRecall:
		r.Quality.Retrieval.Recall = &v
	case MetricNDCG3:
		r.Quality.Retrieval.NDCG3 = &v
	case MetricNDCG10:
		r.Quality.Retrieval.NDCG10 = &v
	case MetricMRR:
		r.Quality.Retrieval.MRR = &v
	case MetricMAP:
		r.Quality.Retrieval.MAP = &v
	case MetricBLEU1:
		r.Quality.Answer.BLEU1 = &v
	case MetricBLEU2:
		r.Quality.Answer.BLEU2 = &v
	case MetricBLEU4:
		r.Quality.Answer.BLEU4 = &v
	case MetricROUGE1:
		r.Quality.Answer.ROUGE1 = &v
	case MetricROUGE2:
		r.Quality.Answer.ROUGE2 = &v
	case MetricROUGEL:
		r.Quality.Answer.ROUGEL = &v
	}
}

func nilMetric(r *types.BenchmarkResult, key MetricKey) {
	switch key {
	case MetricPrecision:
		r.Quality.Retrieval.Precision = nil
	case MetricRecall:
		r.Quality.Retrieval.Recall = nil
	case MetricNDCG3:
		r.Quality.Retrieval.NDCG3 = nil
	case MetricNDCG10:
		r.Quality.Retrieval.NDCG10 = nil
	case MetricMRR:
		r.Quality.Retrieval.MRR = nil
	case MetricMAP:
		r.Quality.Retrieval.MAP = nil
	case MetricBLEU1:
		r.Quality.Answer.BLEU1 = nil
	case MetricBLEU2:
		r.Quality.Answer.BLEU2 = nil
	case MetricBLEU4:
		r.Quality.Answer.BLEU4 = nil
	case MetricROUGE1:
		r.Quality.Answer.ROUGE1 = nil
	case MetricROUGE2:
		r.Quality.Answer.ROUGE2 = nil
	case MetricROUGEL:
		r.Quality.Answer.ROUGEL = nil
	}
}

func mustCompare(t *testing.T, baseline, current *types.BenchmarkResult, policy Policy) *Report {
	t.Helper()
	report, err := Compare(baseline, current, policy)
	require.NoError(t, err)
	return report
}

// Test 1: baseline == current -> PASS.
func TestCompareEqualResultPass(t *testing.T) {
	report := mustCompare(t, fullResult("task-1"), fullResult("task-1"), DefaultPolicy())
	require.Equal(t, StatusPass, report.OverallStatus)
	require.Len(t, report.Comparisons, 12)
	require.Empty(t, report.FailedMetrics)
	require.Empty(t, report.MissingMetrics)
}

// Test 2: an improved metric -> PASS.
func TestCompareImprovedResultPass(t *testing.T) {
	baseline := fullResult("baseline")
	current := fullResult("current")
	setMetric(current, MetricRecall, 0.90) // better than baseline 0.85
	report := mustCompare(t, baseline, current, DefaultPolicy())
	require.Equal(t, StatusPass, report.OverallStatus)
}

// Test 3: regression within threshold -> PASS.
func TestCompareRegressionWithinThresholdPass(t *testing.T) {
	baseline := fullResult("baseline")
	current := fullResult("current")
	setMetric(baseline, MetricRecall, 0.80)
	setMetric(current, MetricRecall, 0.79) // delta = -0.01 >= -0.02
	report := mustCompare(t, baseline, current, DefaultPolicy())
	require.Equal(t, StatusPass, report.OverallStatus)
}

// Test 4: regression beyond threshold -> FAIL, with full per-metric detail.
func TestCompareRegressionBeyondThresholdFail(t *testing.T) {
	baseline := fullResult("baseline")
	current := fullResult("current")
	setMetric(baseline, MetricRecall, 0.80)
	setMetric(current, MetricRecall, 0.77) // delta = -0.03 < -0.02
	report := mustCompare(t, baseline, current, DefaultPolicy())

	require.Equal(t, StatusFail, report.OverallStatus)
	require.Equal(t, []string{string(MetricRecall)}, report.FailedMetrics)

	var mc MetricComparison
	for _, c := range report.Comparisons {
		if c.Key == MetricRecall {
			mc = c
		}
	}
	require.Equal(t, StatusFail, mc.Status)
	require.NotNil(t, mc.Baseline)
	require.NotNil(t, mc.Current)
	require.NotNil(t, mc.Delta)
	require.InDelta(t, 0.80, *mc.Baseline, 1e-9)
	require.InDelta(t, 0.77, *mc.Current, 1e-9)
	require.InDelta(t, -0.03, *mc.Delta, 1e-9)
	require.InDelta(t, 0.02, mc.AllowedDrop, 1e-9)
}

// Test 5: multiple regressions -> every failing metric is reported, no early
// exit after the first.
func TestCompareMultipleRegressionsAllReported(t *testing.T) {
	baseline := fullResult("baseline")
	current := fullResult("current")
	setMetric(current, MetricRecall, 0.70) // -0.15
	setMetric(current, MetricNDCG10, 0.88) // -0.03
	setMetric(current, MetricMAP, 0.80)    // -0.03
	report := mustCompare(t, baseline, current, DefaultPolicy())

	require.Equal(t, StatusFail, report.OverallStatus)
	require.ElementsMatch(t,
		[]string{string(MetricRecall), string(MetricNDCG10), string(MetricMAP)},
		report.FailedMetrics,
	)
	text := report.RenderText()
	require.Contains(t, text, "Recall")
	require.Contains(t, text, "NDCG@10")
	require.Contains(t, text, "MAP")
}

// Test 6: a missing metric in either side -> FAIL, never silently skipped.
func TestCompareMissingMetricFails(t *testing.T) {
	baseline := fullResult("baseline")
	current := fullResult("current")
	nilMetric(current, MetricROUGEL)

	report := mustCompare(t, baseline, current, DefaultPolicy())
	require.Equal(t, StatusFail, report.OverallStatus)
	require.Equal(t, []string{string(MetricROUGEL)}, report.MissingMetrics)
	require.Contains(t, report.Summary, "missing")
}

func TestCompareNilInputsError(t *testing.T) {
	_, err := Compare(nil, fullResult("current"), DefaultPolicy())
	require.ErrorContains(t, err, "baseline result is nil")
	_, err = Compare(fullResult("baseline"), nil, DefaultPolicy())
	require.ErrorContains(t, err, "current result is nil")
}

// Non-finite values must fail closed rather than produce a bogus comparison
// (and must not leak NaN/Inf into the JSON-serializable report).
func TestCompareNonFiniteFailsClosed(t *testing.T) {
	baseline := fullResult("baseline")
	current := fullResult("current")
	nan := math.NaN()
	current.Quality.Retrieval.MRR = &nan

	report := mustCompare(t, baseline, current, DefaultPolicy())
	require.Equal(t, StatusFail, report.OverallStatus)
	for _, c := range report.Comparisons {
		if c.Key == MetricMRR {
			require.True(t, c.NonFinite)
			require.Nil(t, c.Baseline)
			require.Nil(t, c.Delta)
		}
	}
}

func TestMetricsRegistryIsTwelveHigherIsBetter(t *testing.T) {
	require.Len(t, Metrics(), 12)
	for _, spec := range Metrics() {
		require.True(t, spec.HigherIsBetter, "metric %s must be higher-is-better", spec.Key)
		require.NotEmpty(t, spec.DisplayName)
	}
}
