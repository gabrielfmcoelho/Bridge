package models

import "time"

// GitLabAutor is one GitLab commit e-mail (lowercase, unique), with the scan
// counts, the scan's Gestor-user suggestion and the human-made link. A
// suggestion is never a link: only a person sets the gestor_* fields.
type GitLabAutor struct {
	ID               int64      `json:"id"`
	Email            string     `json:"email"`
	NomeGitLab       string     `json:"nome_gitlab"`
	Commits          int        `json:"commits"`
	Repos            int        `json:"repos"`
	VistoEm          *time.Time `json:"visto_em"`
	SugestaoPessoaID *int64     `json:"sugestao_pessoa_id"`
	SugestaoNome     string     `json:"sugestao_nome"`
	SugestaoMotivo   string     `json:"sugestao_motivo"` // "email" | "nome" | ""
	GestorPessoaID   *int64     `json:"gestor_pessoa_id"`
	GestorNome       string     `json:"gestor_nome"`
	GestorEmail      string     `json:"gestor_email"`
	VinculadoPor     *int64     `json:"vinculado_por"`
	VinculadoEm      *time.Time `json:"vinculado_em"`
	Ignorado         bool       `json:"ignorado"`
	CriadoEm         time.Time  `json:"criado_em"`
	AtualizadoEm     time.Time  `json:"atualizado_em"`
}
