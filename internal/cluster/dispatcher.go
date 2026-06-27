package cluster

import (
	"sort"
	"sync"
	"time"
)

// DispatchTopRequest reparte candidatos entre nodos ML, solicita predicciones por TCP
// y consolida el top N global. Se usa para demostrar procesamiento distribuido
// en PC4: cada nodo calcula scores de una parte distinta del lote.
func DispatchTopRequest(registry *Registry, requestID string, candidates []PredictionInput, topN int, timeout time.Duration) ([]PredictionResult, []NodeInfo, error) {
	if registry == nil {
		return nil, nil, ErrNoRegistry
	}
	nodes := registry.ActiveNodes()
	if len(nodes) == 0 {
		return nil, nil, ErrNoActiveNodes
	}
	if topN <= 0 || topN > len(candidates) {
		topN = len(candidates)
	}
	chunks := SplitCandidates(candidates, len(nodes))

	type partial struct {
		idx     int
		node    NodeInfo
		results []PredictionResult
		err     error
	}

	out := make(chan partial, len(chunks))
	var wg sync.WaitGroup
	used := make([]NodeInfo, 0, len(chunks))
	for i, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}
		node := nodes[i%len(nodes)]
		used = append(used, node)
		wg.Add(1)
		go func(i int, node NodeInfo, chunk []PredictionInput) {
			defer wg.Done()
			req := PredictionRequest{
				RequestID:  requestID,
				Command:    "predict_batch",
				Candidates: chunk,
				TopN:       topN,
			}
			resp, err := SendTCPRequest(node.Address, req, timeout)
			if err != nil {
				out <- partial{idx: i, node: node, err: err}
				return
			}
			out <- partial{idx: i, node: node, results: resp.Results}
		}(i, node, chunk)
	}
	wg.Wait()
	close(out)

	all := make([]PredictionResult, 0, len(candidates))
	for p := range out {
		if p.err != nil {
			return nil, used, p.err
		}
		all = append(all, p.results...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].RiskScore > all[j].RiskScore
	})
	if topN < len(all) {
		all = all[:topN]
	}
	return all, used, nil
}

// SplitCandidates divide una lista de candidatos en chunks lo más balanceados posible.
func SplitCandidates(candidates []PredictionInput, parts int) [][]PredictionInput {
	if parts <= 0 {
		return nil
	}
	chunks := make([][]PredictionInput, parts)
	if len(candidates) == 0 {
		return chunks
	}
	for i, c := range candidates {
		idx := i % parts
		chunks[idx] = append(chunks[idx], c)
	}
	return chunks
}
