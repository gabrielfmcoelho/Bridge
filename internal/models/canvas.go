package models

import (
	"encoding/json"
	"time"
)

// Canvas is an entidade's idea board. Content is the board's xyflow JSON
// (nodes + edges), opaque to the server; it's omitted from list rows.
// Persistence lives in internal/store.CanvasRepo.
type Canvas struct {
	ID           int64           `json:"id"`
	EntidadeID   int64           `json:"entidade_id"`
	EntidadeName string          `json:"entidade_name"`
	Title        string          `json:"title"`
	Content      json.RawMessage `json:"content,omitempty" swaggertype:"object"`
	Version      int             `json:"version"`
	CreatedBy    *int64          `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
