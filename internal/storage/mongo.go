package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"pc3-seguridad-ciudadana/internal/cluster"
)

const opMsg int32 = 2013

var mongoRequestID atomic.Int32

// MongoClient es un cliente minimo para ejecutar comandos OP_MSG basicos contra MongoDB.
// Implementa solo lo que requiere PC4: insertar predicciones y leer historial.
type MongoClient struct {
	Addr       string
	DB         string
	Collection string
	Timeout    time.Duration
}

func NewMongoClient(addr, db, collection string) *MongoClient {
	if strings.TrimSpace(db) == "" {
		db = "pc4"
	}
	if strings.TrimSpace(collection) == "" {
		collection = "predictions"
	}
	return &MongoClient{Addr: addr, DB: db, Collection: collection, Timeout: 3 * time.Second}
}

func (c *MongoClient) InsertPrediction(record PredictionRecord) error {
	if c == nil || strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("mongo no configurado")
	}
	doc := predictionRecordToBSON(record)
	cmd := []bsonElement{
		{Key: "insert", Value: c.Collection},
		{Key: "documents", Value: bsonArray{doc}},
		{Key: "ordered", Value: true},
		{Key: "$db", Value: c.DB},
	}
	reply, err := c.runCommand(cmd)
	if err != nil {
		return err
	}
	if ok, _ := asFloat(reply["ok"]); ok != 1 {
		return fmt.Errorf("mongo insert no confirmado: %v", reply)
	}
	return nil
}

func (c *MongoClient) FindPredictions(limit int) ([]PredictionRecord, error) {
	if c == nil || strings.TrimSpace(c.Addr) == "" {
		return nil, fmt.Errorf("mongo no configurado")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	cmd := []bsonElement{
		{Key: "find", Value: c.Collection},
		{Key: "sort", Value: []bsonElement{{Key: "created_at", Value: int32(-1)}}},
		{Key: "limit", Value: int32(limit)},
		{Key: "$db", Value: c.DB},
	}
	reply, err := c.runCommand(cmd)
	if err != nil {
		return nil, err
	}
	cursor, ok := reply["cursor"].(bsonDoc)
	if !ok {
		return []PredictionRecord{}, nil
	}
	batch, ok := cursor["firstBatch"].(bsonArray)
	if !ok {
		return []PredictionRecord{}, nil
	}
	records := make([]PredictionRecord, 0, len(batch))
	for _, item := range batch {
		doc, ok := item.(bsonDoc)
		if !ok {
			continue
		}
		records = append(records, bsonToPredictionRecord(doc))
	}
	return records, nil
}

func (c *MongoClient) Ping() error {
	cmd := []bsonElement{{Key: "ping", Value: int32(1)}, {Key: "$db", Value: c.DB}}
	reply, err := c.runCommand(cmd)
	if err != nil {
		return err
	}
	if ok, _ := asFloat(reply["ok"]); ok != 1 {
		return fmt.Errorf("ping mongo no confirmado")
	}
	return nil
}

func (c *MongoClient) runCommand(elements []bsonElement) (bsonDoc, error) {
	body, err := encodeBSONOrdered(elements)
	if err != nil {
		return nil, err
	}
	requestID := mongoRequestID.Add(1)
	msg := buildOPMsg(requestID, body)

	conn, err := net.DialTimeout("tcp", c.Addr, c.Timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.Timeout))
	if _, err := conn.Write(msg); err != nil {
		return nil, err
	}
	return readOPMsg(conn)
}

func buildOPMsg(requestID int32, command []byte) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.LittleEndian, int32(0))
	_ = binary.Write(buf, binary.LittleEndian, requestID)
	_ = binary.Write(buf, binary.LittleEndian, int32(0))
	_ = binary.Write(buf, binary.LittleEndian, opMsg)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // flagBits
	buf.WriteByte(0)                                      // kind 0: body
	buf.Write(command)
	b := buf.Bytes()
	binary.LittleEndian.PutUint32(b[:4], uint32(len(b)))
	return b
}

func readOPMsg(r io.Reader) (bsonDoc, error) {
	header := make([]byte, 16)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	length := int(int32(binary.LittleEndian.Uint32(header[:4])))
	opcode := int32(binary.LittleEndian.Uint32(header[12:16]))
	if length < 21 || length > 16*1024*1024 {
		return nil, fmt.Errorf("mensaje mongo invalido length=%d", length)
	}
	body := make([]byte, length-16)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	if opcode != opMsg {
		return nil, fmt.Errorf("opcode mongo no soportado: %d", opcode)
	}
	if len(body) < 5 {
		return nil, fmt.Errorf("OP_MSG sin body")
	}
	// body: flagBits(4) + kind(1) + bson
	if body[4] != 0 {
		return nil, fmt.Errorf("OP_MSG kind no soportado: %d", body[4])
	}
	return decodeBSON(body[5:])
}

func predictionRecordToBSON(r PredictionRecord) bsonDoc {
	created := r.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	return bsonDoc{
		"request_id": r.RequestID,
		"query_type": r.QueryType,
		"cached":     r.Cached,
		"latency_ms": r.LatencyMS,
		"created_at": created.UTC(),
		"input": bsonDoc{
			"district":       r.Input.District,
			"community_area": r.Input.CommunityArea,
			"day_of_week":    r.Input.DayOfWeek,
			"hour":           r.Input.Hour,
			"week_start":     r.Input.WeekStart,
		},
		"result": bsonDoc{
			"district":       r.Result.District,
			"community_area": r.Result.CommunityArea,
			"day_of_week":    r.Result.DayOfWeek,
			"hour":           r.Result.Hour,
			"risk_score":     r.Result.RiskScore,
			"risk_level":     r.Result.RiskLevel,
			"node_id":        r.Result.NodeID,
		},
		"node": bsonDoc{
			"id":      r.Node.ID,
			"address": r.Node.Address,
			"status":  r.Node.Status,
		},
	}
}

func bsonToPredictionRecord(doc bsonDoc) PredictionRecord {
	rec := PredictionRecord{}
	rec.RequestID, _ = doc["request_id"].(string)
	rec.QueryType, _ = doc["query_type"].(string)
	rec.Cached, _ = doc["cached"].(bool)
	rec.LatencyMS = asInt64(doc["latency_ms"])
	if t, ok := doc["created_at"].(time.Time); ok {
		rec.CreatedAt = t
	}
	if input, ok := doc["input"].(bsonDoc); ok {
		rec.Input = cluster.PredictionInput{
			District:      asInt(input["district"]),
			CommunityArea: asInt(input["community_area"]),
			DayOfWeek:     asInt(input["day_of_week"]),
			Hour:          asInt(input["hour"]),
		}
		rec.Input.WeekStart, _ = input["week_start"].(string)
	}
	if result, ok := doc["result"].(bsonDoc); ok {
		rec.Result = cluster.PredictionResult{
			District:      asInt(result["district"]),
			CommunityArea: asInt(result["community_area"]),
			DayOfWeek:     asInt(result["day_of_week"]),
			Hour:          asInt(result["hour"]),
			RiskScore:     asNumber(result["risk_score"]),
		}
		rec.Result.RiskLevel, _ = result["risk_level"].(string)
		rec.Result.NodeID, _ = result["node_id"].(string)
	}
	if node, ok := doc["node"].(bsonDoc); ok {
		rec.Node = cluster.NodeInfo{}
		rec.Node.ID, _ = node["id"].(string)
		rec.Node.Address, _ = node["address"].(string)
		rec.Node.Status, _ = node["status"].(string)
	}
	return rec
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func asNumber(v any) float64 {
	x, _ := asFloat(v)
	return x
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	default:
		return 0
	}
}
