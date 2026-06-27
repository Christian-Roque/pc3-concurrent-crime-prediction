package cluster

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// SendTCPRequest envia una solicitud JSON a un nodo ML por TCP y espera una respuesta JSON.
// Se usara desde la API en PC4. En esta fase sirve tambien para pruebas manuales.
func SendTCPRequest(address string, req PredictionRequest, timeout time.Duration) (PredictionResponse, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return PredictionResponse{}, fmt.Errorf("conectar a nodo %s: %w", address, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return PredictionResponse{}, fmt.Errorf("enviar request a nodo %s: %w", address, err)
	}

	var resp PredictionResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return PredictionResponse{}, fmt.Errorf("leer respuesta de nodo %s: %w", address, err)
	}
	if resp.Status == "error" {
		return resp, fmt.Errorf("nodo %s devolvio error: %s", address, resp.Error)
	}
	return resp, nil
}
