package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookMetricRetrievalRanking(t *testing.T) {
	tests := []struct {
		name          string
		relevantPIDs  []int
		retrievedPIDs []int
		precision     float64
		recall        float64
		mrr           float64
	}{
		{
			name:          "irrelevant result before hit preserves rank",
			relevantPIDs:  []int{10},
			retrievedPIDs: []int{20, 10, 30},
			precision:     1.0 / 3.0,
			recall:        1,
			mrr:           1.0 / 2.0,
		},
		{
			name:          "first result hits",
			relevantPIDs:  []int{10},
			retrievedPIDs: []int{10, 20, 30},
			precision:     1.0 / 3.0,
			recall:        1,
			mrr:           1,
		},
		{
			name:          "no result hits",
			relevantPIDs:  []int{10},
			retrievedPIDs: []int{20, 30, 40},
			precision:     0,
			recall:        0,
			mrr:           0,
		},
		{
			name:          "multiple relevant passages",
			relevantPIDs:  []int{10, 30},
			retrievedPIDs: []int{20, 10, 30, 40},
			precision:     0.5,
			recall:        1,
			mrr:           0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := NewHookMetric(1)
			hook.recordInit(0)
			hook.recordQaPair(0, &types.QAPair{QID: 1, PIDs: tt.relevantPIDs})
			hook.recordChatResponse(0, &types.ChatResponse{Content: "valid answer"})

			results := make([]*types.SearchResult, len(tt.retrievedPIDs))
			for i, pid := range tt.retrievedPIDs {
				results[i] = &types.SearchResult{ChunkIndex: pid}
			}
			hook.recordRerankResult(0, results)
			require.NoError(t, hook.recordFinish(0))

			metrics := hook.MetricResult().RetrievalMetrics
			assert.InDelta(t, tt.precision, metrics.Precision, 1e-12)
			assert.InDelta(t, tt.recall, metrics.Recall, 1e-12)
			assert.InDelta(t, tt.mrr, metrics.MRR, 1e-12)
		})
	}
}

func TestHookMetricRejectsMissingGeneratedResponse(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *types.ChatResponse
		want     string
	}{
		{name: "nil", response: nil, want: "no generated response"},
		{name: "empty", response: &types.ChatResponse{}, want: "empty generated response"},
		{name: "whitespace", response: &types.ChatResponse{Content: " \n\t"}, want: "empty generated response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hook := NewHookMetric(1)
			hook.recordInit(0)
			hook.recordQaPair(0, &types.QAPair{QID: 9, Answer: "expected"})
			hook.recordChatResponse(0, tc.response)
			err := hook.recordFinish(0)
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, hook.metricResults.results)
		})
	}
}

func TestHookMetricAcceptsGeneratedResponse(t *testing.T) {
	hook := NewHookMetric(1)
	hook.recordInit(0)
	hook.recordQaPair(0, &types.QAPair{QID: 9, Answer: "expected"})
	hook.recordChatResponse(0, &types.ChatResponse{Content: "generated"})
	require.NoError(t, hook.recordFinish(0))
	require.Len(t, hook.metricResults.results, 1)
}
