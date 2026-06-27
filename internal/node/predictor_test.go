package node

import (
	"testing"

	"pc3-seguridad-ciudadana/internal/cluster"
	"pc3-seguridad-ciudadana/internal/ml"
)

func TestBuildFeaturesFromSimpleInput(t *testing.T) {
	model := ml.Model{
		FeatureNames: []string{"district_norm", "community_area_norm", "hour_sin", "hour_cos", "day_sin", "day_cos", "district_onehot_11", "community_area_onehot_23"},
		Weights:      []float64{0, 1, 1, 1, 1, 1, 1, 1, 1},
		Means:        make([]float64, 8),
		Stds:         []float64{1, 1, 1, 1, 1, 1, 1, 1},
	}
	p := &Predictor{NodeID: "test-node", Model: model}
	features, err := p.buildFeatures(cluster.PredictionInput{District: 11, CommunityArea: 23, DayOfWeek: 2, Hour: 18})
	if err != nil {
		t.Fatalf("buildFeatures error: %v", err)
	}
	if len(features) != len(model.FeatureNames) {
		t.Fatalf("features=%d, esperado=%d", len(features), len(model.FeatureNames))
	}
	if features[6] != 1 || features[7] != 1 {
		t.Fatalf("one-hot esperado en district=11 y community_area=23, recibido %v", features)
	}
}
