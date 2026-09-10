package types

// EvaluationPassage is one retrieval candidate in an evaluation dataset.
// PID is preserved as the temporary knowledge-base ChunkIndex so retrieval
// metrics can compare SearchResult.ChunkIndex directly with QAPair.PIDs.
type EvaluationPassage struct {
	PID  int    `json:"pid"`
	Text string `json:"text"`
}

// EvaluationDatasetIdentity is a stable semantic identity for the facts used
// by one evaluation. The digest is computed from sorted corpus, query, qrel,
// answer, and QA-link facts rather than Parquet bytes.
type EvaluationDatasetIdentity struct {
	DatasetID             string `json:"dataset_id"`
	DatasetSemanticSHA256 string `json:"dataset_semantic_sha256"`
	CorpusCount           int    `json:"corpus_count"`
	QuestionCount         int    `json:"question_count"`
	QrelsCount            int    `json:"qrels_count"`
	AnswerCount           int    `json:"answer_count"`
}

// EvaluationDataset keeps the retrieval candidate universe separate from
// qrels. Corpus is the complete candidate set; QAPair.PIDs are ground truth.
type EvaluationDataset struct {
	ID       string                    `json:"id"`
	Corpus   []EvaluationPassage       `json:"corpus"`
	QAPairs  []*QAPair                 `json:"qa_pairs"`
	Identity EvaluationDatasetIdentity `json:"identity"`
}

// QAPair represents a complete QA example with question, related passages and answer
type QAPair struct {
	QID      int    // Question ID
	Question string // Question text
	PIDs     []int  // Relevant passage IDs (ground truth only)
	AID      int    // Answer ID
	Answer   string // Answer text
}
