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
		pairs, err := service.GetDatasetByID(context.Background(), "default")
		require.NoError(t, err)
		require.NotEmpty(t, pairs)
	})

	t.Run("benchmark v1", func(t *testing.T) {
		pairs, err := service.GetDatasetByID(context.Background(), "benchmark_v1")
		require.NoError(t, err)
		require.Len(t, pairs, 15)
		for index, pair := range pairs {
			require.Equal(t, index, pair.QID, "benchmark iteration must be deterministic by QID")
			require.NotEmpty(t, pair.Question)
			require.NotEmpty(t, pair.Answer)
			require.NotEmpty(t, pair.PIDs)
			require.Len(t, pair.Passages, len(pair.PIDs))
		}
	})

	for _, datasetID := range []string{"not-exist", "../samples", "benchmark_v1/../samples", ""} {
		t.Run("reject "+datasetID, func(t *testing.T) {
			_, err := service.GetDatasetByID(context.Background(), datasetID)
			require.ErrorContains(t, err, "unsupported dataset ID")
		})
	}
}

func TestDatasetServiceReturnsLoadError(t *testing.T) {
	service := &DatasetService{datasetRoot: t.TempDir()}

	_, err := service.GetDatasetByID(context.Background(), "default")
	require.ErrorContains(t, err, `load dataset "default"`)
	require.ErrorContains(t, err, "load queries")
}
