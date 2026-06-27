package crime

import "time"

// AggregateKey representa la unidad espacio-temporal usada por PC3:
// semana + distrito + area comunitaria + dia de semana + hora.
// La semana inicia lunes para poder construir variables de recencia.
type AggregateKey struct {
	WeekStart     string `json:"week_start"` // formato YYYY-MM-DD, lunes de la semana
	Year          int    `json:"year"`
	Month         int    `json:"month"`
	District      int    `json:"district"`
	CommunityArea int    `json:"community_area"`
	DayOfWeek     int    `json:"day_of_week"` // 0=domingo, 6=sabado
	Hour          int    `json:"hour"`
}

// AggregateValue conserva conteos agregados por celda espacio-temporal.
// Count incluye todos los registros validos; RelevantCount y sus subgrupos se usan para el modelo preventivo.
type AggregateValue struct {
	Count         int `json:"count"`
	RelevantCount int `json:"relevant_count"`
	ViolentCount  int `json:"violent_count"`
	PropertyCount int `json:"property_count"`
	WeaponsCount  int `json:"weapons_count"`
	SevereCount   int `json:"severe_count"`
	OtherCount    int `json:"other_count"` // delitos no relevantes usados como contexto predictivo
	ArrestCount   int `json:"arrest_count"`
	DomesticCount int `json:"domestic_count"`
}

// CrimeClass representa una agrupacion simple por relevancia de seguridad ciudadana.
type CrimeClass struct {
	Relevant bool
	Group    string // violent, property, weapons, severe, other
}

// LoaderStats almacena indicadores de auditoria del proceso de carga y limpieza.
type LoaderStats struct {
	TotalRows       int64             `json:"total_rows"`
	ValidRows       int64             `json:"valid_rows"`
	InvalidRows     int64             `json:"invalid_rows"`
	FilteredRows    int64             `json:"filtered_rows"`
	AggregatedCells int               `json:"aggregated_cells"`
	Years           map[int]int64     `json:"years"`
	CrimeTypes      map[string]int64  `json:"crime_types"`
	RelevantRows    int64             `json:"relevant_rows"`
	StartedAt       time.Time         `json:"started_at"`
	FinishedAt      time.Time         `json:"finished_at"`
	ElapsedSeconds  float64           `json:"elapsed_seconds"`
	Workers         int               `json:"workers"`
	BatchSize       int               `json:"batch_size"`
	InputFile       string            `json:"input_file"`
	HeaderMapping   map[string]string `json:"header_mapping"`
}

// AggregationResult agrupa el resultado de procesamiento concurrente.
type AggregationResult struct {
	Aggregates map[AggregateKey]AggregateValue `json:"aggregates"`
	Stats      LoaderStats                     `json:"stats"`
}

// WorkerResult es enviado por cada worker al reducer principal.
type WorkerResult struct {
	Aggregates   map[AggregateKey]AggregateValue
	TotalRows    int64
	ValidRows    int64
	InvalidRows  int64
	FilteredRows int64
	RelevantRows int64
	Years        map[int]int64
	CrimeTypes   map[string]int64
}
