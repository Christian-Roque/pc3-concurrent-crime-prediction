package cluster

import (
	"errors"
	"sync"
)

var (
	ErrNoRegistry    = errors.New("registro de nodos no configurado")
	ErrNoActiveNodes = errors.New("no hay nodos ML activos")
)

type NodeInfo struct {
	ID      string `json:"node_id"`
	Address string `json:"address"`
	Status  string `json:"status"`
}

type Registry struct {
	mu    sync.Mutex
	nodes []NodeInfo
	next  int
}

func NewRegistry(nodes []NodeInfo) *Registry {
	copyNodes := append([]NodeInfo(nil), nodes...)
	return &Registry{nodes: copyNodes}
}

func (r *Registry) Nodes() []NodeInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]NodeInfo(nil), r.nodes...)
}

func (r *Registry) NextNode() (NodeInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.nodes) == 0 {
		return NodeInfo{}, ErrNoActiveNodes
	}
	for i := 0; i < len(r.nodes); i++ {
		idx := r.next % len(r.nodes)
		r.next++
		if r.nodes[idx].Status == "" || r.nodes[idx].Status == "active" {
			return r.nodes[idx], nil
		}
	}
	return NodeInfo{}, ErrNoActiveNodes
}

// ActiveNodes devuelve una copia de los nodos habilitados para procesamiento distribuido.
func (r *Registry) ActiveNodes() []NodeInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	active := make([]NodeInfo, 0, len(r.nodes))
	for _, n := range r.nodes {
		if n.Status == "" || n.Status == "active" {
			active = append(active, n)
		}
	}
	return active
}
