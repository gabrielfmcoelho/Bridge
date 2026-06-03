package models

import "time"

// Orchestrator describes the container-orchestration tool running on a host
// (one per host). Persistence lives in internal/store.OrchestratorRepo — this
// file is the pure data type only.
type Orchestrator struct {
	ID          int64     `json:"id"`
	HostID      int64     `json:"host_id"`
	Type        string    `json:"type"`
	Version     string    `json:"version"`
	Observacoes string    `json:"observacoes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
