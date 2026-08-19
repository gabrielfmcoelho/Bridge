package models

import "time"

// Entidade is an organisational unit (órgão/setor) in a strict tree. Users
// belong to N entidades; assets are created by one and may be the
// responsibility of N. Visibility flows UP the tree: an entidade sees every
// asset granted to itself or any descendant. Persistence lives in
// internal/store.EntidadeRepo.
type Entidade struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	ParentID    *int64    `json:"parent_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserEntidade is one membership row as returned on /api/auth/me and
// /api/users — the entidade plus whether it is the user's primary one (the
// default creator entidade when the user creates an asset).
type UserEntidade struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	IsPrimary bool   `json:"is_primary"`
}

// AssetGrantsInput is the write-side payload for an asset's entidade grants.
// It is embedded in create/update request bodies of every root asset and is
// the body of PUT /api/assets/{type}/{id}/entidades. Pointer fields mean
// "present in the request": an update that omits all three leaves the
// existing grants untouched.
type AssetGrantsInput struct {
	CreatorEntidadeID      *int64   `json:"creator_entidade_id,omitempty"`
	ResponsibleEntidadeIDs *[]int64 `json:"responsible_entidade_ids,omitempty"`
	IsGlobal               *bool    `json:"is_global,omitempty"`
}

// Present reports whether any grant field was supplied.
func (in AssetGrantsInput) Present() bool {
	return in.CreatorEntidadeID != nil || in.ResponsibleEntidadeIDs != nil || in.IsGlobal != nil
}

// AssetGrants is the resolved, read-side view of an asset's grants.
type AssetGrants struct {
	CreatorEntidadeID      *int64  `json:"creator_entidade_id"`
	ResponsibleEntidadeIDs []int64 `json:"responsible_entidade_ids"`
	IsGlobal               bool    `json:"is_global"`
}
