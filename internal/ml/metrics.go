package ml

import (
	"math"
	"sort"
)

type scoredSample struct {
	Score float64
	Label float64
}

func Evaluate(model Model, samples []Sample) Metrics {
	if len(samples) == 0 {
		return Metrics{}
	}
	var tp, tn, fp, fn int
	logLoss := 0.0
	scored := make([]scoredSample, 0, len(samples))
	positives := 0
	for _, s := range samples {
		p := PredictProbability(model.Weights, s.Features)
		pred := 0.0
		if p >= 0.5 {
			pred = 1.0
		}
		if pred == 1 && s.Label == 1 {
			tp++
		}
		if pred == 0 && s.Label == 0 {
			tn++
		}
		if pred == 1 && s.Label == 0 {
			fp++
		}
		if pred == 0 && s.Label == 1 {
			fn++
		}
		if s.Label == 1 {
			positives++
		}
		logLoss += binaryLogLoss(s.Label, p)
		scored = append(scored, scoredSample{Score: p, Label: s.Label})
	}
	accuracy := float64(tp+tn) / float64(len(samples))
	precision := safeDivide(float64(tp), float64(tp+fp))
	recall := safeDivide(float64(tp), float64(tp+fn))
	f1 := safeDivide(2*precision*recall, precision+recall)
	auc := aucApprox(scored)
	gini := 2*auc - 1
	return Metrics{
		Accuracy:  round4(accuracy),
		Precision: round4(precision),
		Recall:    round4(recall),
		F1:        round4(f1),
		AUC:       round4(auc),
		Gini:      round4(gini),
		LogLoss:   round4(logLoss / float64(len(samples))),
		Samples:   len(samples),
		Positives: positives,
		Negatives: len(samples) - positives,
		TP:        tp,
		TN:        tn,
		FP:        fp,
		FN:        fn,
	}
}

// EvaluateCompact calcula metricas sobre una matriz compacta row-major.
// Se usa para evaluar el entrenamiento final sobre la misma representacion compacta
// empleada durante el calculo de gradientes.
func EvaluateCompact(model Model, data CompactDataset) Metrics {
	if data.N == 0 || data.P == 0 {
		return Metrics{}
	}
	var tp, tn, fp, fn int
	logLoss := 0.0
	scored := make([]scoredSample, 0, data.N)
	positives := 0
	for i := 0; i < data.N; i++ {
		base := i * data.P
		z := 0.0
		if len(model.Weights) > 0 {
			z = model.Weights[0]
		}
		for j := 0; j < data.P; j++ {
			if j+1 < len(model.Weights) {
				z += model.Weights[j+1] * data.X[base+j]
			}
		}
		p := sigmoid(z)
		y := data.Y[i]
		pred := 0.0
		if p >= 0.5 {
			pred = 1.0
		}
		if pred == 1 && y == 1 {
			tp++
		}
		if pred == 0 && y == 0 {
			tn++
		}
		if pred == 1 && y == 0 {
			fp++
		}
		if pred == 0 && y == 1 {
			fn++
		}
		if y == 1 {
			positives++
		}
		logLoss += binaryLogLoss(y, p)
		scored = append(scored, scoredSample{Score: p, Label: y})
	}
	accuracy := float64(tp+tn) / float64(data.N)
	precision := safeDivide(float64(tp), float64(tp+fp))
	recall := safeDivide(float64(tp), float64(tp+fn))
	f1 := safeDivide(2*precision*recall, precision+recall)
	auc := aucApprox(scored)
	gini := 2*auc - 1
	return Metrics{
		Accuracy:  round4(accuracy),
		Precision: round4(precision),
		Recall:    round4(recall),
		F1:        round4(f1),
		AUC:       round4(auc),
		Gini:      round4(gini),
		LogLoss:   round4(logLoss / float64(data.N)),
		Samples:   data.N,
		Positives: positives,
		Negatives: data.N - positives,
		TP:        tp,
		TN:        tn,
		FP:        fp,
		FN:        fn,
	}
}

func safeDivide(a, b float64) float64 {
	if math.Abs(b) < 1e-12 {
		return 0
	}
	return a / b
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// aucApprox calcula AUC con ranking de puntajes. Es suficiente para reporte preliminar de PC3.
func aucApprox(samples []scoredSample) float64 {
	if len(samples) == 0 {
		return 0
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].Score < samples[j].Score })
	pos, neg := 0, 0
	rankSum := 0.0
	for i, s := range samples {
		if s.Label == 1 {
			pos++
			rankSum += float64(i + 1)
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		return 0
	}
	return (rankSum - float64(pos*(pos+1))/2.0) / float64(pos*neg)
}
