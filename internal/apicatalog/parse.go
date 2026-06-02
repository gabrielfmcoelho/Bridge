// Package apicatalog ingests OpenAPI/Swagger specifications for the Atlas
// REST API catalog (Phase A). It has three responsibilities:
//
//   - parse.go:  detect Swagger 2.0 vs OpenAPI 3.x, normalize the spec to
//     canonical JSON, and extract a flat operation index for search +
//     stable partial-share selection.
//   - filter.go: reduce a spec to a selected subset of operations/tags
//     (used by Phase D share bundles); keeps all components in v1.
//   - fetch.go:  SSRF-guarded HTTP fetch of a spec from a URL.
//
// libopenapi handles version detection and model building for both 2.0 and
// 3.x; canonical-JSON normalization is done generically (YAML is a superset
// of JSON) so it never depends on libopenapi's renderer quirks across the
// two spec versions.
package apicatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/utils"
	yaml "gopkg.in/yaml.v3"
)

// Operation is a single endpoint extracted from a spec. OpKey is the stable
// selector key (operationId when present, else "METHOD path") used by
// partial share bundles so a selection survives spec re-imports.
type Operation struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operation_id,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	OpKey       string   `json:"op_key"`
}

// ParsedSpec is the result of Parse: the canonical JSON, version metadata,
// the "open externally" base URL, and the flat operation index.
type ParsedSpec struct {
	SpecJSON     []byte
	SpecHash     string
	SpecVersion  string // e.g. "swagger-2.0" | "openapi-3.0.1"
	Title        string
	VersionLabel string
	ExternalURL  string
	Operations   []Operation
}

// OperationKey returns the stable selector key for an operation. operationId
// wins when present; otherwise the method+path pair (which is unique within a
// spec) is used. Both parse.go and filter.go route through this so selectors
// computed at share time match the index built at import time.
func OperationKey(method, path, operationID string) string {
	if operationID != "" {
		return operationID
	}
	return strings.ToUpper(method) + " " + path
}

// Parse detects the spec version, normalizes it to canonical JSON, and
// extracts metadata + the operation index. Returns an error for empty or
// unparseable input (the import handler maps that to a 400).
func Parse(raw []byte) (*ParsedSpec, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("empty specification")
	}

	specJSON, err := normalizeToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("normalize spec to json: %w", err)
	}

	doc, err := libopenapi.NewDocument(raw)
	if err != nil {
		return nil, fmt.Errorf("parse openapi document: %w", err)
	}
	info := doc.GetSpecInfo()

	sum := sha256.Sum256(specJSON)
	ps := &ParsedSpec{SpecJSON: specJSON, SpecHash: hex.EncodeToString(sum[:])}

	if info != nil && info.SpecType == utils.OpenApi2 {
		m, err := doc.BuildV2Model()
		if m == nil {
			return nil, fmt.Errorf("build swagger 2.0 model: %w", err)
		}
		ps.SpecVersion = "swagger-" + info.Version
		sw := m.Model
		if sw.Info != nil {
			ps.Title = sw.Info.Title
			ps.VersionLabel = sw.Info.Version
		}
		ps.ExternalURL = v2ExternalURL(&sw)
		ps.Operations = collectV2(&sw)
		return ps, nil
	}

	// Default: OpenAPI 3.x.
	m, err := doc.BuildV3Model()
	if m == nil {
		return nil, fmt.Errorf("build openapi 3 model: %w", err)
	}
	if info != nil {
		ps.SpecVersion = "openapi-" + info.Version
	} else {
		ps.SpecVersion = "openapi"
	}
	d := m.Model
	if d.Info != nil {
		ps.Title = d.Info.Title
		ps.VersionLabel = d.Info.Version
	}
	for _, s := range d.Servers {
		if s != nil && strings.TrimSpace(s.URL) != "" {
			ps.ExternalURL = s.URL
			break
		}
	}
	ps.Operations = collectV3(&d)
	return ps, nil
}

func collectV3(d *v3high.Document) []Operation {
	var ops []Operation
	if d.Paths == nil || d.Paths.PathItems == nil {
		return ops
	}
	for path, item := range d.Paths.PathItems.FromOldest() {
		if item == nil {
			continue
		}
		for method, op := range item.GetOperations().FromOldest() {
			if op == nil {
				continue
			}
			ops = append(ops, Operation{
				Method:      strings.ToUpper(method),
				Path:        path,
				OperationID: op.OperationId,
				Summary:     op.Summary,
				Description: op.Description,
				Tags:        op.Tags,
				OpKey:       OperationKey(method, path, op.OperationId),
			})
		}
	}
	return ops
}

func collectV2(sw *v2high.Swagger) []Operation {
	var ops []Operation
	if sw.Paths == nil || sw.Paths.PathItems == nil {
		return ops
	}
	for path, item := range sw.Paths.PathItems.FromOldest() {
		if item == nil {
			continue
		}
		for method, op := range item.GetOperations().FromOldest() {
			if op == nil {
				continue
			}
			ops = append(ops, Operation{
				Method:      strings.ToUpper(method),
				Path:        path,
				OperationID: op.OperationId,
				Summary:     op.Summary,
				Description: op.Description,
				Tags:        op.Tags,
				OpKey:       OperationKey(method, path, op.OperationId),
			})
		}
	}
	return ops
}

// v2ExternalURL reconstructs a base URL from a Swagger 2.0 host/basePath/
// schemes triad for the "open externally" affordance. Returns "" when host
// is absent (spec served relative to wherever it was fetched from).
func v2ExternalURL(sw *v2high.Swagger) string {
	if sw.Host == "" {
		return ""
	}
	scheme := "https"
	if len(sw.Schemes) > 0 && sw.Schemes[0] != "" {
		scheme = sw.Schemes[0]
	}
	return scheme + "://" + sw.Host + sw.BasePath
}

// normalizeToJSON converts an OpenAPI/Swagger spec (JSON or YAML) into
// canonical JSON. JSON is a subset of YAML, so a single YAML decode handles
// both; normalizeYAMLValue then coerces any non-string-keyed maps so the
// result is json.Marshal-safe.
func normalizeToJSON(raw []byte) ([]byte, error) {
	var v interface{}
	if err := yaml.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("specification decoded to empty document")
	}
	return json.Marshal(normalizeYAMLValue(v))
}

func normalizeYAMLValue(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{}, len(x))
		for k, val := range x {
			m[fmt.Sprint(k)] = normalizeYAMLValue(val)
		}
		return m
	case map[string]interface{}:
		for k, val := range x {
			x[k] = normalizeYAMLValue(val)
		}
		return x
	case []interface{}:
		for i, val := range x {
			x[i] = normalizeYAMLValue(val)
		}
		return x
	default:
		return v
	}
}
