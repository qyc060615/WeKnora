package service

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	"golang.org/x/sync/errgroup"
)

/*
corpus: pid -> content
queries: qid -> content
answers: aid -> content
qrels: qid -> pid
arels: qid -> aid
*/

// EvaluationService handles evaluation tasks for knowledge base and chat models
type EvaluationService struct {
	config                  *config.Config                  // Application configuration
	dataset                 interfaces.DatasetService       // Service for dataset operations
	knowledgeBaseService    interfaces.KnowledgeBaseService // Service for knowledge base operations
	knowledgeService        interfaces.KnowledgeService     // Service for knowledge operations
	sessionService          interfaces.SessionService       // Service for chat sessions
	modelService            interfaces.ModelService         // Service for model operations
	evaluationRunRepository interfaces.EvaluationRunRepository
}

func NewEvaluationService(
	config *config.Config,
	dataset interfaces.DatasetService,
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	sessionService interfaces.SessionService,
	modelService interfaces.ModelService,
	evaluationRunRepository interfaces.EvaluationRunRepository,
) interfaces.EvaluationService {
	return &EvaluationService{
		config:                  config,
		dataset:                 dataset,
		knowledgeBaseService:    knowledgeBaseService,
		knowledgeService:        knowledgeService,
		sessionService:          sessionService,
		modelService:            modelService,
		evaluationRunRepository: evaluationRunRepository,
	}
}

func (e *EvaluationService) EvaluationResult(ctx context.Context, taskID string) (*types.EvaluationDetail, error) {
	logger.Info(ctx, "Start getting evaluation result")
	logger.Infof(ctx, "Task ID: %s", taskID)

	tenantID := types.MustTenantIDFromContext(ctx)
	run, err := e.evaluationRunRepository.GetByTaskID(ctx, tenantID, taskID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get evaluation task: %v", err)
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("task not found")
	}

	logger.Info(ctx, "Evaluation result retrieved successfully")
	return evaluationRunToDetail(run), nil
}

// Evaluation starts a new evaluation task with given parameters
// datasetID: ID of the dataset to evaluate against
// knowledgeBaseID: ID of the knowledge base to use (empty to create new)
// chatModelID: ID of the chat model to evaluate
// rerankModelID: ID of the rerank model to evaluate
func (e *EvaluationService) Evaluation(ctx context.Context,
	datasetID string, knowledgeBaseID string, chatModelID string, rerankModelID string,
) (*types.EvaluationDetail, error) {
	logger.Info(ctx, "Start evaluation")
	logger.Infof(ctx, "Dataset ID: %s, Knowledge Base ID: %s, Chat Model ID: %s, Rerank Model ID: %s",
		datasetID, knowledgeBaseID, chatModelID, rerankModelID)

	// Get tenant ID from context for multi-tenancy support
	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Tenant ID: %d", tenantID)
	var sourceKnowledgeBaseID *string
	var embeddingModelID, summaryModelID string

	// Handle knowledge base creation if not provided
	if knowledgeBaseID == "" {
		logger.Info(ctx, "No knowledge base ID provided, creating new knowledge base")
		// Create new knowledge base with default evaluation settings
		// 获取默认的嵌入模型和LLM模型
		models, err := e.modelService.ListModels(ctx)
		if err != nil {
			logger.Errorf(ctx, "Failed to list models: %v", err)
			return nil, err
		}

		var llmModelID string
		for _, model := range models {
			if model == nil {
				continue
			}
			if model.Type == types.ModelTypeEmbedding {
				embeddingModelID = model.ID
			}
			if model.Type == types.ModelTypeKnowledgeQA {
				llmModelID = model.ID
			}
		}

		if embeddingModelID == "" || llmModelID == "" {
			return nil, fmt.Errorf("no default models found for evaluation")
		}
		summaryModelID = llmModelID

		kb, err := e.knowledgeBaseService.CreateKnowledgeBase(ctx, &types.KnowledgeBase{
			Name:             "evaluation",
			Description:      "evaluation",
			EmbeddingModelID: embeddingModelID,
			SummaryModelID:   llmModelID,
		})
		if err != nil {
			logger.Errorf(ctx, "Failed to create knowledge base: %v", err)
			return nil, err
		}
		knowledgeBaseID = kb.ID
		logger.Infof(ctx, "Created new knowledge base with ID: %s", knowledgeBaseID)
	} else {
		logger.Infof(ctx, "Using existing knowledge base ID: %s", knowledgeBaseID)
		// Create evaluation-specific knowledge base based on existing one
		kb, err := e.knowledgeBaseService.GetKnowledgeBaseByID(ctx, knowledgeBaseID)
		if err != nil {
			logger.Errorf(ctx, "Failed to get knowledge base: %v", err)
			return nil, err
		}
		sourceID := knowledgeBaseID
		sourceKnowledgeBaseID = &sourceID
		embeddingModelID = kb.EmbeddingModelID
		summaryModelID = kb.SummaryModelID

		kb, err = e.knowledgeBaseService.CreateKnowledgeBase(ctx, &types.KnowledgeBase{
			Name:             "evaluation",
			Description:      "evaluation",
			EmbeddingModelID: kb.EmbeddingModelID,
			SummaryModelID:   kb.SummaryModelID,
		})
		if err != nil {
			logger.Errorf(ctx, "Failed to create knowledge base: %v", err)
			return nil, err
		}
		knowledgeBaseID = kb.ID
		logger.Infof(ctx, "Created new knowledge base with ID: %s based on existing one", knowledgeBaseID)
	}

	// Set default values for optional parameters
	if datasetID == "" {
		datasetID = "default"
		logger.Info(ctx, "Using default dataset")
	}

	if rerankModelID == "" {
		// 获取默认的重排模型
		models, err := e.modelService.ListModels(ctx)
		if err == nil {
			for _, model := range models {
				if model == nil {
					continue
				}
				if model.Type == types.ModelTypeRerank {
					rerankModelID = model.ID
					break
				}
			}
		}
		if rerankModelID == "" {
			logger.Warnf(ctx, "No rerank model found, skipping rerank")
		} else {
			logger.Infof(ctx, "Using default rerank model: %s", rerankModelID)
		}
	}

	if chatModelID == "" {
		// 获取默认的LLM模型
		models, err := e.modelService.ListModels(ctx)
		if err == nil {
			for _, model := range models {
				if model == nil {
					continue
				}
				if model.Type == types.ModelTypeKnowledgeQA {
					chatModelID = model.ID
					break
				}
			}
		}
		if chatModelID == "" {
			return nil, fmt.Errorf("no default chat model found")
		}
		logger.Infof(ctx, "Using default chat model: %s", chatModelID)
	}

	// Create evaluation task with unique ID
	logger.Info(ctx, "Creating evaluation task")
	taskID := utils.GenerateTaskID("evaluation", tenantID, datasetID)
	logger.Infof(ctx, "Generated task ID: %s", taskID)

	// Prepare evaluation detail with all parameters
	detail := &types.EvaluationDetail{
		Task: &types.EvaluationTask{
			ID:        taskID,
			TenantID:  tenantID,
			DatasetID: datasetID,
			Status:    types.EvaluationStatuePending,
			StartTime: time.Now(),
		},
		Params: &types.ChatManage{
			PipelineRequest: types.PipelineRequest{
				VectorThreshold:  e.config.Conversation.VectorThreshold,
				KeywordThreshold: e.config.Conversation.KeywordThreshold,
				EmbeddingTopK:    e.config.Conversation.EmbeddingTopK,
				MaxRounds:        e.config.Conversation.MaxRounds,
				RerankModelID:    rerankModelID,
				RerankTopK:       e.config.Conversation.RerankTopK,
				RerankThreshold:  e.config.Conversation.RerankThreshold,
				ChatModelID:      chatModelID,
				SummaryConfig: types.SummaryConfig{
					MaxTokens:           e.config.Conversation.Summary.MaxTokens,
					RepeatPenalty:       e.config.Conversation.Summary.RepeatPenalty,
					TopK:                e.config.Conversation.Summary.TopK,
					TopP:                e.config.Conversation.Summary.TopP,
					Prompt:              e.config.Conversation.Summary.Prompt,
					ContextTemplate:     e.config.Conversation.Summary.ContextTemplate,
					FrequencyPenalty:    e.config.Conversation.Summary.FrequencyPenalty,
					PresencePenalty:     e.config.Conversation.Summary.PresencePenalty,
					NoMatchPrefix:       e.config.Conversation.Summary.NoMatchPrefix,
					Temperature:         e.config.Conversation.Summary.Temperature,
					Seed:                e.config.Conversation.Summary.Seed,
					MaxCompletionTokens: e.config.Conversation.Summary.MaxCompletionTokens,
				},
				FallbackResponse:    e.config.Conversation.FallbackResponse,
				RewritePromptSystem: e.config.Conversation.RewritePromptSystem,
				RewritePromptUser:   e.config.Conversation.RewritePromptUser,
			},
		},
	}

	retrieveDriver := os.Getenv("RETRIEVE_DRIVER")
	if e.config.VectorDatabase != nil && e.config.VectorDatabase.Driver != "" {
		retrieveDriver = e.config.VectorDatabase.Driver
	}
	snapshot := evaluationSnapshot(
		detail, sourceKnowledgeBaseID, embeddingModelID, summaryModelID, retrieveDriver,
	)
	run := &types.EvaluationRun{
		TaskID: taskID, TenantID: tenantID, DatasetID: datasetID,
		SourceKnowledgeBaseID: sourceKnowledgeBaseID, EmbeddingModelID: embeddingModelID,
		ChatModelID: chatModelID, Status: types.EvaluationStatuePending,
		ConfigSnapshot: snapshot, CreatedAt: detail.Task.StartTime, UpdatedAt: detail.Task.StartTime,
	}
	if rerankModelID != "" {
		rerankID := rerankModelID
		run.RerankModelID = &rerankID
	}
	logger.Info(ctx, "Persisting evaluation task")
	if err := e.evaluationRunRepository.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create evaluation run: %w", err)
	}
	evaluationRunID := run.ID
	workerTask := *detail.Task
	workerDetail := &types.EvaluationDetail{Task: &workerTask, Params: detail.Params.Clone()}

	// Start evaluation in background goroutine
	logger.Info(ctx, "Starting evaluation in background")
	go func() {
		// Create new context with logger for background task, then attribute
		// every synchronous model call in this evaluation lifecycle to the
		// run so Model Usage v1 can tag usage rows with evaluation_run_id.
		// Asynq async post-processing is a separate boundary and stays
		// unattributed (the run ID does not ride the task payload).
		newCtx := types.WithEvaluationRunID(logger.CloneContext(ctx), evaluationRunID)
		logger.Infof(newCtx, "Background evaluation started for task ID: %s", taskID)

		startedAt := time.Now()
		if err := e.evaluationRunRepository.MarkRunning(newCtx, tenantID, taskID, startedAt); err != nil {
			logger.Errorf(newCtx, "Failed to mark evaluation task running: %v", err)
			return
		}
		workerDetail.Task.Status = types.EvaluationStatueRunning
		logger.Info(newCtx, "Evaluation task status set to running")

		// Execute actual evaluation
		if err := e.EvalDataset(newCtx, workerDetail, knowledgeBaseID); err != nil {
			workerDetail.Task.Status = types.EvaluationStatueFailed
			workerDetail.Task.ErrMsg = err.Error()
			if persistErr := e.evaluationRunRepository.MarkFailed(
				newCtx, tenantID, taskID, nil, err.Error(), time.Now(),
			); persistErr != nil {
				logger.Errorf(newCtx, "Failed to persist evaluation failure: %v", persistErr)
			}
			logger.Errorf(newCtx, "Evaluation task failed: %v, task ID: %s", err, taskID)
			return
		}

		// Mark task as completed successfully
		logger.Infof(newCtx, "Evaluation task completed successfully, task ID: %s", taskID)
		if err := e.evaluationRunRepository.MarkSuccess(
			newCtx, tenantID, taskID, workerDetail.Metric, time.Now(),
		); err != nil {
			logger.Errorf(newCtx, "Failed to persist evaluation success: %v", err)
			message := fmt.Sprintf("failed to persist evaluation metrics: %v", err)
			if failErr := e.evaluationRunRepository.MarkFailed(
				newCtx, tenantID, taskID, nil, message, time.Now(),
			); failErr != nil {
				logger.Errorf(newCtx, "Failed to persist evaluation metric failure: %v", failErr)
			}
			return
		}
		workerDetail.Task.Status = types.EvaluationStatueSuccess
	}()

	logger.Infof(ctx, "Evaluation task created successfully, task ID: %s", taskID)
	return detail, nil
}

// EvalDataset performs the actual evaluation of a dataset
// Processes each QA pair in parallel and records metrics
func (e *EvaluationService) EvalDataset(ctx context.Context, detail *types.EvaluationDetail, knowledgeBaseID string) error {
	logger.Info(ctx, "Start evaluating dataset")
	logger.Infof(ctx, "Task ID: %s, Dataset ID: %s", detail.Task.ID, detail.Task.DatasetID)

	// Retrieve dataset from storage
	dataset, err := e.dataset.GetDatasetByID(ctx, detail.Task.DatasetID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get dataset: %v", err)
		return err
	}
	logger.Infof(ctx, "Dataset retrieved successfully with %d QA pairs", len(dataset))

	if err := e.evaluationRunRepository.UpdateTotal(
		ctx, detail.Task.TenantID, detail.Task.ID, len(dataset),
	); err != nil {
		return fmt.Errorf("persist evaluation total: %w", err)
	}
	detail.Task.Total = len(dataset)

	// Extract and organize passages from dataset
	passages := getPassageList(dataset)
	logger.Infof(ctx, "Creating knowledge from %d passages", len(passages))

	// Create knowledge base from passages (sync: wait for indexing to complete before querying)
	knowledge, err := e.knowledgeService.CreateKnowledgeFromPassageSync(ctx, knowledgeBaseID, passages, "")
	if err != nil {
		logger.Errorf(ctx, "Failed to create knowledge from passages: %v", err)
		return err
	}
	logger.Infof(ctx, "Knowledge created and indexed successfully, ID: %s", knowledge.ID)

	// Setup cleanup of temporary resources
	defer func() {
		logger.Infof(ctx, "Cleaning up resources - deleting knowledge: %s", knowledge.ID)
		if err := e.knowledgeService.DeleteKnowledge(ctx, knowledge.ID); err != nil {
			logger.Errorf(ctx, "Failed to delete knowledge: %v, knowledge ID: %s", err, knowledge.ID)
		}

		logger.Infof(ctx, "Cleaning up resources - deleting knowledge base: %s", knowledgeBaseID)
		if err := e.knowledgeBaseService.DeleteKnowledgeBase(ctx, knowledgeBaseID); err != nil {
			logger.Errorf(
				ctx,
				"Failed to delete knowledge base: %v, knowledge base ID: %s",
				err, knowledgeBaseID,
			)
		}
	}()

	// Initialize parallel evaluation metrics
	var g errgroup.Group
	metricHook := NewHookMetric(len(dataset))

	// Set worker limit based on available CPUs
	g.SetLimit(max(runtime.GOMAXPROCS(0)-1, 1))
	logger.Infof(ctx, "Starting evaluation with %d parallel workers", max(runtime.GOMAXPROCS(0)-1, 1))

	// Process each QA pair in parallel
	for i, qaPair := range dataset {
		qaPair := qaPair
		i := i
		g.Go(func() error {
			logger.Infof(ctx, "Processing QA pair %d, question: %s", i, qaPair.Question)

			// Prepare chat management parameters for this QA pair
			chatManage := detail.Params.Clone()
			chatManage.Query = qaPair.Question
			chatManage.RewriteQuery = qaPair.Question
			// Set knowledge base ID and search targets for this evaluation
			chatManage.KnowledgeBaseIDs = []string{knowledgeBaseID}
			chatManage.SearchTargets = types.SearchTargets{
				&types.SearchTarget{
					Type:            types.SearchTargetTypeKnowledgeBase,
					KnowledgeBaseID: knowledgeBaseID,
				},
			}

			// Execute knowledge QA pipeline
			logger.Infof(ctx, "Running knowledge QA for question: %s", qaPair.Question)
			err = e.sessionService.KnowledgeQAByEvent(ctx, chatManage, types.Pipline["rag"])
			if err != nil {
				logger.Errorf(ctx, "Failed to process question %d: %v", i, err)
				return err
			}

			// Record evaluation metrics
			logger.Infof(ctx, "Recording metrics for QA pair %d", i)
			metricHook.recordInit(i)
			metricHook.recordQaPair(i, qaPair)
			metricHook.recordSearchResult(i, chatManage.SearchResult)
			metricHook.recordRerankResult(i, chatManage.RerankResult)
			metricHook.recordChatResponse(i, chatManage.ChatResponse)
			metricHook.recordFinish(i)

			if err := e.evaluationRunRepository.IncrementFinished(
				ctx, detail.Task.TenantID, detail.Task.ID,
			); err != nil {
				return fmt.Errorf("persist evaluation progress: %w", err)
			}
			return nil
		})
	}

	// Wait for all parallel evaluations to complete
	logger.Info(ctx, "Waiting for all evaluation tasks to complete")
	if err := g.Wait(); err != nil {
		logger.Errorf(ctx, "Evaluation error: %v", err)
		return err
	}

	detail.Metric = metricHook.MetricResult()
	detail.Task.Finished = detail.Task.Total

	logger.Infof(ctx, "Dataset evaluation completed successfully, task ID: %s", detail.Task.ID)
	return nil
}

func evaluationSnapshot(
	detail *types.EvaluationDetail,
	sourceKnowledgeBaseID *string,
	embeddingModelID, summaryModelID, retrieveDriver string,
) types.EvaluationConfigSnapshotV1 {
	params := detail.Params
	var rerankModelID *string
	if params.RerankModelID != "" {
		id := params.RerankModelID
		rerankModelID = &id
	}
	snapshot := types.EvaluationConfigSnapshotV1{
		SnapshotSchemaVersion: 1,
		Dataset:               types.EvaluationDatasetSnapshot{DatasetID: detail.Task.DatasetID},
		Pipeline: types.EvaluationPipelineSnapshot{
			Name: "rag", Metrics: []string{"precision", "recall", "ndcg", "mrr", "map", "bleu", "rouge"},
			NDCGCutoffs: []int{3, 10},
		},
		Retrieval: types.EvaluationRetrievalSnapshot{
			VectorThreshold: params.VectorThreshold, KeywordThreshold: params.KeywordThreshold,
			EmbeddingTopK: params.EmbeddingTopK, RerankTopK: params.RerankTopK,
			RerankThreshold: params.RerankThreshold, RetrieveDriver: retrieveDriver,
		},
		Models: types.EvaluationModelsSnapshot{
			EmbeddingModelID: embeddingModelID, ChatModelID: params.ChatModelID,
			RerankModelID: rerankModelID, SummaryModelID: summaryModelID,
		},
		Generation: types.EvaluationGenerationSnapshot{
			MaxRounds: params.MaxRounds, SummaryConfig: params.SummaryConfig,
			FallbackResponse: params.FallbackResponse, RewritePromptSystem: params.RewritePromptSystem,
			RewritePromptUser: params.RewritePromptUser,
		},
	}
	if sourceKnowledgeBaseID != nil {
		snapshot.SourceKnowledgeBase = &types.EvaluationSourceKBSnapshot{
			ID: *sourceKnowledgeBaseID, EmbeddingModelID: embeddingModelID, SummaryModelID: summaryModelID,
		}
	}
	return snapshot
}

func evaluationRunToDetail(run *types.EvaluationRun) *types.EvaluationDetail {
	snapshot := run.ConfigSnapshot
	params := &types.ChatManage{PipelineRequest: types.PipelineRequest{
		MaxRounds:           snapshot.Generation.MaxRounds,
		VectorThreshold:     snapshot.Retrieval.VectorThreshold,
		KeywordThreshold:    snapshot.Retrieval.KeywordThreshold,
		EmbeddingTopK:       snapshot.Retrieval.EmbeddingTopK,
		RerankTopK:          snapshot.Retrieval.RerankTopK,
		RerankThreshold:     snapshot.Retrieval.RerankThreshold,
		ChatModelID:         run.ChatModelID,
		SummaryConfig:       snapshot.Generation.SummaryConfig,
		FallbackResponse:    snapshot.Generation.FallbackResponse,
		RewritePromptSystem: snapshot.Generation.RewritePromptSystem,
		RewritePromptUser:   snapshot.Generation.RewritePromptUser,
	}}
	if run.RerankModelID != nil {
		params.RerankModelID = *run.RerankModelID
	}
	detail := &types.EvaluationDetail{
		Task: &types.EvaluationTask{
			ID: run.TaskID, TenantID: run.TenantID, DatasetID: run.DatasetID,
			StartTime: run.CreatedAt, Status: run.Status, ErrMsg: run.ErrorMessage,
			Total: run.Total, Finished: run.Finished,
		},
		Params: params,
	}
	detail.Metric = evaluationRunMetric(run)
	return detail
}

func evaluationRunMetric(run *types.EvaluationRun) *types.MetricResult {
	if run.Precision == nil || run.Recall == nil || run.NDCG3 == nil || run.NDCG10 == nil ||
		run.MRR == nil || run.MAP == nil || run.BLEU1 == nil || run.BLEU2 == nil ||
		run.BLEU4 == nil || run.ROUGE1 == nil || run.ROUGE2 == nil || run.ROUGEL == nil {
		return nil
	}
	return &types.MetricResult{
		RetrievalMetrics: types.RetrievalMetrics{
			Precision: *run.Precision, Recall: *run.Recall, NDCG3: *run.NDCG3,
			NDCG10: *run.NDCG10, MRR: *run.MRR, MAP: *run.MAP,
		},
		GenerationMetrics: types.GenerationMetrics{
			BLEU1: *run.BLEU1, BLEU2: *run.BLEU2, BLEU4: *run.BLEU4,
			ROUGE1: *run.ROUGE1, ROUGE2: *run.ROUGE2, ROUGEL: *run.ROUGEL,
		},
	}
}

// getPassageList extracts and organizes passages from QA pairs
// Returns a slice of passages indexed by their passage IDs
func getPassageList(dataset []*types.QAPair) []string {
	pIDMap := make(map[int]string)
	maxPID := 0
	for _, qaPair := range dataset {
		for i := 0; i < len(qaPair.PIDs); i++ {
			pIDMap[qaPair.PIDs[i]] = qaPair.Passages[i]
			maxPID = max(maxPID, qaPair.PIDs[i])
		}
	}
	passages := make([]string, maxPID+1)
	for i := 0; i <= maxPID; i++ {
		if _, ok := pIDMap[i]; ok {
			passages[i] = pIDMap[i]
		}
	}
	return passages
}
