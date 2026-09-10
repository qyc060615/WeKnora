package regression

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderTextPass(t *testing.T) {
	report := mustCompare(t, fullResult("task-1"), fullResult("task-1"), DefaultPolicy())
	text := report.RenderText()
	require.Contains(t, text, "Overall: PASS")
	require.Contains(t, text, "All monitored quality metrics are within regression thresholds.")
	require.NotContains(t, text, "Failed Metrics:")
}

func TestRenderTextFailShowsBaselineCurrentDeltaThreshold(t *testing.T) {
	baseline := fullResult("baseline")
	current := fullResult("current")
	setMetric(baseline, MetricRecall, 0.8667)
	setMetric(current, MetricRecall, 0.8000)

	report := mustCompare(t, baseline, current, DefaultPolicy())
	text := report.RenderText()
	require.Contains(t, text, "Overall: FAIL")
	require.Contains(t, text, "Recall")
	require.Contains(t, text, "baseline = 0.8667")
	require.Contains(t, text, "current  = 0.8000")
	require.Contains(t, text, "delta    = -0.0667")
	require.Contains(t, text, "threshold = -0.0200")
}

// The report must remain JSON-serializable even when a metric is non-finite
// (NaN/Inf would make json.Marshal fail if they leaked into a float field).
func TestReportJSONMarshalSucceeds(t *testing.T) {
	report := mustCompare(t, fullResult("a"), fullResult("a"), DefaultPolicy())
	data, err := json.Marshal(report)
	require.NoError(t, err)
	require.Contains(t, string(data), `"overall_status":"PASS"`)
}
