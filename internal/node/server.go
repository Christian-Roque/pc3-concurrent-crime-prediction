package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	"pc3-seguridad-ciudadana/internal/cluster"
)

type Server struct {
	Addr      string
	Predictor *Predictor
}

func NewServer(addr string, predictor *Predictor) *Server {
	return &Server{Addr: addr, Predictor: predictor}
}

func (s *Server) ListenAndServe() error {
	if s.Predictor == nil {
		return errors.New("predictor no inicializado")
	}
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("iniciar nodo TCP en %s: %w", s.Addr, err)
	}
	defer ln.Close()
	log.Printf("%s escuchando por TCP en %s | modelo=%s | features=%d", s.Predictor.NodeID, s.Addr, s.Predictor.Model.ModelType, s.Predictor.FeatureCount())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("error aceptando conexion: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	var req cluster.PredictionRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		s.writeResponse(conn, cluster.PredictionResponse{NodeID: s.Predictor.NodeID, Status: "error", Error: "JSON invalido: " + err.Error()})
		return
	}
	cmd := strings.ToLower(strings.TrimSpace(req.Command))
	if cmd == "" {
		cmd = "predict"
	}
	resp := cluster.PredictionResponse{RequestID: req.RequestID, NodeID: s.Predictor.NodeID, Status: "ok"}
	switch cmd {
	case "health":
		// respuesta OK sin calculo
	case "predict":
		result, err := s.Predictor.Predict(req.Input)
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
		} else {
			resp.Result = &result
		}
	case "predict_batch":
		results, err := s.Predictor.PredictBatch(req.Candidates, req.TopN)
		if err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
		} else {
			resp.Results = results
		}
	default:
		resp.Status = "error"
		resp.Error = "comando no soportado: " + req.Command
	}
	s.writeResponse(conn, resp)
}

func (s *Server) writeResponse(conn net.Conn, resp cluster.PredictionResponse) {
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		log.Printf("error enviando respuesta: %v", err)
	}
}
