package vault

import (
	"context"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

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
