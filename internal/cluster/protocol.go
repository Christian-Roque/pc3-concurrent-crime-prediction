package cluster

// PredictionInput representa una celda zona-tiempo a evaluar.
// En PC4 la API puede enviar una consulta simple (district, community_area,
// day_of_week, hour) o un vector completo de features cuando se requiera
// una prediccion tecnica exacta.
type PredictionInput struct {
	District      int       `json:"district"`
	CommunityArea int       `json:"community_area"`
	DayOfWeek     int       `json:"day_of_week"`
	Hour          int       `json:"hour"`
	WeekStart     string    `json:"week_start,omitempty"`
	Features      []float64 `json:"features,omitempty"`
}

// PredictionRequest es el mensaje JSON que la API enviara por TCP a un nodo ML.
type PredictionRequest struct {
	RequestID  string            `json:"request_id"`
	Command    string            `json:"command"` // predict, predict_batch, health
	Input      PredictionInput   `json:"input,omitempty"`
	Candidates []PredictionInput `json:"candidates,omitempty"`
	TopN       int               `json:"top_n,omitempty"`
}

// PredictionResult representa el score generado para una celda candidata.
type PredictionResult struct {
	District      int     `json:"district"`
	CommunityArea int     `json:"community_area"`
	DayOfWeek     int     `json:"day_of_week"`
	Hour          int     `json:"hour"`
	RiskScore     float64 `json:"risk_score"`
	RiskLevel     string  `json:"risk_level"`
	NodeID        string  `json:"node_id"`
}

// PredictionResponse es el mensaje JSON que el nodo ML devuelve a la API.
type PredictionResponse struct {
	RequestID string             `json:"request_id"`
	NodeID    string             `json:"node_id"`
	Status    string             `json:"status"`
	Error     string             `json:"error,omitempty"`
	Result    *PredictionResult  `json:"result,omitempty"`
	Results   []PredictionResult `json:"results,omitempty"`
}
