package api

import (
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/auth/providers"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	glpiclient "github.com/gabrielfmcoelho/ssh-config-manager/internal/integrations/glpi"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// App is the dependency-injection container. newApp wires the whole graph once
// at startup — db → encryptor → auth registry → repos → services → handlers —
// replacing the per-handler struct soup that used to live inline in NewRouter.
// Route registration (NewRouter) consumes the constructed handlers from here.
type App struct {
	db       *database.DB
	deps     *Deps
	registry *auth.ProviderRegistry

	// HTTP handler groups (one field per group; constructed in newApp).
	auth                *authHandlers
	host                *hostHandlers
	dns                 *dnsHandlers
	project             *projectHandlers
	service             *serviceHandlers
	orchestrator        *orchestratorHandlers
	ssh                 *sshHandlers
	graph               *graphHandlers
	dashboard           *dashboardHandlers
	enum                *enumHandlers
	settings            *settingsHandlers
	issue               *issueHandlers
	release             *releaseHandlers
	tool                *toolHandlers
	contact             *contactHandlers
	sshKey              *sshKeyHandlers
	importH             *importHandlers
	backup              *backupHandlers
	globalIssue         *globalIssueHandlers
	hostAlert           *hostAlertHandlers
	hostChamado         *hostChamadoHandlers
	integrationSettings *integrationSettingsHandlers
	oauth               *oauthHandlers
	gitlab              *gitlabHandlers
	projectGitlab       *projectGitLabHandlers
	ai                  *aiHandlers
	coolify             *coolifyHandlers
	grafana             *grafanaHandlers
	grafanaWebhook      *grafanaWebhookHandlers
	outline             *outlineHandlers
	glpi                *glpiHandlers
}

// newApp constructs the container: build the auth provider registry, the shared
// DI singletons (Deps: db + secrets repo + domain services), then every handler
// group. Construction order is db → registry → deps → handlers.
func newApp(db *database.DB, configPath string) *App {
	registry := auth.NewProviderRegistry()
	registry.Register(providers.NewLocalProvider(db.SQL))
	registry.Register(providers.NewLDAPProvider(db.SQL, db.Encryptor))
	registry.Register(providers.NewKeycloakProvider(db.SQL, db.Encryptor))
	registry.Register(providers.NewGitLabProvider(db.SQL, db.Encryptor))

	deps := NewDeps(db)

	ah := &authHandlers{db: db, registry: registry}

	return &App{
		db:                  db,
		deps:                deps,
		registry:            registry,
		auth:                ah,
		host:                &hostHandlers{host: deps.Host, db: db},
		dns:                 &dnsHandlers{dns: deps.DNS},
		project:             &projectHandlers{project: deps.Project},
		service:             &serviceHandlers{service: deps.Service, db: db},
		orchestrator:        &orchestratorHandlers{db: db},
		ssh:                 &sshHandlers{db: db, configPath: configPath},
		graph:               &graphHandlers{db: db},
		dashboard:           &dashboardHandlers{db: db},
		enum:                &enumHandlers{enum: store.NewEnumOptionRepo(db.SQL)},
		settings:            &settingsHandlers{db: db},
		issue:               &issueHandlers{db: db},
		release:             &releaseHandlers{db: db},
		tool:                &toolHandlers{db: db},
		contact:             &contactHandlers{contacts: store.NewContactRepo(db.SQL)},
		sshKey:              &sshKeyHandlers{db: db},
		importH:             &importHandlers{db: db},
		backup:              &backupHandlers{db: db},
		globalIssue:         &globalIssueHandlers{db: db},
		hostAlert:           &hostAlertHandlers{db: db},
		hostChamado:         &hostChamadoHandlers{db: db},
		integrationSettings: &integrationSettingsHandlers{db: db, registry: registry},
		oauth:               &oauthHandlers{db: db, registry: registry, ah: ah},
		gitlab:              &gitlabHandlers{db: db},
		projectGitlab:       &projectGitLabHandlers{db: db},
		ai:                  &aiHandlers{db: db},
		coolify:             &coolifyHandlers{db: db},
		grafana:             &grafanaHandlers{db: db},
		grafanaWebhook:      &grafanaWebhookHandlers{db: db},
		outline:             &outlineHandlers{db: db},
		glpi:                &glpiHandlers{db: db, cache: glpiclient.NewSessionCache(30 * time.Minute)},
	}
}
