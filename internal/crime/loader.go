package crime

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const dateLayout = "2006-01-02"

// LoadAndAggregateCSV lee el CSV en streaming y procesa los registros usando goroutines y channels.
// No carga todo el dataset en memoria; solo conserva agregaciones parciales por celda espacio-temporal.
func LoadAndAggregateCSV(path string, workers int, batchSize int, minYear int, maxRows int) (AggregationResult, error) {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if batchSize <= 0 {
		batchSize = 20000
	}
	started := time.Now()

	file, err := os.Open(path)
	if err != nil {
		return AggregationResult{}, fmt.Errorf("no se pudo abrir el archivo %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return AggregationResult{}, fmt.Errorf("no se pudo leer la cabecera del CSV: %w", err)
	}
	indices, headerMap, err := buildHeaderIndex(header)
	if err != nil {
		return AggregationResult{}, err
	}

	batches := make(chan [][]string, workers*2)
	results := make(chan WorkerResult, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			results <- processBatches(batches, indices, minYear)
		}()
	}

	readRows := 0
	currentBatch := make([][]string, 0, batchSize)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			currentBatch = append(currentBatch, nil)
		} else {
			// csv.Reader crea un slice nuevo por registro cuando ReuseRecord=false (valor por defecto).
			// Se conserva el registro leído por csv.Reader para enviarlo al lote de trabajo.
			currentBatch = append(currentBatch, record)
		}
		readRows++

		if len(currentBatch) >= batchSize {
			batches <- currentBatch
			currentBatch = make([][]string, 0, batchSize)
		}
		if maxRows > 0 && readRows >= maxRows {
			break
		}
	}
	if len(currentBatch) > 0 {
		batches <- currentBatch
	}
	close(batches)

	wg.Wait()
	close(results)

	finalAgg := make(map[AggregateKey]AggregateValue)
	stats := LoaderStats{
		Years:         make(map[int]int64),
		CrimeTypes:    make(map[string]int64),
		StartedAt:     started,
		Workers:       workers,
		BatchSize:     batchSize,
		InputFile:     path,
		HeaderMapping: headerMap,
	}

	for partial := range results {
		mergeWorkerResultInto(&stats, finalAgg, partial)
	}

	stats.AggregatedCells = len(finalAgg)
	stats.FinishedAt = time.Now()
	stats.ElapsedSeconds = stats.FinishedAt.Sub(stats.StartedAt).Seconds()

	return AggregationResult{Aggregates: finalAgg, Stats: stats}, nil
}

type recordProcessStats struct {
	TotalRows    int64
	ValidRows    int64
	InvalidRows  int64
	FilteredRows int64
	RelevantRows int64
	Year         int
	CrimeType    string
}

// processRecordIntoAggregation contiene la logica de limpieza, clasificacion y agregacion
// aplicada a cada registro valido.
func processRecordIntoAggregation(record []string, idx headerIndices, minYear int, agg map[AggregateKey]AggregateValue) recordProcessStats {
	stats := recordProcessStats{TotalRows: 1}
	if record == nil {
		stats.InvalidRows = 1
		return stats
	}
	parsed, ok := parseCrimeRecord(record, idx)
	if !ok {
		stats.InvalidRows = 1
		return stats
	}
	if parsed.Date.Year() < minYear {
		stats.FilteredRows = 1
		return stats
	}
	stats.ValidRows = 1
	stats.Year = parsed.Date.Year()
	stats.CrimeType = parsed.PrimaryType

	crimeClass := ClassifyCrimeType(parsed.PrimaryType)
	if crimeClass.Relevant {
		stats.RelevantRows = 1
	}

	ws := WeekStart(parsed.Date)
	key := AggregateKey{
		WeekStart:     ws.Format(dateLayout),
		Year:          ws.Year(),
		Month:         int(ws.Month()),
		District:      parsed.District,
		CommunityArea: parsed.CommunityArea,
		DayOfWeek:     int(parsed.Date.Weekday()),
		Hour:          parsed.Date.Hour(),
	}
	value := agg[key]
	value.Count++
	if crimeClass.Relevant {
		value.RelevantCount++
		switch crimeClass.Group {
		case "violent":
			value.ViolentCount++
		case "property":
			value.PropertyCount++
		case "weapons":
			value.WeaponsCount++
		case "severe":
			value.SevereCount++
		}
	} else {
		// OtherCount representa delitos no relevantes para el target principal.
		// No activan la etiqueta futura, pero sirven como contexto predictivo.
		value.OtherCount++
	}
	if parsed.Arrest {
		value.ArrestCount++
	}
	if parsed.Domestic {
		value.DomesticCount++
	}
	agg[key] = value
	return stats
}

func addRecordStatsToLoader(row recordProcessStats, stats *LoaderStats) {
	stats.TotalRows += row.TotalRows
	stats.ValidRows += row.ValidRows
	stats.InvalidRows += row.InvalidRows
	stats.FilteredRows += row.FilteredRows
	stats.RelevantRows += row.RelevantRows
	if row.Year != 0 {
		stats.Years[row.Year]++
	}
	if row.CrimeType != "" {
		stats.CrimeTypes[row.CrimeType]++
	}
}

func addRecordStatsToWorker(row recordProcessStats, result *WorkerResult) {
	result.TotalRows += row.TotalRows
	result.ValidRows += row.ValidRows
	result.InvalidRows += row.InvalidRows
	result.FilteredRows += row.FilteredRows
	result.RelevantRows += row.RelevantRows
	if row.Year != 0 {
		result.Years[row.Year]++
	}
	if row.CrimeType != "" {
		result.CrimeTypes[row.CrimeType]++
	}
}

type headerIndices struct {
	Date          int
	District      int
	CommunityArea int
	PrimaryType   int
	Arrest        int
	Domestic      int
	Year          int
}

func buildHeaderIndex(header []string) (headerIndices, map[string]string, error) {
	lookup := map[string]int{}
	original := map[string]string{}
	for i, h := range header {
		n := normalizeHeader(h)
		lookup[n] = i
		original[n] = h
	}

	idx := headerIndices{
		Date:          findAny(lookup, "date"),
		District:      findAny(lookup, "district"),
		CommunityArea: findAny(lookup, "communityarea", "community_area"),
		PrimaryType:   findAny(lookup, "primarytype", "primary_type"),
		Arrest:        findAny(lookup, "arrest"),
		Domestic:      findAny(lookup, "domestic"),
		Year:          findAny(lookup, "year"),
	}

	missing := []string{}
	if idx.Date < 0 {
		missing = append(missing, "Date")
	}
	if idx.District < 0 {
		missing = append(missing, "District")
	}
	if idx.CommunityArea < 0 {
		missing = append(missing, "Community Area")
	}
	if idx.PrimaryType < 0 {
		missing = append(missing, "Primary Type")
	}
	if len(missing) > 0 {
		return idx, nil, fmt.Errorf("faltan columnas obligatorias en el CSV: %s", strings.Join(missing, ", "))
	}

	mapping := map[string]string{
		"date":           safeOriginal(original, "date"),
		"district":       safeOriginal(original, "district"),
		"community_area": safeOriginal(original, "communityarea"),
		"primary_type":   safeOriginal(original, "primarytype"),
		"arrest":         safeOriginal(original, "arrest"),
		"domestic":       safeOriginal(original, "domestic"),
		"year":           safeOriginal(original, "year"),
	}
	return idx, mapping, nil
}

func safeOriginal(original map[string]string, key string) string {
	if v, ok := original[key]; ok {
		return v
	}
	return ""
}

func normalizeHeader(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "", "(", "", ")", "")
	return replacer.Replace(s)
}

func findAny(lookup map[string]int, candidates ...string) int {
	for _, c := range candidates {
		if idx, ok := lookup[normalizeHeader(c)]; ok {
			return idx
		}
	}
	return -1
}

func newWorkerResult() WorkerResult {
	return WorkerResult{
		Aggregates: make(map[AggregateKey]AggregateValue),
		Years:      make(map[int]int64),
		CrimeTypes: make(map[string]int64),
	}
}

func processBatchIntoResult(batch [][]string, idx headerIndices, minYear int, result *WorkerResult) {
	for _, record := range batch {
		addRecordStatsToWorker(processRecordIntoAggregation(record, idx, minYear, result.Aggregates), result)
	}
}

func processBatches(batches <-chan [][]string, idx headerIndices, minYear int) WorkerResult {
	result := newWorkerResult()
	for batch := range batches {
		processBatchIntoResult(batch, idx, minYear, &result)
	}
	return result
}

func mergeWorkerResultInto(stats *LoaderStats, finalAgg map[AggregateKey]AggregateValue, partial WorkerResult) {
	stats.TotalRows += partial.TotalRows
	stats.ValidRows += partial.ValidRows
	stats.InvalidRows += partial.InvalidRows
	stats.FilteredRows += partial.FilteredRows
	stats.RelevantRows += partial.RelevantRows
	for year, count := range partial.Years {
		stats.Years[year] += count
	}
	for t, count := range partial.CrimeTypes {
		stats.CrimeTypes[t] += count
	}
	for key, value := range partial.Aggregates {
		current := finalAgg[key]
		current.Count += value.Count
		current.RelevantCount += value.RelevantCount
		current.ViolentCount += value.ViolentCount
		current.PropertyCount += value.PropertyCount
		current.WeaponsCount += value.WeaponsCount
		current.SevereCount += value.SevereCount
		current.OtherCount += value.OtherCount
		current.ArrestCount += value.ArrestCount
		current.DomesticCount += value.DomesticCount
		finalAgg[key] = current
	}
}

type parsedCrime struct {
	Date          time.Time
	District      int
	CommunityArea int
	PrimaryType   string
	Arrest        bool
	Domestic      bool
}

func parseCrimeRecord(record []string, idx headerIndices) (parsedCrime, bool) {
	if idx.Date >= len(record) || idx.District >= len(record) || idx.CommunityArea >= len(record) || idx.PrimaryType >= len(record) {
		return parsedCrime{}, false
	}

	date, ok := parseDate(record[idx.Date])
	if !ok {
		return parsedCrime{}, false
	}
	district, ok := parsePositiveInt(record[idx.District])
	if !ok {
		return parsedCrime{}, false
	}
	communityArea, ok := parsePositiveInt(record[idx.CommunityArea])
	if !ok {
		return parsedCrime{}, false
	}

	primaryType := strings.TrimSpace(strings.ToUpper(record[idx.PrimaryType]))
	arrest := false
	if idx.Arrest >= 0 && idx.Arrest < len(record) {
		arrest = parseBool(record[idx.Arrest])
	}
	domestic := false
	if idx.Domestic >= 0 && idx.Domestic < len(record) {
		domestic = parseBool(record[idx.Domestic])
	}

	return parsedCrime{Date: date, District: district, CommunityArea: communityArea, PrimaryType: primaryType, Arrest: arrest, Domestic: domestic}, true
}

func WeekStart(t time.Time) time.Time {
	base := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	// time.Weekday: Sunday=0. Convertimos lunes a inicio de semana.
	offset := (int(base.Weekday()) + 6) % 7
	return base.AddDate(0, 0, -offset)
}

func AddWeeks(weekStart string, weeks int) string {
	t, err := time.Parse(dateLayout, weekStart)
	if err != nil {
		return weekStart
	}
	return t.AddDate(0, 0, 7*weeks).Format(dateLayout)
}

func ParseWeekStart(weekStart string) (time.Time, bool) {
	t, err := time.Parse(dateLayout, weekStart)
	return t, err == nil
}

// ClassifyCrimeType identifica delitos relevantes de mayor impacto para seguridad ciudadana.
// No todos los eventos del dataset se tratan igual: el target preventivo usa delitos severos, violentos, armas y patrimoniales de alto impacto.
// Delitos de menor impacto relativo, como THEFT, CRIMINAL DAMAGE y CRIMINAL TRESPASS, no activan el target; se mantienen como contexto predictivo.
func ClassifyCrimeType(primaryType string) CrimeClass {
	t := strings.TrimSpace(strings.ToUpper(primaryType))
	switch t {
	case "HOMICIDE", "CRIMINAL SEXUAL ASSAULT", "SEX OFFENSE", "KIDNAPPING", "HUMAN TRAFFICKING":
		return CrimeClass{Relevant: true, Group: "severe"}
	case "ROBBERY", "ASSAULT", "BATTERY", "INTIMIDATION", "STALKING":
		return CrimeClass{Relevant: true, Group: "violent"}
	case "WEAPONS VIOLATION", "CONCEALED CARRY LICENSE VIOLATION":
		return CrimeClass{Relevant: true, Group: "weapons"}
	case "BURGLARY", "MOTOR VEHICLE THEFT", "ARSON":
		return CrimeClass{Relevant: true, Group: "property"}
	case "THEFT", "CRIMINAL DAMAGE", "CRIMINAL TRESPASS":
		return CrimeClass{Relevant: false, Group: "other"}
	default:
		return CrimeClass{Relevant: false, Group: "other"}
	}
}

func parsePositiveInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f <= 0 {
			return 0, false
		}
		return int(f), true
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func parseBool(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "true" || s == "t" || s == "1" || s == "yes" || s == "y" || s == "si" || s == "sí"
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"01/02/2006 03:04:05 PM",
		"1/2/2006 3:04:05 PM",
		"01/02/2006 15:04:05",
		"1/2/2006 15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
