package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/parquet-go/parquet-go"
)

// DatasetService provides operations for working with datasets
type DatasetService struct {
	datasetRoot string
}

// NewDatasetService creates a new DatasetService instance
func NewDatasetService() interfaces.DatasetService {
	return &DatasetService{datasetRoot: "./dataset"}
}

var datasetDirectories = map[string]string{
	"default":      "samples",
	"benchmark_v1": "benchmark_v1",
}

// TextInfo represents text data with ID in parquet format
type TextInfo struct {
	ID   int64  `parquet:"id"`   // Unique identifier
	Text string `parquet:"text"` // Text content
}

// RelsInfo represents question-passage relations in parquet format
type RelsInfo struct {
	QID int64 `parquet:"qid"` // Question ID
	PID int64 `parquet:"pid"` // Passage ID
}

// QaInfo represents question-answer relations in parquet format
type QaInfo struct {
	QID int64 `parquet:"qid"` // Question ID
	AID int64 `parquet:"aid"` // Answer ID
}

// GetDatasetByID retrieves the complete candidate corpus independently from
// qrels, plus QA ground truth and a deterministic semantic identity.
func (d *DatasetService) GetDatasetByID(ctx context.Context, datasetID string) (*types.EvaluationDataset, error) {
	logger.Info(ctx, "Start getting dataset by ID")
	logger.Infof(ctx, "Getting dataset with ID: %s", datasetID)

	directory, ok := datasetDirectories[datasetID]
	if !ok {
		return nil, fmt.Errorf("unsupported dataset ID %q", datasetID)
	}

	root := d.datasetRoot
	if root == "" {
		root = "./dataset"
	}
	dataset, err := loadDataset(filepath.Join(root, directory))
	if err != nil {
		return nil, fmt.Errorf("load dataset %q: %w", datasetID, err)
	}
	dataset.PrintStats(ctx)
	descriptor, err := dataset.describe(datasetID)
	if err != nil {
		return nil, fmt.Errorf("describe dataset %q: %w", datasetID, err)
	}

	logger.Infof(ctx, "Retrieved %d corpus passages and %d QA pairs from dataset",
		len(descriptor.Corpus), len(descriptor.QAPairs))
	return descriptor, nil
}

// DefaultDataset loads and initializes the default dataset from parquet files
func DefaultDataset() dataset {
	dataset, err := loadDataset(filepath.Join("dataset", datasetDirectories["default"]))
	if err != nil {
		panic(err)
	}
	return dataset
}

func loadDataset(datasetDir string) (dataset, error) {
	queries, err := loadParquet[TextInfo](fmt.Sprintf("%s/queries.parquet", datasetDir))
	if err != nil {
		return dataset{}, fmt.Errorf("load queries: %w", err)
	}
	corpus, err := loadParquet[TextInfo](fmt.Sprintf("%s/corpus.parquet", datasetDir))
	if err != nil {
		return dataset{}, fmt.Errorf("load corpus: %w", err)
	}
	answers, err := loadParquet[TextInfo](fmt.Sprintf("%s/answers.parquet", datasetDir))
	if err != nil {
		return dataset{}, fmt.Errorf("load answers: %w", err)
	}
	qrels, err := loadParquet[RelsInfo](fmt.Sprintf("%s/qrels.parquet", datasetDir))
	if err != nil {
		return dataset{}, fmt.Errorf("load qrels: %w", err)
	}
	qas, err := loadParquet[QaInfo](fmt.Sprintf("%s/qas.parquet", datasetDir))
	if err != nil {
		return dataset{}, fmt.Errorf("load qas: %w", err)
	}

	res := dataset{
		queries: make(map[int64]string),  // qid -> question text
		corpus:  make(map[int64]string),  // pid -> passage text
		answers: make(map[int64]string),  // aid -> answer text
		qrels:   make(map[int64][]int64), // qid -> list of pid
		qas:     make(map[int64]int64),   // qid -> aid
	}
	for _, qi := range queries {
		res.queries[qi.ID] = qi.Text
	}
	for _, ci := range corpus {
		res.corpus[ci.ID] = ci.Text
	}
	for _, ai := range answers {
		res.answers[ai.ID] = ai.Text
	}
	for _, ri := range qrels {
		res.qrels[ri.QID] = append(res.qrels[ri.QID], ri.PID)
	}
	for _, qi := range qas {
		res.qas[qi.QID] = qi.AID
	}
	return res, nil
}

// dataset represents the in-memory dataset structure
type dataset struct {
	queries map[int64]string  // qid -> question text
	corpus  map[int64]string  // pid -> passage text
	answers map[int64]string  // aid -> answer text
	qrels   map[int64][]int64 // qid -> list of related pids
	qas     map[int64]int64   // qid -> aid
}

// Iterate generates QA pairs from the dataset
func (d *dataset) Iterate() []*types.QAPair {
	var pairs []*types.QAPair

	qids := make([]int64, 0, len(d.queries))
	for qid := range d.queries {
		qids = append(qids, qid)
	}
	sort.Slice(qids, func(i, j int) bool { return qids[i] < qids[j] })

	for _, qid := range qids {
		question := d.queries[qid]
		// Get answer info
		aid, hasAnswer := d.qas[qid]
		answer := ""
		if hasAnswer {
			answer = d.answers[aid]
		}

		// Get related passages
		pids := d.qrels[qid]
		var pidStr []int
		for _, pid := range pids {
			pidStr = append(pidStr, int(pid))
		}
		pairs = append(pairs, &types.QAPair{
			QID:      int(qid),
			Question: question,
			PIDs:     pidStr,
			AID:      int(aid),
			Answer:   answer,
		})
	}

	return pairs
}

type datasetSemanticCorpusFact struct {
	PID  int64  `json:"pid"`
	Text string `json:"text"`
}

type datasetSemanticQueryFact struct {
	QID      int64  `json:"qid"`
	Question string `json:"question"`
}

type datasetSemanticQrelFact struct {
	QID  int64   `json:"qid"`
	PIDs []int64 `json:"pids"`
}

type datasetSemanticAnswerFact struct {
	AID    int64  `json:"aid"`
	Answer string `json:"answer"`
}

type datasetSemanticQAFact struct {
	QID int64 `json:"qid"`
	AID int64 `json:"aid"`
}

type datasetSemanticFacts struct {
	Corpus  []datasetSemanticCorpusFact `json:"corpus"`
	Queries []datasetSemanticQueryFact  `json:"queries"`
	Qrels   []datasetSemanticQrelFact   `json:"qrels"`
	Answers []datasetSemanticAnswerFact `json:"answers"`
	QAs     []datasetSemanticQAFact     `json:"qas"`
}

func (d *dataset) describe(datasetID string) (*types.EvaluationDataset, error) {
	facts := d.semanticFacts()
	canonical, err := json.Marshal(facts)
	if err != nil {
		return nil, fmt.Errorf("marshal semantic facts: %w", err)
	}
	digest := sha256.Sum256(canonical)

	corpus := make([]types.EvaluationPassage, len(facts.Corpus))
	for i, passage := range facts.Corpus {
		corpus[i] = types.EvaluationPassage{PID: int(passage.PID), Text: passage.Text}
	}
	qrelsCount := 0
	for _, qrel := range facts.Qrels {
		qrelsCount += len(qrel.PIDs)
	}

	return &types.EvaluationDataset{
		ID:      datasetID,
		Corpus:  corpus,
		QAPairs: d.Iterate(),
		Identity: types.EvaluationDatasetIdentity{
			DatasetID: datasetID, DatasetSemanticSHA256: hex.EncodeToString(digest[:]),
			CorpusCount: len(d.corpus), QuestionCount: len(d.queries),
			QrelsCount: qrelsCount, AnswerCount: len(d.answers),
		},
	}, nil
}

func (d *dataset) semanticFacts() datasetSemanticFacts {
	facts := datasetSemanticFacts{
		Corpus:  make([]datasetSemanticCorpusFact, 0, len(d.corpus)),
		Queries: make([]datasetSemanticQueryFact, 0, len(d.queries)),
		Qrels:   make([]datasetSemanticQrelFact, 0, len(d.qrels)),
		Answers: make([]datasetSemanticAnswerFact, 0, len(d.answers)),
		QAs:     make([]datasetSemanticQAFact, 0, len(d.qas)),
	}
	for pid, content := range d.corpus {
		facts.Corpus = append(facts.Corpus, datasetSemanticCorpusFact{PID: pid, Text: content})
	}
	for qid, question := range d.queries {
		facts.Queries = append(facts.Queries, datasetSemanticQueryFact{QID: qid, Question: question})
	}
	for qid, pids := range d.qrels {
		ordered := append([]int64(nil), pids...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		facts.Qrels = append(facts.Qrels, datasetSemanticQrelFact{QID: qid, PIDs: ordered})
	}
	for aid, answer := range d.answers {
		facts.Answers = append(facts.Answers, datasetSemanticAnswerFact{AID: aid, Answer: answer})
	}
	for qid, aid := range d.qas {
		facts.QAs = append(facts.QAs, datasetSemanticQAFact{QID: qid, AID: aid})
	}
	sort.Slice(facts.Corpus, func(i, j int) bool { return facts.Corpus[i].PID < facts.Corpus[j].PID })
	sort.Slice(facts.Queries, func(i, j int) bool { return facts.Queries[i].QID < facts.Queries[j].QID })
	sort.Slice(facts.Qrels, func(i, j int) bool { return facts.Qrels[i].QID < facts.Qrels[j].QID })
	sort.Slice(facts.Answers, func(i, j int) bool { return facts.Answers[i].AID < facts.Answers[j].AID })
	sort.Slice(facts.QAs, func(i, j int) bool { return facts.QAs[i].QID < facts.QAs[j].QID })
	return facts
}

// GetContextForQID retrieves context passages for a given question ID
func (d *dataset) GetContextForQID(qid int64) ([]string, error) {
	pids, ok := d.qrels[qid]
	if !ok {
		return nil, errors.New("question ID not found")
	}

	var contextParts []string
	for _, pid := range pids {
		if text, exists := d.corpus[pid]; exists {
			contextParts = append(contextParts, text)
		}
	}

	return contextParts, nil
}

// PrintStats prints dataset statistics to the logger
func (d *dataset) PrintStats(ctx context.Context) {
	logger.Infof(ctx, "QA System Statistics:")
	logger.Infof(ctx, "- Total queries: %d", len(d.queries))
	logger.Infof(ctx, "- Total corpus passages: %d", len(d.corpus))
	logger.Infof(ctx, "- Total answers: %d", len(d.answers))

	// Calculate average passages per query
	totalRelations := 0
	for _, pids := range d.qrels {
		totalRelations += len(pids)
	}
	avgPassages := float64(totalRelations) / float64(len(d.qrels))
	logger.Infof(ctx, "- Average passages per query: %.2f", avgPassages)

	// Calculate coverage
	coveredQueries := len(d.qas)
	coverage := float64(coveredQueries) / float64(len(d.queries)) * 100
	logger.Infof(ctx, "- Answer coverage: %.2f%% (%d/%d)", coverage, coveredQueries, len(d.queries))
}

// PrintRandomQA prints a random question with its related passages and answer
func (d *dataset) PrintRandomQA() error {
	// Get a random qid
	var qid int64
	for k := range d.qas {
		qid = k
		break
	}
	if qid == 0 {
		return errors.New("no questions available")
	}

	// Get question text
	question, ok := d.queries[qid]
	if !ok {
		return fmt.Errorf("question %d not found", qid)
	}

	// Get answer info
	aid, ok := d.qas[qid]
	if !ok {
		return fmt.Errorf("answer for question %d not found", qid)
	}
	answer, ok := d.answers[aid]
	if !ok {
		return fmt.Errorf("answer %d not found", aid)
	}

	// Print formatted QA
	fmt.Println("===== Random QA =====")
	fmt.Printf("QID: %d\n", qid)
	fmt.Printf("Question: %s\n", question)

	// Print passages if available
	if pids, exists := d.qrels[qid]; exists && len(pids) > 0 {
		fmt.Println("\nRelated passages:")
		for i, pid := range pids {
			if text, exists := d.corpus[pid]; exists {
				fmt.Printf("\nPassage %d (PID: %d):\n%s\n", i+1, pid, text)
			}
		}
	} else {
		fmt.Println("\nNo related passages found")
	}

	// Print answer
	fmt.Printf("\nAnswer (AID: %d):\n%s\n", aid, answer)

	return nil
}

// loadParquet loads data from parquet file into specified type
func loadParquet[T any](filePath string) ([]T, error) {
	rows, err := parquet.ReadFile[T](filePath)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
