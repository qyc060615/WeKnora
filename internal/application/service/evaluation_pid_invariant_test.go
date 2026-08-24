package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestEvaluationSparsePIDsRemainChunkIndexes(t *testing.T) {
	qaPairs := []*types.QAPair{
		{
			PIDs:     []int{2, 10, 30},
			Passages: []string{"passage-2", "passage-10", "passage-30"},
		},
	}
	passages := getPassageList(qaPairs)
	require.Len(t, passages, 31)
	require.Equal(t, "passage-2", passages[2])
	require.Equal(t, "passage-10", passages[10])
	require.Equal(t, "passage-30", passages[30])
	require.Empty(t, passages[0])
	require.Empty(t, passages[9])

	knowledge := &types.Knowledge{
		ID:              "evaluation-knowledge",
		TenantID:        1,
		KnowledgeBaseID: "evaluation-kb",
		Type:            "passage",
		ParseStatus:     types.ParseStatusProcessing,
	}
	chunkService := &parentChildChunkService{}
	tenant := &types.Tenant{ID: 1}
	ctx := context.WithValue(context.Background(), types.TenantInfoContextKey, tenant)
	svc := &knowledgeService{
		repo:           &parentChildKnowledgeRepo{knowledge: knowledge},
		chunkService:   chunkService,
		retrieveEngine: parentChildRetrieveRegistry{engine: &parentChildRetrieveEngine{}},
		graphEngine:    parentChildGraphRepo{},
		tenantRepo:     parentChildTenantRepo{},
		task:           parentChildTaskEnqueuer{},
	}
	kb := &types.KnowledgeBase{ID: "evaluation-kb", TenantID: 1}

	svc.processDocumentFromPassage(ctx, kb, knowledge, passages)

	chunkIndexes := make([]int, 0, len(chunkService.created))
	var pid10Chunk *types.Chunk
	for _, chunk := range chunkService.created {
		if chunk.ChunkType != types.ChunkTypeText {
			continue
		}
		chunkIndexes = append(chunkIndexes, chunk.ChunkIndex)
		if chunk.Content == "passage-10" {
			pid10Chunk = chunk
		}
	}
	require.Equal(t, []int{2, 10, 30}, chunkIndexes)
	require.NotNil(t, pid10Chunk)

	result := (&knowledgeBaseService{}).buildSearchResult(
		pid10Chunk, knowledge, 1, types.MatchTypeEmbedding, pid10Chunk.Content,
	)
	require.Equal(t, 10, result.ChunkIndex)
}
