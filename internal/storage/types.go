package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pc3-seguridad-ciudadana/internal/cluster"
)

// PredictionRecord representa una prediccion guardada en almacenamiento persistente.
// Se usa en PC4 para evidenciar que la API no solo responde, sino que tambien
// registra historico de consultas, nodo usado, latencia y resultado.
type PredictionRecord struct {
	RequestID string                   `json:"request_id"`
	QueryType string                   `json:"query_type"`
	Input     cluster.PredictionInput  `json:"input"`
	Result    cluster.PredictionResult `json:"result"`
	Node      cluster.NodeInfo         `json:"node"`
	Cached    bool                     `json:"cached"`
	LatencyMS int64                    `json:"latency_ms"`
	CreatedAt time.Time                `json:"created_at"`
}

// Store integra Redis (cache) y MongoDB (persistencia). Ambos son opcionales
// para permitir pruebas locales sin levantar servicios externos.
type Store struct {
	Mongo *MongoClient
	Redis *RedisClient
	TTL   time.Duration
}

func NewStore(mongoAddr, mongoDB, mongoCollection, redisAddr string, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	st := &Store{TTL: ttl}
	if strings.TrimSpace(mongoAddr) != "" {
		st.Mongo = NewMongoClient(mongoAddr, mongoDB, mongoCollection)
	}
	if strings.TrimSpace(redisAddr) != "" {
		st.Redis = NewRedisClient(redisAddr, ttl)
	}
	return st
}

func (s *Store) Enabled() bool {
	return s != nil && (s.Mongo != nil || s.Redis != nil)
}

func (s *Store) CacheEnabled() bool { return s != nil && s.Redis != nil }
func (s *Store) MongoEnabled() bool { return s != nil && s.Mongo != nil }

func (s *Store) CacheKey(input cluster.PredictionInput) string {
	return CacheKey(input)
}

func CacheKey(input cluster.PredictionInput) string {
	if len(input.Features) == 0 {
		return fmt.Sprintf("risk:district=%d:community=%d:dow=%d:hour=%d:week=%s", input.District, input.CommunityArea, input.DayOfWeek, input.Hour, input.WeekStart)
	}
	b, _ := json.Marshal(input.Features)
	sum := sha1.Sum(b)
	return fmt.Sprintf("risk:features:%s", hex.EncodeToString(sum[:]))
}

func (s *Store) GetCachedPrediction(input cluster.PredictionInput) (cluster.PredictionResult, bool, error) {
	if s == nil || s.Redis == nil {
		return cluster.PredictionResult{}, false, nil
	}
	var result cluster.PredictionResult
	ok, err := s.Redis.GetJSON(CacheKey(input), &result)
	return result, ok, err
}

func (s *Store) SetCachedPrediction(input cluster.PredictionInput, result cluster.PredictionResult) error {
	if s == nil || s.Redis == nil {
		return nil
	}
	return s.Redis.SetJSON(CacheKey(input), result, s.TTL)
}

func (s *Store) SavePrediction(record PredictionRecord) error {
	if s == nil || s.Mongo == nil {
		return nil
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	return s.Mongo.InsertPrediction(record)
}

func (s *Store) History(limit int) ([]PredictionRecord, error) {
	if s == nil || s.Mongo == nil {
		return []PredictionRecord{}, nil
	}
	return s.Mongo.FindPredictions(limit)
}

func (s *Store) Status() map[string]any {
	status := map[string]any{
		"mongo_enabled":     false,
		"redis_enabled":     false,
		"cache_ttl_seconds": int64(0),
	}
	if s == nil {
		return status
	}
	status["mongo_enabled"] = s.Mongo != nil
	status["redis_enabled"] = s.Redis != nil
	status["cache_ttl_seconds"] = int64(s.TTL.Seconds())
	if s.Mongo != nil {
		status["mongo_addr"] = s.Mongo.Addr
		status["mongo_db"] = s.Mongo.DB
		status["mongo_collection"] = s.Mongo.Collection
	}
	if s.Redis != nil {
		status["redis_addr"] = s.Redis.Addr
	}
	return status
}
