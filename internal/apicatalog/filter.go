package apicatalog

import (
	"encoding/json"
	"strings"
)

// Selector mode constants for partial share bundles (Phase D).
const (
	SelectorAll        = "all"
	SelectorOperations = "operations"
	SelectorTags       = "tags"
)

// Selector describes which slice of a spec a share bundle exposes.
//
//	{mode:"all"}                                  -> whole spec
//	{mode:"operations", op_keys:["getUser",...]}  -> only those operations
//	{mode:"tags", tags:["billing","users"]}       -> only ops carrying a tag
type Selector struct {
	Mode   string   `json:"mode"`
	OpKeys []string `json:"op_keys,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// httpMethods are the path-item keys that hold operations; everything else at
// the path level (parameters, $ref, summary, description, servers) is shared
// metadata that is preserved verbatim whenever a path keeps at least one op.
var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// Filter reduces specJSON to the operations selected by sel. v1 keeps all
// components/definitions untouched (correctness over minimality — pruning
// unreachable $refs is a documented follow-up) and only trims `paths`.
// mode "all"/"" returns the input unchanged.
func Filter(specJSON []byte, sel Selector) ([]byte, error) {
	if sel.Mode == "" || sel.Mode == SelectorAll {
		return specJSON, nil
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(specJSON, &doc); err != nil {
		return nil, err
	}
	pathsRaw, ok := doc["paths"].(map[string]interface{})
	if !ok {
		return specJSON, nil
	}

	opKeep := toSet(sel.OpKeys)
	tagKeep := toSet(sel.Tags)
	newPaths := make(map[string]interface{})

	for path, piRaw := range pathsRaw {
		pi, ok := piRaw.(map[string]interface{})
		if !ok {
			continue
		}
		kept := make(map[string]interface{})
		for _, method := range httpMethods {
			opRaw, present := pi[method]
			if !present {
				continue
			}
			op, _ := opRaw.(map[string]interface{})
			if keepOperation(method, path, op, sel.Mode, opKeep, tagKeep) {
				kept[method] = opRaw
			}
		}
		if len(kept) == 0 {
			continue
		}
		// Preserve path-level shared fields (parameters, $ref, etc.).
		for k, val := range pi {
			if !isHTTPMethod(k) {
				kept[k] = val
			}
		}
		newPaths[path] = kept
	}

	doc["paths"] = newPaths
	return json.Marshal(doc)
}

func keepOperation(method, path string, op map[string]interface{}, mode string, opKeep, tagKeep map[string]bool) bool {
	switch mode {
	case SelectorOperations:
		operationID, _ := op["operationId"].(string)
		return opKeep[OperationKey(method, path, operationID)]
	case SelectorTags:
		tags, _ := op["tags"].([]interface{})
		for _, t := range tags {
			if s, ok := t.(string); ok && tagKeep[s] {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func isHTTPMethod(k string) bool {
	lk := strings.ToLower(k)
	for _, m := range httpMethods {
		if lk == m {
			return true
		}
	}
	return false
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, it := range items {
		set[it] = true
	}
	return set
}
