package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPricingImportCommandSQLiteSmokeAndIdempotency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pricing-import.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	for _, path := range []string{
		"../../migrations/sqlite/000013_evaluation_runs.up.sql",
		"../../migrations/sqlite/000014_model_usage.up.sql",
		"../../migrations/sqlite/000015_model_pricing.up.sql",
	} {
		ddl, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(ddl)).Error)
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", dbPath)
	var first bytes.Buffer
	require.NoError(t, run([]string{"--file", "../../config/model_pricing/example.yaml"}, &first))
	require.Contains(t, first.String(), "pricing_version=example-v1")
	require.Contains(t, first.String(), "inserted=1 no_op=0 closed=0")

	var second bytes.Buffer
	require.NoError(t, run([]string{"--file", "../../config/model_pricing/example.yaml"}, &second))
	require.Contains(t, second.String(), "inserted=0 no_op=1 closed=0")
}
