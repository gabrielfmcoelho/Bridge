package api

// General API info for swag. Regenerate the spec in internal/api/docs with
// `make swagger`; `make swagger-check` fails when the committed spec is stale.
//
//	@title			Bridge API
//	@version		1.0
//	@description	Infrastructure inventory for SEAD-PI: hosts, services, DNS, projects, secrets, Atlas API catalog and integrations.
//	@description	Auth is the session cookie set by POST /api/auth/login. The docs are served same-origin, so "Try it out" sends it automatically once you are logged in.
//	@description	Roles are viewer < editor < admin; each operation's description states the minimum it needs. Assets scoped by entidades answer 404 (never 403) when invisible to the caller.
//	@description	Lists return {data, meta:{page, per_page, total}}; omit per_page to get every row.
//	@BasePath		/

// StatusResponse is the {"status": "..."} acknowledgement returned by deletes,
// restores and similar actions.
type StatusResponse struct {
	Status string `json:"status" example:"deleted"`
}
