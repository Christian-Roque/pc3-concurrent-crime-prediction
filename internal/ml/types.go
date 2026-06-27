package ml

import (
	"encoding/json"
	"os"
	"time"
)

type Sample struct {
	Features      []float64 `json:"features"`
	Label         float64   `json:"label"`
	Year          int       `json:"year"`
	WeekStart     string    `json:"week_start"`
	TargetWeek    string    `json:"target_week_start"`
	District      int       `json:"district"`
	CommunityArea int       `json:"community_area"`
	DayOfWeek     int       `json:"day_of_week"`
	Hour          int       `json:"hour"`
	RelevantCount int       `json:"relevant_count_current_week"`
	OtherCount    int       `json:"minor_or_context_count_current_week"`
	TotalCount    int       `json:"total_incidents_current_week"`
	NextRelevant  int       `json:"relevant_count_next_week"`
}

type TargetInfo struct {
	TargetName              string `json:"target_name"`
	Definition              string `json:"definition"`
	CandidateCells          int    `json:"candidate_cells"`
	PositiveCells           int    `json:"positive_cells"`
	NegativeCells           int    `json:"negative_cells"`
	KeptNegativeCells       int    `json:"kept_negative_cells"`
	NegativeSamplingRatio   int    `json:"negative_sampling_ratio"`
	UsesAllNegatives        bool   `json:"uses_all_negatives"`
	TargetBefore            string `json:"target_before"`
	RelevantCrimeDefinition string `json:"relevant_crime_definition"`
}

type Metrics struct {
	Accuracy  float64 `json:"accuracy"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
	AUC       float64 `json:"auc_approx"`
	Gini      float64 `json:"gini"`
	LogLoss   float64 `json:"log_loss"`
	Samples   int     `json:"samples"`
	Positives int     `json:"positives"`
	Negatives int     `json:"negatives"`
	TP        int     `json:"true_positive"`
	TN        int     `json:"true_negative"`
	FP        int     `json:"false_positive"`
	FN        int     `json:"false_negative"`
}

type TrainingHistory struct {
	Epoch       int     `json:"epoch"`
	LogLoss     float64 `json:"log_loss"`
	ElapsedSecs float64 `json:"elapsed_seconds"`
}

type Model struct {
	ModelType      string            `json:"model_type"`
	TargetInfo     TargetInfo        `json:"target_info"`
	FeatureNames   []string          `json:"feature_names"`
	Weights        []float64         `json:"weights"`
	Means          []float64         `json:"means"`
	Stds           []float64         `json:"stds"`
	LearningRate   float64           `json:"learning_rate"`
	Lambda         float64           `json:"lambda_l2"`
	Epochs         int               `json:"epochs"`
	Workers        int               `json:"workers"`
	TrainUntilYear int               `json:"train_until_year"`
	TrainMetrics   Metrics           `json:"train_metrics"`
	TestMetrics    Metrics           `json:"test_metrics"`
	History        []TrainingHistory `json:"history"`
	CreatedAt      time.Time         `json:"created_at"`
	Notes          string            `json:"notes"`
}

func SaveJSON(path string, value any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func LoadModel(path string) (Model, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Model{}, err
	}
	var m Model
	if err := json.Unmarshal(b, &m); err != nil {
		return Model{}, err
	}
	return m, nil
}
