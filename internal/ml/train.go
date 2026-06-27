package ml

import (
	"math"
	"runtime"
	"sync"
	"time"
)

type trainPartial struct {
	Gradient []float64
	Loss     float64
	Count    int
}

func TrainLogisticRegression(train []Sample, test []Sample, means []float64, stds []float64, targetInfo TargetInfo, epochs int, learningRate float64, lambda float64, workers int, trainUntilYear int) Model {
	return TrainLogisticRegressionWithFeatures(FeatureNames, train, test, means, stds, targetInfo, epochs, learningRate, lambda, workers, trainUntilYear)
}

func TrainLogisticRegressionWithFeatures(featureNames []string, train []Sample, test []Sample, means []float64, stds []float64, targetInfo TargetInfo, epochs int, learningRate float64, lambda float64, workers int, trainUntilYear int) Model {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if epochs <= 0 {
		epochs = 50
	}
	if learningRate <= 0 {
		learningRate = 0.05
	}
	if lambda < 0 {
		lambda = 0.0
	}

	nFeatures := len(featureNames)
	weights := make([]float64, nFeatures+1) // weight[0] es intercepto.
	history := make([]TrainingHistory, 0, epochs)
	start := time.Now()

	for epoch := 1; epoch <= epochs; epoch++ {
		p := computeGradientForWorkers(train, weights, workers)
		totalGrad := p.Gradient
		totalLoss := p.Loss
		totalCount := p.Count
		if totalCount == 0 {
			break
		}

		for j := range weights {
			reg := 0.0
			if j > 0 {
				reg = lambda * weights[j]
			}
			weights[j] -= learningRate * ((totalGrad[j] / float64(totalCount)) + reg)
		}

		if epoch == 1 || epoch == epochs || epoch%5 == 0 {
			history = append(history, TrainingHistory{
				Epoch:       epoch,
				LogLoss:     totalLoss / float64(totalCount),
				ElapsedSecs: time.Since(start).Seconds(),
			})
		}
	}

	model := Model{
		ModelType:      "regresion_logistica_binaria_con_gradiente_paralelo",
		TargetInfo:     targetInfo,
		FeatureNames:   featureNames,
		Weights:        weights,
		Means:          means,
		Stds:           stds,
		LearningRate:   learningRate,
		Lambda:         lambda,
		Epochs:         epochs,
		Workers:        workers,
		TrainUntilYear: trainUntilYear,
		History:        history,
		CreatedAt:      time.Now(),
		Notes:          "El target es ocurrencia futura: 1 si en la siguiente semana ocurre al menos un delito relevante en la misma zona, dia de semana y hora. El entrenamiento divide el dataset en bloques y calcula gradientes parciales con goroutines.",
	}
	model.TrainMetrics = Evaluate(model, train)
	model.TestMetrics = Evaluate(model, test)
	return model
}

// TrainLogisticRegressionCompactWithFeatures entrena el modelo final usando una
// representacion compacta row-major para procesar millones de observaciones con menor overhead de memoria.
func TrainLogisticRegressionCompactWithFeatures(featureNames []string, trainData CompactDataset, test []Sample, means []float64, stds []float64, targetInfo TargetInfo, epochs int, learningRate float64, lambda float64, workers int, trainUntilYear int) Model {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if epochs <= 0 {
		epochs = 50
	}
	if learningRate <= 0 {
		learningRate = 0.05
	}
	if lambda < 0 {
		lambda = 0.0
	}
	if len(featureNames) == 0 {
		featureNames = FeatureNames
	}

	weights := make([]float64, len(featureNames)+1)
	history := make([]TrainingHistory, 0, epochs)
	start := time.Now()

	for epoch := 1; epoch <= epochs; epoch++ {
		p := computeGradientCompactForWorkers(trainData, weights, workers)
		if p.Count == 0 {
			break
		}
		for j := range weights {
			reg := 0.0
			if j > 0 {
				reg = lambda * weights[j]
			}
			weights[j] -= learningRate * ((p.Gradient[j] / float64(p.Count)) + reg)
		}
		if epoch == 1 || epoch == epochs || epoch%5 == 0 {
			history = append(history, TrainingHistory{
				Epoch:       epoch,
				LogLoss:     p.Loss / float64(p.Count),
				ElapsedSecs: time.Since(start).Seconds(),
			})
		}
	}

	model := Model{
		ModelType:      "regresion_logistica_binaria_con_gradiente_paralelo_compacto",
		TargetInfo:     targetInfo,
		FeatureNames:   featureNames,
		Weights:        weights,
		Means:          means,
		Stds:           stds,
		LearningRate:   learningRate,
		Lambda:         lambda,
		Epochs:         epochs,
		Workers:        workers,
		TrainUntilYear: trainUntilYear,
		History:        history,
		CreatedAt:      time.Now(),
		Notes:          "El entrenamiento usa una matriz compacta row-major. Workers=1 y workers>1 recorren la misma estructura; solo cambia la division del calculo de gradientes entre goroutines.",
	}
	model.TrainMetrics = EvaluateCompact(model, trainData)
	model.TestMetrics = Evaluate(model, test)
	return model
}

func computeGradient(samples []Sample, weights []float64) trainPartial {
	grad := make([]float64, len(weights))
	loss := 0.0
	for _, sample := range samples {
		p := PredictProbability(weights, sample.Features)
		errorTerm := p - sample.Label
		grad[0] += errorTerm
		for j, x := range sample.Features {
			grad[j+1] += errorTerm * x
		}
		loss += binaryLogLoss(sample.Label, p)
	}
	return trainPartial{Gradient: grad, Loss: loss, Count: len(samples)}
}

// computeGradientForWorkers divide las observaciones y calcula gradientes parciales.
// Con workers>1 usa goroutines para paralelizar el calculo.
func computeGradientForWorkers(samples []Sample, weights []float64, workers int) trainPartial {
	if workers <= 1 || len(samples) == 0 {
		return computeGradient(samples, weights)
	}
	grad := make([]float64, len(weights))
	partials := make(chan trainPartial, workers)
	var wg sync.WaitGroup
	chunkSize := int(math.Ceil(float64(len(samples)) / float64(workers)))
	if chunkSize < 1 {
		chunkSize = 1
	}
	launched := 0
	for w := 0; w < workers; w++ {
		from := w * chunkSize
		to := from + chunkSize
		if from >= len(samples) {
			break
		}
		if to > len(samples) {
			to = len(samples)
		}
		chunk := samples[from:to]
		launched++
		wg.Add(1)
		go func(part []Sample, weightsSnapshot []float64) {
			defer wg.Done()
			partials <- computeGradient(part, weightsSnapshot)
		}(chunk, cloneFloat64(weights))
	}
	wg.Wait()
	close(partials)
	totalLoss := 0.0
	totalCount := 0
	_ = launched
	for p := range partials {
		for j := range grad {
			grad[j] += p.Gradient[j]
		}
		totalLoss += p.Loss
		totalCount += p.Count
	}
	return trainPartial{Gradient: grad, Loss: totalLoss, Count: totalCount}
}

func PredictProbability(weights []float64, features []float64) float64 {
	z := 0.0
	if len(weights) > 0 {
		z = weights[0]
	}
	for j, x := range features {
		if j+1 < len(weights) {
			z += weights[j+1] * x
		}
	}
	return sigmoid(z)
}

func PredictRiskLabel(p float64) string {
	if p >= 0.70 {
		return "Alto"
	}
	if p >= 0.40 {
		return "Medio"
	}
	return "Bajo"
}

func sigmoid(z float64) float64 {
	if z >= 0 {
		e := math.Exp(-z)
		return 1.0 / (1.0 + e)
	}
	e := math.Exp(z)
	return e / (1.0 + e)
}

func binaryLogLoss(y, p float64) float64 {
	const eps = 1e-15
	if p < eps {
		p = eps
	}
	if p > 1-eps {
		p = 1 - eps
	}
	return -(y*math.Log(p) + (1-y)*math.Log(1-p))
}

func cloneFloat64(in []float64) []float64 {
	out := make([]float64, len(in))
	copy(out, in)
	return out
}

// CompactDataset almacena las observaciones en una matriz densa row-major.
// Se usa para entrenar el modelo final con una matriz densa y reducir overhead de memoria.
type CompactDataset struct {
	X []float64
	Y []float64
	N int
	P int
}

func computeGradientCompactRange(data CompactDataset, weights []float64, from int, to int) trainPartial {
	if from < 0 {
		from = 0
	}
	if to > data.N {
		to = data.N
	}
	if to < from {
		to = from
	}
	grad := make([]float64, len(weights))
	loss := 0.0
	pFeatures := data.P
	for i := from; i < to; i++ {
		base := i * pFeatures
		z := weights[0]
		for j := 0; j < pFeatures; j++ {
			z += weights[j+1] * data.X[base+j]
		}
		prob := sigmoid(z)
		y := data.Y[i]
		errorTerm := prob - y
		grad[0] += errorTerm
		for j := 0; j < pFeatures; j++ {
			grad[j+1] += errorTerm * data.X[base+j]
		}
		loss += binaryLogLoss(y, prob)
	}
	return trainPartial{Gradient: grad, Loss: loss, Count: to - from}
}

func computeGradientCompactForWorkers(data CompactDataset, weights []float64, workers int) trainPartial {
	if data.N == 0 {
		return trainPartial{Gradient: make([]float64, len(weights)), Loss: 0, Count: 0}
	}
	if workers <= 1 {
		return computeGradientCompactRange(data, weights, 0, data.N)
	}
	grad := make([]float64, len(weights))
	partials := make(chan trainPartial, workers)
	var wg sync.WaitGroup
	chunkSize := int(math.Ceil(float64(data.N) / float64(workers)))
	if chunkSize < 1 {
		chunkSize = 1
	}
	for w := 0; w < workers; w++ {
		from := w * chunkSize
		to := from + chunkSize
		if from >= data.N {
			break
		}
		if to > data.N {
			to = data.N
		}
		wg.Add(1)
		go func(start int, end int, weightsSnapshot []float64) {
			defer wg.Done()
			partials <- computeGradientCompactRange(data, weightsSnapshot, start, end)
		}(from, to, cloneFloat64(weights))
	}
	wg.Wait()
	close(partials)
	totalLoss := 0.0
	totalCount := 0
	for p := range partials {
		for j := range grad {
			grad[j] += p.Gradient[j]
		}
		totalLoss += p.Loss
		totalCount += p.Count
	}
	return trainPartial{Gradient: grad, Loss: totalLoss, Count: totalCount}
}
