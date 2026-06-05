package api

import (
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/vault"
)

// Deps is the dependency-injection container: shared singletons (DB, the
// secrets repo, and domain services) constructed once at startup and handed to
// the handlers that need them. It replaces the previous pattern of every
// handler holding a raw *database.DB and building repos inline on each call.
//
// It grows one field per migrated entity as Phase 2 proceeds; handlers that
// still hold *database.DB use Deps.DB until they are converted.
type Deps struct {
	DB *database.DB

	// Secrets is the unified vault repository (per-row ACL + crypto + audit).
	Secrets *vault.SecretRepo

	// Domain services (Phase 2). One field per service as entities migrate.
	DNS     *service.DNSService
	Project *service.ProjectService
}

// NewDeps wires the container from a database handle. Construction order:
// db → repos → services → (handlers, in NewRouter).
func NewDeps(db *database.DB) *Deps {
	return &Deps{
		DB:      db,
		Secrets: vault.NewSecretRepo(db),
		DNS:     service.NewDNSService(db.SQL),
		Project: service.NewProjectService(db.SQL),
	}
}
