package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
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
			hook.recordQaPair(0, &types.QAPair{PIDs: tt.relevantPIDs})

			results := make([]*types.SearchResult, len(tt.retrievedPIDs))
			for i, pid := range tt.retrievedPIDs {
				results[i] = &types.SearchResult{ChunkIndex: pid}
			}
			hook.recordRerankResult(0, results)
			hook.recordFinish(0)

			metrics := hook.MetricResult().RetrievalMetrics
			assert.InDelta(t, tt.precision, metrics.Precision, 1e-12)
			assert.InDelta(t, tt.recall, metrics.Recall, 1e-12)
			assert.InDelta(t, tt.mrr, metrics.MRR, 1e-12)
		})
	}
}
