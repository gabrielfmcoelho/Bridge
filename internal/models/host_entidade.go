package models

// HostEntidade is the junction between a host and an entidade
// (department/sector). A host can be allocated to multiple entidades; one is
// the main entidade shown on the host card. Persistence lives in
// internal/store.HostEntidadeRepo — this file is the pure data types only.
type HostEntidade struct {
	ID       int64  `json:"id"`
	HostID   int64  `json:"host_id"`
	Entidade string `json:"entidade"`
	IsMain   bool   `json:"is_main"`
}

// HostEntidadeInput is the write-side payload.
type HostEntidadeInput struct {
	Entidade string `json:"entidade"`
	IsMain   bool   `json:"is_main"`
}
