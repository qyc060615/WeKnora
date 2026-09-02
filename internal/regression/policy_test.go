package regression

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	require.NoError(t, p.Validate())
	require.Equal(t, DefaultAllowedDrop, p.DefaultAllowedDrop)
	require.Equal(t, DefaultAllowedDrop, p.AllowedDropFor(MetricRecall))
}

func TestPolicyPerMetricOverride(t *testing.T) {
	p := DefaultPolicy()
	p.AllowedDrop[MetricRecall] = 0.05
	require.Equal(t, 0.05, p.AllowedDropFor(MetricRecall))
	require.Equal(t, DefaultAllowedDrop, p.AllowedDropFor(MetricPrecision))
}

func TestPolicyValidateRejectsBadThresholds(t *testing.T) {
	negative := DefaultPolicy()
	negative.DefaultAllowedDrop = -0.01
	require.ErrorContains(t, negative.Validate(), "non-negative")

	unknown := DefaultPolicy()
	unknown.AllowedDrop[MetricKey("not_a_metric")] = 0.02
	require.ErrorContains(t, unknown.Validate(), "unknown metric")
}

func TestLoadPolicy(t *testing.T) {
	doc := `{
		"default_allowed_drop": 0.03,
		"allowed_drop": {"recall": 0.05}
	}`
	p, err := LoadPolicy(strings.NewReader(doc))
	require.NoError(t, err)
	require.Equal(t, 0.03, p.DefaultAllowedDrop)
	require.Equal(t, 0.05, p.AllowedDropFor(MetricRecall))
}

func TestLoadPolicyRejectsUnknownMetric(t *testing.T) {
	doc := `{"default_allowed_drop": 0.02, "allowed_drop": {"latency_ms": 1.0}}`
	_, err := LoadPolicy(strings.NewReader(doc))
	require.ErrorContains(t, err, "unknown metric")
}

func TestLoadPolicyRejectsMalformedJSON(t *testing.T) {
	_, err := LoadPolicy(strings.NewReader("{not json"))
	require.ErrorContains(t, err, "parse policy")
}
