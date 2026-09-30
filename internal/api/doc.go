package api

// General API info for swag. Regenerate the spec in internal/api/docs with
// `make swagger`; `make swagger-check` fails when the committed spec is stale.
// It is served as ReDoc at /docs (docs_handlers.go), with token scopes and
// security schemes added at serve time.
//
//	@title			Bridge API
//	@version		1.0
//	@description	Infrastructure inventory for SEAD-PI: hosts, services, DNS, projects, secrets, Atlas API catalog and integrations.
//	@description	Auth: the browser session cookie (POST /api/auth/login), or an API token sent as "Authorization: Bearer brg_…" (Configurações → Tokens de API; personal or for a service account). A token acts as its owner — same role, permissions and entidades — narrowed to its scopes: each operation below names the scope it needs (x-bridge-scope); a token without it gets 403. GET /api/auth/tokens/scopes lists them.
//	@description	Roles are viewer < editor < admin; each operation's description states the minimum it needs. Assets scoped by entidades answer 404 (never 403) when invisible to the caller.
//	@description	Lists return {data, meta:{page, per_page, total}}; omit per_page to get every row.
//	@BasePath		/

// StatusResponse is the {"status": "..."} acknowledgement returned by deletes,
// restores and similar actions.
type StatusResponse struct {
	Status string `json:"status" example:"deleted"`
}
