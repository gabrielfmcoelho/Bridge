package models

import "time"

// Project is a pure data type (DTO). All SQL for projects and the
// project_host_links junction lives in internal/store.ProjectRepo; enrichment
// and orchestration live in internal/service.ProjectService (R2 refactor).
type Project struct {
	ID                           int64     `json:"id"`
	Name                         string    `json:"name"`
	Description                  string    `json:"description"`
	Situacao                     string    `json:"situacao"`
	SetorResponsavel             string    `json:"setor_responsavel"`
	Responsavel                  string    `json:"responsavel"`
	TemEmpresaExternaResponsavel bool      `json:"tem_empresa_externa_responsavel"`
	ContatoEmpresaResponsavel    string    `json:"contato_empresa_responsavel"`
	IsDirectlyManaged            bool      `json:"is_directly_managed"`
	IsResponsible                bool      `json:"is_responsible"`
	GitlabURL                    string    `json:"gitlab_url"`
	DocumentationURL             string    `json:"documentation_url"`
	OutlineCollectionID          string    `json:"outline_collection_id"`
	GlpiTokenID                  *int64    `json:"glpi_token_id"`
	GlpiEntityID                 int       `json:"glpi_entity_id"`
	GlpiCategoryID               int       `json:"glpi_category_id"`
	CreatedAt                    time.Time `json:"created_at"`
	UpdatedAt                    time.Time `json:"updated_at"`
}
