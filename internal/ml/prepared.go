package ml

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// PreparedMetadata describe el dataset ML ya construido.
// La preparacion de observaciones se ejecuta una sola vez antes del entrenamiento.
type PreparedMetadata struct {
	FeatureNames []string   `json:"feature_names"`
	Means        []float64  `json:"means"`
	Stds         []float64  `json:"stds"`
	TargetInfo   TargetInfo `json:"target_info"`
	TrainCount   int        `json:"train_count"`
	TestCount    int        `json:"test_count"`
	CreatedAt    time.Time  `json:"created_at"`
	Notes        string     `json:"notes"`
}

var preparedMetaHeader = []string{
	"week_start",
	"target_week_start",
	"year",
	"district",
	"community_area",
	"day_of_week",
	"hour",
	"current_relevant_count",
	"current_minor_count",
	"current_total_incidents",
	"next_week_relevant_count",
	"label",
}

// SavePreparedObservationsCSV guarda las observaciones ML ya estandarizadas.
// Este archivo se genera una vez en cmd/prepare y luego se reutiliza en cmd/train.
func SavePreparedObservationsCSV(path string, samples []Sample, featureNames []string) error {
	w, closeFn, err := newPreparedCSVWriter(path)
	if err != nil {
		return err
	}
	defer closeFn()
	header := append(append([]string{}, preparedMetaHeader...), featureNames...)
	if err := w.Write(header); err != nil {
		return err
	}
	for _, s := range samples {
		record := []string{
			s.WeekStart,
			s.TargetWeek,
			strconv.Itoa(s.Year),
			strconv.Itoa(s.District),
			strconv.Itoa(s.CommunityArea),
			strconv.Itoa(s.DayOfWeek),
			strconv.Itoa(s.Hour),
			strconv.Itoa(s.RelevantCount),
			strconv.Itoa(s.OtherCount),
			strconv.Itoa(s.TotalCount),
			strconv.Itoa(s.NextRelevant),
			strconv.FormatFloat(s.Label, 'f', 0, 64),
		}
		for _, x := range s.Features {
			record = append(record, strconv.FormatFloat(x, 'g', -1, 64))
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return w.Error()
}

// LoadPreparedObservationsCSV carga observaciones ML previamente generadas por cmd/prepare.
// La lectura es streaming: no usa ReadAll() para evitar duplicar en memoria millones de filas CSV.
func LoadPreparedObservationsCSV(path string, expectedFeatures []string) ([]Sample, error) {
	r, closeFn, err := newPreparedCSVReader(path)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	metaCount := len(preparedMetaHeader)
	if len(header) < metaCount {
		return nil, fmt.Errorf("archivo preparado invalido: columnas insuficientes")
	}
	featureNames := header[metaCount:]
	if err := validatePreparedFeatures(featureNames, expectedFeatures); err != nil {
		return nil, err
	}

	samples := make([]Sample, 0, 1024)
	rowIdx := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rowIdx++
		if len(rec) < metaCount {
			continue
		}
		parseInt := func(pos int) int {
			v, _ := strconv.Atoi(rec[pos])
			return v
		}
		label, _ := strconv.ParseFloat(rec[11], 64)
		features := make([]float64, 0, len(featureNames))
		for i := metaCount; i < len(rec); i++ {
			x, err := strconv.ParseFloat(rec[i], 64)
			if err != nil {
				return nil, fmt.Errorf("feature invalida fila=%d columna=%d valor=%q", rowIdx, i+1, rec[i])
			}
			features = append(features, x)
		}
		samples = append(samples, Sample{
			WeekStart:     rec[0],
			TargetWeek:    rec[1],
			Year:          parseInt(2),
			District:      parseInt(3),
			CommunityArea: parseInt(4),
			DayOfWeek:     parseInt(5),
			Hour:          parseInt(6),
			RelevantCount: parseInt(7),
			OtherCount:    parseInt(8),
			TotalCount:    parseInt(9),
			NextRelevant:  parseInt(10),
			Label:         label,
			Features:      features,
		})
	}
	return samples, nil
}

// LoadPreparedObservationsCompactCSV carga solo features y label en matriz densa row-major.
// Es usada por el entrenamiento de entrenamiento para procesar workers=1 vs workers>1 sobre
// la misma estructura de datos y evitar overhead de millones de slices Sample.Features.
func LoadPreparedObservationsCompactCSV(path string, expectedFeatures []string, maxRows int) (CompactDataset, error) {
	r, closeFn, err := newPreparedCSVReader(path)
	if err != nil {
		return CompactDataset{}, err
	}
	defer closeFn()
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return CompactDataset{}, err
	}
	metaCount := len(preparedMetaHeader)
	if len(header) < metaCount {
		return CompactDataset{}, fmt.Errorf("archivo preparado invalido: columnas insuficientes")
	}
	featureNames := header[metaCount:]
	if err := validatePreparedFeatures(featureNames, expectedFeatures); err != nil {
		return CompactDataset{}, err
	}
	p := len(featureNames)
	initialCap := 1024 * p
	if maxRows > 0 {
		initialCap = maxRows * p
	}
	X := make([]float64, 0, initialCap)
	Y := make([]float64, 0, 1024)
	rowIdx := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return CompactDataset{}, err
		}
		rowIdx++
		if len(rec) < metaCount {
			continue
		}
		if maxRows > 0 && len(Y) >= maxRows {
			break
		}
		label, _ := strconv.ParseFloat(rec[11], 64)
		Y = append(Y, label)
		for i := metaCount; i < metaCount+p; i++ {
			if i >= len(rec) {
				return CompactDataset{}, fmt.Errorf("fila=%d con columnas incompletas: tiene=%d esperado=%d", rowIdx, len(rec), metaCount+p)
			}
			x, err := strconv.ParseFloat(rec[i], 64)
			if err != nil {
				return CompactDataset{}, fmt.Errorf("feature invalida fila=%d columna=%d valor=%q", rowIdx, i+1, rec[i])
			}
			X = append(X, x)
		}
	}
	return CompactDataset{X: X, Y: Y, N: len(Y), P: p}, nil
}

func validatePreparedFeatures(featureNames []string, expectedFeatures []string) error {
	if len(expectedFeatures) > 0 && len(featureNames) != len(expectedFeatures) {
		return fmt.Errorf("cantidad de features incompatible: archivo=%d esperado=%d", len(featureNames), len(expectedFeatures))
	}
	if len(expectedFeatures) > 0 {
		for i := range expectedFeatures {
			if featureNames[i] != expectedFeatures[i] {
				return fmt.Errorf("feature incompatible en columna %d: archivo=%s esperado=%s", i, featureNames[i], expectedFeatures[i])
			}
		}
	}
	return nil
}

func newPreparedCSVWriter(path string) (*csv.Writer, func() error, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	var gz *gzip.Writer
	var writer io.Writer = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err = gzip.NewWriterLevel(f, gzip.BestSpeed)
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		writer = gz
	}
	cw := csv.NewWriter(writer)
	closeFn := func() error {
		cw.Flush()
		if err := cw.Error(); err != nil {
			_ = f.Close()
			return err
		}
		if gz != nil {
			if err := gz.Close(); err != nil {
				_ = f.Close()
				return err
			}
		}
		return f.Close()
	}
	return cw, closeFn, nil
}

func newPreparedCSVReader(path string) (*csv.Reader, func() error, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	var gz *gzip.Reader
	var reader io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err = gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		reader = gz
	}
	closeFn := func() error {
		if gz != nil {
			_ = gz.Close()
		}
		return f.Close()
	}
	return csv.NewReader(reader), closeFn, nil
}

func SavePreparedMetadata(path string, train []Sample, test []Sample, featureNames []string, means []float64, stds []float64, info TargetInfo) error {
	meta := PreparedMetadata{
		FeatureNames: featureNames,
		Means:        means,
		Stds:         stds,
		TargetInfo:   info,
		TrainCount:   len(train),
		TestCount:    len(test),
		CreatedAt:    time.Now(),
		Notes:        "Dataset ML preparado una sola vez a partir de los agregados semanales. cmd/train reutiliza estos archivos para entrenar sin reconstruir observaciones.",
	}
	return SaveJSON(path, meta)
}

func LoadPreparedMetadata(path string) (PreparedMetadata, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PreparedMetadata{}, err
	}
	var meta PreparedMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return PreparedMetadata{}, err
	}
	return meta, nil
}
