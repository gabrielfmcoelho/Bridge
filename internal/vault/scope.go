package vault

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// revealPermission names the permission a scope's secrets need on top of the
// visibility ACL before their plaintext leaves the vault (reveal, or bundling
// into a share link). API access keys are issued under apis.keys.manage, so
// seeing the API is not enough to read its keys.
var revealPermission = map[models.SecretScope]string{
	models.SecretScopeAPICatalog: "apis.keys.manage",
	models.SecretScopeDatabase:   "databases.users.manage",
}

// revealPermitted reports whether actor may read the plaintext of a secret in
// scope, as far as the scope's extra permission goes (admins always may). A
// host secret the scan found (group "scan-…") needs PermHostSecretsReveal; a
// captured DB-admin login, superuser app login or root SSH key (group
// "scan-admin-…") is admin-only; the host's SSH password and keys keep the
// plain host rule.
func revealPermitted(ctx context.Context, db *sql.DB, actor ActorContext, scope models.SecretScope, groupLabel string) bool {
	if scope == models.SecretScopeHost && strings.HasPrefix(groupLabel, ScanSecretAdminGroupPrefix) {
		// Admin-only tier — checked before the scan- branch it also matches.
		return actor.Role == "admin"
	}
	perm, ok := revealPermission[scope]
	if scope == models.SecretScopeHost && strings.HasPrefix(groupLabel, ScanSecretGroupPrefix) {
		perm, ok = PermHostSecretsReveal, true
	}
	return !ok || store.NewPermissionRepo(db).Has(ctx, actor.Role, perm)
}

// secretVisibleSQL returns the entidade predicate for the secrets table (given
// alias or table name): personal rows pass (decideAccess handles them); shared
// rows inherit from their parent (scope host/service/tool/projeto → the
// parent's asset type); shared avulso ones use their own grants (asset_type
// 'secret', asset_id = secrets.id). "TRUE" with nil args when unscoped.
func secretVisibleSQL(ctx context.Context, alias string) (string, []any) {
	vis, args := store.VisibleExprDyn(ctx,
		"CASE "+alias+".scope WHEN 'avulso' THEN 'secret' WHEN 'projeto' THEN 'project' ELSE "+alias+".scope END",
		"CASE WHEN "+alias+".scope = 'avulso' THEN "+alias+".id ELSE "+alias+".parent_id END")
	if vis == "TRUE" {
		return "TRUE", nil
	}
	return "(" + alias + ".visibility = 'personal' OR " + vis + ")", args
}
