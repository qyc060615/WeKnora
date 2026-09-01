package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var jiebaDictionaryFiles = []string{
	"jieba.dict.utf8",
	"hmm_model.utf8",
	"user.dict.utf8",
	"idf.utf8",
	"stop_words.utf8",
}

func evaluationTokenizerSnapshot() (types.EvaluationTokenizerSnapshot, error) {
	directory := os.Getenv("JIEBA_DICT_DIR")
	if directory == "" {
		return types.EvaluationTokenizerSnapshot{Name: "jieba", DictionaryMode: "builtin"}, nil
	}

	hash := sha256.New()
	for _, name := range jiebaDictionaryFiles {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return types.EvaluationTokenizerSnapshot{}, fmt.Errorf("fingerprint custom jieba dictionary %s: %w", name, err)
		}
		// Include the stable basename and byte length so concatenation cannot make
		// distinct file sets ambiguous. The absolute local path is never stored.
		if _, err := fmt.Fprintf(hash, "%s\x00%d\x00", name, len(data)); err != nil {
			return types.EvaluationTokenizerSnapshot{}, err
		}
		if _, err := hash.Write(data); err != nil {
			return types.EvaluationTokenizerSnapshot{}, err
		}
	}
	return types.EvaluationTokenizerSnapshot{
		Name: "jieba", DictionaryMode: "custom", DictionaryFingerprint: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func loadEvaluationModelSnapshots(
	ctx context.Context,
	modelService interfaces.ModelService,
	embeddingModelID, chatModelID, rerankModelID, summaryModelID string,
) (types.EvaluationModelsSnapshot, error) {
	result := types.EvaluationModelsSnapshot{
		EmbeddingModelID: embeddingModelID,
		ChatModelID:      chatModelID,
		SummaryModelID:   summaryModelID,
	}
	if rerankModelID != "" {
		id := rerankModelID
		result.RerankModelID = &id
	}

	cache := make(map[string]*types.Model, 4)
	load := func(id string) (*types.Model, error) {
		if id == "" {
			return nil, nil
		}
		if model, ok := cache[id]; ok {
			return model, nil
		}
		model, err := modelService.GetModelByID(ctx, id)
		if err != nil {
			return nil, err
		}
		cache[id] = model
		return model, nil
	}

	embedding, err := load(embeddingModelID)
	if err != nil {
		return types.EvaluationModelsSnapshot{}, fmt.Errorf("snapshot embedding model: %w", err)
	}
	chat, err := load(chatModelID)
	if err != nil {
		return types.EvaluationModelsSnapshot{}, fmt.Errorf("snapshot chat model: %w", err)
	}
	rerank, err := load(rerankModelID)
	if err != nil {
		return types.EvaluationModelsSnapshot{}, fmt.Errorf("snapshot rerank model: %w", err)
	}
	summary, err := load(summaryModelID)
	if err != nil {
		return types.EvaluationModelsSnapshot{}, fmt.Errorf("snapshot summary model: %w", err)
	}

	result.Embedding = configuredModelSnapshot(embedding, true)
	result.Chat = configuredModelSnapshot(chat, false)
	result.Rerank = configuredModelSnapshot(rerank, false)
	result.Summary = configuredModelSnapshot(summary, false)
	return result, nil
}

func configuredModelSnapshot(model *types.Model, includeEmbeddingParameters bool) *types.EvaluationConfiguredModelSnapshot {
	if model == nil {
		return nil
	}
	snapshot := &types.EvaluationConfiguredModelSnapshot{
		ID: model.ID, Name: model.Name, Type: string(model.Type), Source: string(model.Source),
		Provider: model.Parameters.Provider, InterfaceType: model.Parameters.InterfaceType,
	}
	if includeEmbeddingParameters {
		params := model.Parameters.EmbeddingParameters
		snapshot.Embedding = &types.EvaluationEmbeddingSnapshot{
			Dimension: params.Dimension, TruncatePromptTokens: params.TruncatePromptTokens,
			SupportsDimensionOverride: params.SupportsDimensionOverride,
		}
	}
	return snapshot
}
