package api

import (
	"fmt"
	"net/http"
	"testing"
)

// secretEnvEnv extends the secretAPIEnv fixture with a service whose ID we
// can reference in env_var payloads.
type secretEnvEnv struct {
	*secretAPIEnv
	serviceID int64
}

func newSecretEnvEnv(t *testing.T) *secretEnvEnv {
	base := newSecretAPIEnv(t)
	var id int64
	if err := base.d.SQL.QueryRow(`INSERT INTO services (nickname) VALUES (?) RETURNING id`, "env-test-svc").Scan(&id); err != nil {
		t.Fatalf("seed service: %v", err)
	}
	return &secretEnvEnv{secretAPIEnv: base, serviceID: id}
}

// ---------------------------------------------------------------------------
// POST /api/secrets/env/bulk
// ---------------------------------------------------------------------------

func TestEnvBulk_HappyPath(t *testing.T) {
	env := newSecretEnvEnv(t)
	body := fmt.Sprintf(`{
		"scope":"service","parent_id":%d,"group_label":"prod","visibility":"shared",
		"vars":[
			{"name":"DB_URL","value":"postgres://...","description":"primary db"},
			{"name":"API_KEY","value":"k-12345"}
		]
	}`, env.serviceID)
	rec := env.do(env.bob, "POST", "/api/secrets/env/bulk", body)
	if rec.Code != 200 {
		t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeMap(t, rec)
	if out["created"].(float64) != 2 || out["updated"].(float64) != 0 {
		t.Errorf("bulk result: %v", out)
	}
}

func TestEnvBulk_ViewerForbidden(t *testing.T) {
	env := newSecretEnvEnv(t)
	body := fmt.Sprintf(`{"scope":"service","parent_id":%d,"group_label":"prod","vars":[{"name":"X","value":"v"}]}`, env.serviceID)
	rec := env.do(env.carol, "POST", "/api/secrets/env/bulk", body)
	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer should be forbidden for shared bulk, got %d", rec.Code)
	}
}

func TestEnvBulk_DuplicateRejected(t *testing.T) {
	env := newSecretEnvEnv(t)
	body := fmt.Sprintf(`{
		"scope":"service","parent_id":%d,"group_label":"prod",
		"vars":[
			{"name":"DB_URL","value":"a"},
			{"name":"OK","value":"b"},
			{"name":"DB_URL","value":"c"}
		]
	}`, env.serviceID)
	rec := env.do(env.bob, "POST", "/api/secrets/env/bulk", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("duplicate name should be 400, got %d body=%s", rec.Code, rec.Body)
	}
}

func TestEnvBulk_LowercaseNameRejected(t *testing.T) {
	env := newSecretEnvEnv(t)
	body := fmt.Sprintf(`{"scope":"service","parent_id":%d,"group_label":"prod","vars":[{"name":"db_url","value":"v"}]}`, env.serviceID)
	rec := env.do(env.bob, "POST", "/api/secrets/env/bulk", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("lowercase name should be 400, got %d", rec.Code)
	}
}

func TestEnvBulk_BadGroupLabelRejected(t *testing.T) {
	env := newSecretEnvEnv(t)
	body := fmt.Sprintf(`{"scope":"service","parent_id":%d,"group_label":"PROD","vars":[{"name":"DB","value":"v"}]}`, env.serviceID)
	rec := env.do(env.bob, "POST", "/api/secrets/env/bulk", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("uppercase group_label should be 400, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// GET /api/secrets/env
// ---------------------------------------------------------------------------

func TestEnvList_GroupedByGroupLabel(t *testing.T) {
	env := newSecretEnvEnv(t)

	// Seed two bundles via the bulk endpoint (less brittle than raw SQL).
	for _, group := range []string{"prod", "staging"} {
		body := fmt.Sprintf(`{
			"scope":"service","parent_id":%d,"group_label":"%s",
			"vars":[
				{"name":"DB_URL","value":"%s-db"},
				{"name":"API_KEY","value":"%s-key"}
			]
		}`, env.serviceID, group, group, group)
		if rec := env.do(env.bob, "POST", "/api/secrets/env/bulk", body); rec.Code != 200 {
			t.Fatalf("seed %s: %d body=%s", group, rec.Code, rec.Body)
		}
	}

	rec := env.do(env.bob, "GET", fmt.Sprintf("/api/secrets/env?scope=service&parent_id=%d", env.serviceID), "")
	if rec.Code != 200 {
		t.Fatalf("list: %d body=%s", rec.Code, rec.Body)
	}
	out := decodeMap(t, rec)
	for _, g := range []string{"prod", "staging"} {
		group, ok := out[g].([]any)
		if !ok {
			t.Fatalf("group %s missing or wrong shape: %v", g, out[g])
		}
		if len(group) != 2 {
			t.Errorf("group %s: expected 2 vars, got %d", g, len(group))
		}
		// Crucially, no `payload` field on any entry. The list endpoint is
		// metadata-only — values come from /api/secrets/{id}/reveal.
		for _, row := range group {
			m := row.(map[string]any)
			if _, has := m["payload"]; has {
				t.Errorf("group %s entry %v should not include payload", g, m["name"])
			}
		}
	}
}

func TestEnvList_FilterByGroupLabel(t *testing.T) {
	env := newSecretEnvEnv(t)
	for _, group := range []string{"prod", "staging"} {
		body := fmt.Sprintf(`{"scope":"service","parent_id":%d,"group_label":"%s","vars":[{"name":"DB_URL","value":"v"}]}`,
			env.serviceID, group)
		if rec := env.do(env.bob, "POST", "/api/secrets/env/bulk", body); rec.Code != 200 {
			t.Fatalf("seed: %d", rec.Code)
		}
	}

	rec := env.do(env.bob, "GET",
		fmt.Sprintf("/api/secrets/env?scope=service&parent_id=%d&group_label=prod", env.serviceID),
		"",
	)
	if rec.Code != 200 {
		t.Fatalf("filtered list: %d", rec.Code)
	}
	out := decodeMap(t, rec)
	if _, has := out["staging"]; has {
		t.Error("group_label=prod filter should exclude staging")
	}
	prod, ok := out["prod"].([]any)
	if !ok || len(prod) != 1 {
		t.Errorf("prod group: %v", out["prod"])
	}
}

// ---------------------------------------------------------------------------
// POST /api/secrets/env/bulk — multi-target (targets[] array)
// ---------------------------------------------------------------------------

func TestEnvBulk_TargetsArray_DualSync(t *testing.T) {
	env := newSecretEnvEnv(t)

	var projectID, svcInProject int64
	if err := env.d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('dual-proj') RETURNING id`).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := env.d.SQL.QueryRow(`INSERT INTO services (nickname, project_id) VALUES ('dual-svc', ?) RETURNING id`, projectID).Scan(&svcInProject); err != nil {
		t.Fatalf("seed service-in-project: %v", err)
	}

	body := fmt.Sprintf(`{
		"targets":[
			{"scope":"projeto","parent_id":%d},
			{"scope":"service","parent_id":%d}
		],
		"group_label":"prod","visibility":"shared",
		"vars":[
			{"name":"DB_URL","value":"postgres://x"},
			{"name":"API_KEY","value":"k-1"}
		]
	}`, projectID, svcInProject)
	rec := env.do(env.bob, "POST", "/api/secrets/env/bulk", body)
	if rec.Code != 200 {
		t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeMap(t, rec)
	// 2 vars x 2 targets.
	if out["created"].(float64) != 4 {
		t.Errorf("expected created=4, got %v", out)
	}
}

func TestEnvBulk_TargetsArray_ServiceNotInProject_Rejected(t *testing.T) {
	env := newSecretEnvEnv(t)

	var projectID, otherProject, svcOther int64
	_ = env.d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p1') RETURNING id`).Scan(&projectID)
	_ = env.d.SQL.QueryRow(`INSERT INTO projects (name) VALUES ('p2') RETURNING id`).Scan(&otherProject)
	if err := env.d.SQL.QueryRow(`INSERT INTO services (nickname, project_id) VALUES ('svc-p2', ?) RETURNING id`, otherProject).Scan(&svcOther); err != nil {
		t.Fatalf("seed svc: %v", err)
	}

	body := fmt.Sprintf(`{
		"targets":[
			{"scope":"projeto","parent_id":%d},
			{"scope":"service","parent_id":%d}
		],
		"group_label":"prod","visibility":"shared",
		"vars":[{"name":"DB_URL","value":"v"}]
	}`, projectID, svcOther)
	rec := env.do(env.bob, "POST", "/api/secrets/env/bulk", body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("service outside project should be 400, got %d body=%s", rec.Code, rec.Body)
	}
}
