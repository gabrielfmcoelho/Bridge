package apicatalog

import (
	"encoding/json"
	"testing"
)

func TestFilter_All_Unchanged(t *testing.T) {
	ps, err := Parse([]byte(openapi30YAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := Filter(ps.SpecJSON, Selector{Mode: SelectorAll})
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if string(out) != string(ps.SpecJSON) {
		t.Error("mode=all should return the spec unchanged")
	}
}

func TestFilter_Operations(t *testing.T) {
	ps, err := Parse([]byte(openapi30YAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := Filter(ps.SpecJSON, Selector{Mode: SelectorOperations, OpKeys: []string{"listOrders"}})
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	paths := pathsOf(t, out)
	if _, ok := paths["/orders"]; !ok {
		t.Error("expected /orders to be kept")
	}
	if _, ok := paths["/orders/{id}"]; ok {
		t.Error("expected /orders/{id} to be dropped (op not selected)")
	}
}

func TestFilter_Tags(t *testing.T) {
	ps, err := Parse([]byte(openapi30YAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Both operations carry the "orders" tag → both paths kept.
	out, err := Filter(ps.SpecJSON, Selector{Mode: SelectorTags, Tags: []string{"orders"}})
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	paths := pathsOf(t, out)
	if len(paths) != 2 {
		t.Errorf("expected 2 paths kept, got %d: %v", len(paths), paths)
	}

	// A tag nothing carries → no paths kept.
	out, err = Filter(ps.SpecJSON, Selector{Mode: SelectorTags, Tags: []string{"nope"}})
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(pathsOf(t, out)) != 0 {
		t.Error("expected 0 paths for unmatched tag")
	}
}

func pathsOf(t *testing.T, specJSON []byte) map[string]interface{} {
	t.Helper()
	var doc map[string]interface{}
	if err := json.Unmarshal(specJSON, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p, _ := doc["paths"].(map[string]interface{})
	return p
}
