package models

import "time"

// SSHCustomOperation is an admin-written shell script that editors run on a
// host from its Operações tab. {{name}} placeholders in Script are filled at
// run time; UseSudo runs it under sudo with the host's stored password.
type SSHCustomOperation struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Script      string    `json:"script"`
	UseSudo     bool      `json:"use_sudo"`
	CreatedBy   *int64    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SSHCustomOperationInput is the create / partial-update body: an omitted
// field keeps its value.
type SSHCustomOperationInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Script      *string `json:"script"`
	UseSudo     *bool   `json:"use_sudo"`
}

// SSHCustomOperationRun is the run body: one value per {{placeholder}}.
type SSHCustomOperationRun struct {
	Params map[string]string `json:"params"`
}
