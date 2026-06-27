package node

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"pc3-seguridad-ciudadana/internal/cluster"
	"pc3-seguridad-ciudadana/internal/ml"
)

type Predictor struct {
	NodeID string
	Model  ml.Model
}

func LoadPredictor(nodeID, modelPath string) (*Predictor, error) {
	model, err := ml.LoadModel(modelPath)
	if err != nil {
		return nil, fmt.Errorf("cargar modelo %s: %w", modelPath, err)
	}
	if len(model.Weights) == 0 {
		return nil, fmt.Errorf("modelo sin pesos")
	}
	return &Predictor{NodeID: nodeID, Model: model}, nil
}

func (p *Predictor) FeatureCount() int {
	return len(p.Model.FeatureNames)
}

func (p *Predictor) Predict(input cluster.PredictionInput) (cluster.PredictionResult, error) {
	features, err := p.buildFeatures(input)
	if err != nil {
		return cluster.PredictionResult{}, err
	}
	score := ml.PredictProbability(p.Model.Weights, features)
	return cluster.PredictionResult{
		District:      input.District,
		CommunityArea: input.CommunityArea,
		DayOfWeek:     input.DayOfWeek,
		Hour:          input.Hour,
		RiskScore:     round4(score),
		RiskLevel:     ml.PredictRiskLabel(score),
		NodeID:        p.NodeID,
	}, nil
}

func (p *Predictor) PredictBatch(candidates []cluster.PredictionInput, topN int) ([]cluster.PredictionResult, error) {
	results := make([]cluster.PredictionResult, 0, len(candidates))
	for _, c := range candidates {
		r, err := p.Predict(c)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	// Ordenamiento simple descendente por score para que cada nodo pueda devolver su top parcial.
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].RiskScore > results[i].RiskScore {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if topN > 0 && topN < len(results) {
		results = results[:topN]
	}
	return results, nil
}

func (p *Predictor) buildFeatures(input cluster.PredictionInput) ([]float64, error) {
	nFeatures := len(p.Model.FeatureNames)
	if len(input.Features) > 0 {
		if len(input.Features) != nFeatures {
			return nil, fmt.Errorf("features recibidas=%d, esperadas=%d", len(input.Features), nFeatures)
		}
		return append([]float64(nil), input.Features...), nil
	}
	if input.District <= 0 || input.CommunityArea <= 0 {
		return nil, fmt.Errorf("district y community_area deben ser positivos")
	}
	if input.DayOfWeek < 0 || input.DayOfWeek > 6 {
		return nil, fmt.Errorf("day_of_week debe estar entre 0 y 6")
	}
	if input.Hour < 0 || input.Hour > 23 {
		return nil, fmt.Errorf("hour debe estar entre 0 y 23")
	}

	raw := make([]float64, nFeatures)
	month := 1
	if strings.TrimSpace(input.WeekStart) != "" {
		if t, err := time.Parse("2006-01-02", input.WeekStart); err == nil {
			month = int(t.Month())
		}
	}
	for i, name := range p.Model.FeatureNames {
		switch name {
		case "district_norm":
			raw[i] = float64(input.District) / 25.0
		case "community_area_norm":
			raw[i] = float64(input.CommunityArea) / 77.0
		case "hour_sin":
			raw[i] = math.Sin(2 * math.Pi * float64(input.Hour) / 24.0)
		case "hour_cos":
			raw[i] = math.Cos(2 * math.Pi * float64(input.Hour) / 24.0)
		case "day_sin":
			raw[i] = math.Sin(2 * math.Pi * float64(input.DayOfWeek) / 7.0)
		case "day_cos":
			raw[i] = math.Cos(2 * math.Pi * float64(input.DayOfWeek) / 7.0)
		case "month_sin":
			raw[i] = math.Sin(2 * math.Pi * float64(month) / 12.0)
		case "month_cos":
			raw[i] = math.Cos(2 * math.Pi * float64(month) / 12.0)
		case "is_weekend":
			if input.DayOfWeek == 0 || input.DayOfWeek == 6 {
				raw[i] = 1
			}
		case "is_night":
			if input.Hour >= 0 && input.Hour <= 5 {
				raw[i] = 1
			}
		case "is_morning":
			if input.Hour >= 6 && input.Hour <= 11 {
				raw[i] = 1
			}
		case "is_afternoon":
			if input.Hour >= 12 && input.Hour <= 17 {
				raw[i] = 1
			}
		case "is_evening":
			if input.Hour >= 18 && input.Hour <= 23 {
				raw[i] = 1
			}
		case "is_friday_saturday_night":
			if (input.DayOfWeek == 5 || input.DayOfWeek == 6) && input.Hour >= 18 {
				raw[i] = 1
			}
		default:
			if strings.HasPrefix(name, "district_onehot_") {
				if suffixMatchesInt(name, "district_onehot_", input.District) {
					raw[i] = 1
				}
			} else if strings.HasPrefix(name, "community_area_onehot_") {
				if suffixMatchesInt(name, "community_area_onehot_", input.CommunityArea) {
					raw[i] = 1
				}
			}
		}
	}
	return ml.StandardizeRawFeatures(raw, p.Model.Means, p.Model.Stds), nil
}

func suffixMatchesInt(name, prefix string, value int) bool {
	s := strings.TrimPrefix(name, prefix)
	v, err := strconv.Atoi(s)
	return err == nil && v == value
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
