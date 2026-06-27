package cluster

import "testing"

func TestSplitCandidates(t *testing.T) {
	candidates := []PredictionInput{
		{District: 1}, {District: 2}, {District: 3}, {District: 4}, {District: 5},
	}
	chunks := SplitCandidates(candidates, 3)
	if len(chunks) != 3 {
		t.Fatalf("esperaba 3 chunks, obtuvo %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 2 || len(chunks[2]) != 1 {
		t.Fatalf("distribucion inesperada: %v %v %v", len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
}

func TestActiveNodes(t *testing.T) {
	r := NewRegistry([]NodeInfo{
		{ID: "n1", Address: "n1:9001", Status: "active"},
		{ID: "n2", Address: "n2:9001", Status: "inactive"},
		{ID: "n3", Address: "n3:9001"},
	})
	active := r.ActiveNodes()
	if len(active) != 2 {
		t.Fatalf("esperaba 2 activos, obtuvo %d", len(active))
	}
}
