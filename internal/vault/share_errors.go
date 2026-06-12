package vault

import "errors"

// Public-link error taxonomy. Originally defined for secret_share_links (R3
// retired that table); now shared by the bundle redemption path. The public
// redeem handler collapses all of these to 404 externally so a guest can't
// probe link state, but internal callers and tests switch on the cause.
var (
	// ErrShareTargetNotPersonal is returned when a share targets a secret with
	// visibility != 'personal' (spec §5.3 / D7): shared secrets are already
	// RBAC-accessible internally, so external sharing has no use case.
	ErrShareTargetNotPersonal = errors.New("share links may only be created against personal secrets")

	ErrShareLinkExpired       = errors.New("share link expired")
	ErrShareLinkRevoked       = errors.New("share link revoked")
	ErrShareLinkMaxViews      = errors.New("share link view limit reached")
	ErrShareLinkPassphraseBad = errors.New("share link passphrase incorrect")
	ErrShareLinkNotFound      = errors.New("share link not found")
)
