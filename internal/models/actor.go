package models

// ActorContext identifies the user making a repository call. It carries the
// fields needed for ACL decisions (in the vault/secrets layer) and audit
// attribution (in the store cascade). It lives in models — the neutral
// data-types package both store and vault import — so neither layer has to
// depend on the other just to name the acting principal.
//
// The handler layer constructs this from the auth middleware (see
// internal/api/actor.go).
type ActorContext struct {
	UserID int64
	Role   string // "viewer" | "editor" | "admin"
}
