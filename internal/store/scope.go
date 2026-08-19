package store

import (
	"context"
	"fmt"
)

// AssetType names a table whose rows carry their own entidade grants in
// asset_entidades. Values match the CHECK constraint in migration v82 and,
// for issues, the existing issues.entity_type vocabulary (project / service /
// host / dns) so the polymorphic predicate can be composed on that column.
type AssetType string

const (
	AssetHost       AssetType = "host"
	AssetDNS        AssetType = "dns"
	AssetService    AssetType = "service"
	AssetProject    AssetType = "project"
	AssetContact    AssetType = "contact"
	AssetTool       AssetType = "tool"
	AssetSSHKey     AssetType = "ssh_key"
	AssetAPICatalog AssetType = "api_catalog"
	AssetSecret     AssetType = "secret"
)

// AssetSpec describes how to reach a root asset's rows for the cross-cutting
// operations that don't live in its own repo (unassigned triage, bulk
// assign). Same idiom as parentRegistry in cascade.go: registering the type
// is the one required step.
type AssetSpec struct {
	Table      string // trusted in-code constant
	NameColumn string // human label column for triage lists
	SoftDelete bool   // table has deleted_at
	Extra      string // extra SQL predicate narrowing which rows are grantable
}

var assetRegistry = map[AssetType]AssetSpec{
	AssetHost:       {Table: "hosts", NameColumn: "nickname", SoftDelete: true},
	AssetDNS:        {Table: "dns_records", NameColumn: "domain"},
	AssetService:    {Table: "services", NameColumn: "nickname", SoftDelete: true},
	AssetProject:    {Table: "projects", NameColumn: "name", SoftDelete: true},
	AssetContact:    {Table: "contacts", NameColumn: "name"},
	AssetTool:       {Table: "external_tools", NameColumn: "name", SoftDelete: true},
	AssetSSHKey:     {Table: "ssh_keys", NameColumn: "name"},
	AssetAPICatalog: {Table: "api_catalog", NameColumn: "name", SoftDelete: true},
	// Only standalone shared secrets carry their own grants; parented secrets
	// inherit from host/service/project/tool and personal ones are owner-only.
	AssetSecret: {Table: "secrets", NameColumn: "name", SoftDelete: true, Extra: "scope = 'avulso' AND visibility = 'shared'"},
}

// AssetOf returns the registered spec for an asset type.
func AssetOf(t AssetType) (AssetSpec, bool) {
	s, ok := assetRegistry[t]
	return s, ok
}

// Scope is the caller's visibility: Admin bypasses everything; otherwise
// EntidadeIDs is the user's visible set — every entidade they belong to plus
// all descendants ("above sees all that below sees"). PrimaryEntidadeID is
// the default creator for new assets (0 when the user has no primary).
type Scope struct {
	Admin             bool
	EntidadeIDs       []int64
	PrimaryEntidadeID int64
}

// Allows reports whether entidade id is inside the visible set (or Admin).
func (s Scope) Allows(id int64) bool {
	if s.Admin {
		return true
	}
	for _, e := range s.EntidadeIDs {
		if e == id {
			return true
		}
	}
	return false
}

type scopeKey struct{}

// WithScope attaches the caller's Scope to ctx. auth.RequireAuth does this
// once per request; tests do it directly.
func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

// WithSystemScope marks ctx as unscoped (background jobs, startup migrations).
func WithSystemScope(ctx context.Context) context.Context {
	return WithScope(ctx, Scope{Admin: true})
}

// ScopeFrom returns the Scope on ctx. ok=false means none was attached.
//
// ponytail: scope rides in ctx; an absent scope is treated as unscoped so
// background jobs and repo tests keep working untouched. RequireAuth is the
// single place that sets it for HTTP. Flip to deny-by-default here if a repo
// is ever reached by an authenticated path that skipped RequireAuth.
func ScopeFrom(ctx context.Context) (Scope, bool) {
	s, ok := ctx.Value(scopeKey{}).(Scope)
	return s, ok
}

// unscoped reports whether the predicate can be skipped entirely.
func unscoped(ctx context.Context) (Scope, bool) {
	s, ok := ScopeFrom(ctx)
	return s, !ok || s.Admin
}

// VisibleExpr returns a SQL boolean expression (and its args) that is true
// when the asset identified by (assetType, <idExpr>) is visible to the
// caller: a grant row exists that is global or points at an entidade in the
// caller's visible set. Admin / system / absent scope ⇒ "TRUE", no args.
// Append it to an existing where-builder:
//
//	vis, vargs := store.VisibleExpr(ctx, store.AssetHost, "hosts.id")
//	where = append(where, vis); args = append(args, vargs...)
func VisibleExpr(ctx context.Context, assetType AssetType, idExpr string) (string, []any) {
	return VisibleExprDyn(ctx, "?", idExpr, string(assetType))
}

// VisibleExprDyn is VisibleExpr for polymorphic tables where the asset type
// is itself a SQL expression (issues.entity_type, a CASE over secrets.scope).
// typeArgs are the placeholders consumed by typeExpr, if any.
func VisibleExprDyn(ctx context.Context, typeExpr, idExpr string, typeArgs ...any) (string, []any) {
	s, skip := unscoped(ctx)
	if skip {
		return "TRUE", nil
	}
	ids := s.EntidadeIDs
	if ids == nil {
		ids = []int64{} // nil would bind as NULL and `= ANY(NULL)` is never true but confusing
	}
	args := append(append([]any{}, typeArgs...), ids)
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM asset_entidades ae WHERE ae.asset_type = %s AND ae.asset_id = %s AND (ae.relation = 'global' OR ae.entidade_id = ANY(?)))`,
		typeExpr, idExpr), args
}

// CanSee evaluates the visibility predicate for a single asset id. For
// handlers that hold only an id (issues' polymorphic parent, project
// sub-resources without a parent load). Does NOT check the row exists.
func CanSee(ctx context.Context, q Queryer, assetType AssetType, id int64) (bool, error) {
	if _, skip := unscoped(ctx); skip {
		return true, nil
	}
	vis, args := VisibleExprDyn(ctx, "?", "?", string(assetType), id)
	var ok bool
	err := q.QueryRowContext(ctx, `SELECT `+vis, args...).Scan(&ok)
	return ok, err
}
