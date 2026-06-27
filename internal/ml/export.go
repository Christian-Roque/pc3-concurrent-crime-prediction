package ml

import (
	"encoding/csv"
	"os"
	"sort"
	"strconv"
)

// SaveSamplesCSV exporta una muestra del dataset de entrenamiento construido para evidenciar la transformacion ML.
func SaveSamplesCSV(path string, samples []Sample, maxRows int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	header := []string{"week_start", "target_week_start", "year", "district", "community_area", "day_of_week", "hour", "current_relevant_count", "current_minor_count", "current_total_incidents", "next_week_relevant_count", "target_occurred_next_week"}
	header = append(header, FeatureNames...)
	if err := w.Write(header); err != nil {
		return err
	}
	limit := len(samples)
	if maxRows > 0 && maxRows < limit {
		limit = maxRows
	}
	for i := 0; i < limit; i++ {
		s := samples[i]
		record := []string{
			s.WeekStart, s.TargetWeek, strconv.Itoa(s.Year), strconv.Itoa(s.District), strconv.Itoa(s.CommunityArea), strconv.Itoa(s.DayOfWeek), strconv.Itoa(s.Hour),
			strconv.Itoa(s.RelevantCount), strconv.Itoa(s.OtherCount), strconv.Itoa(s.TotalCount), strconv.Itoa(s.NextRelevant), strconv.FormatFloat(s.Label, 'f', 0, 64),
		}
		for _, x := range s.Features {
			record = append(record, strconv.FormatFloat(x, 'f', 6, 64))
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return w.Error()
}

// SavePredictionsCSV exporta las zonas-horarios con mayor score de riesgo estimado.
func SavePredictionsCSV(path string, model Model, samples []Sample, maxRows int) error {
	type pred struct {
		Sample
		RiskScore float64
		RiskLevel string
	}
	preds := make([]pred, 0, len(samples))
	for _, s := range samples {
		score := PredictProbability(model.Weights, s.Features)
		preds = append(preds, pred{Sample: s, RiskScore: score})
	}
	sort.Slice(preds, func(i, j int) bool { return preds[i].RiskScore > preds[j].RiskScore })
	for i := range preds {
		preds[i].RiskLevel = rankedRiskLevel(i, len(preds))
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	header := []string{"week_start", "target_week_start", "district", "community_area", "day_of_week", "hour", "risk_score_estimated", "risk_level_ranked", "target", "next_week_relevant_count", "current_minor_count", "current_total_incidents"}
	if err := w.Write(header); err != nil {
		return err
	}
	limit := len(preds)
	if maxRows > 0 && maxRows < limit {
		limit = maxRows
	}
	for i := 0; i < limit; i++ {
		p := preds[i]
		record := []string{
			p.WeekStart, p.TargetWeek, strconv.Itoa(p.District), strconv.Itoa(p.CommunityArea), strconv.Itoa(p.DayOfWeek), strconv.Itoa(p.Hour),
			strconv.FormatFloat(round4(p.RiskScore), 'f', 4, 64), p.RiskLevel, strconv.FormatFloat(p.Label, 'f', 0, 64), strconv.Itoa(p.NextRelevant), strconv.Itoa(p.OtherCount), strconv.Itoa(p.TotalCount),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return w.Error()
}

func rankedRiskLevel(rank int, total int) string {
	if total <= 0 {
		return "Bajo"
	}
	position := float64(rank+1) / float64(total)
	if position <= 0.10 {
		return "Alto"
	}
	if position <= 0.30 {
		return "Medio"
	}
	return "Bajo"
}
