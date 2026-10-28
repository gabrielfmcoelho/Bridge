package gestor

import "fmt"

// Pessoa is a person in Gestor's directory.
type Pessoa struct {
	IDPessoa int64
	Nome     string
	Email    string
	Lotacao  string
}

// PessoaQuery filters BuscarPessoas; empty fields are not sent.
type PessoaQuery struct {
	Nome, Email string
	Limite      int
}

// Projeto is a Gestor project.
type Projeto struct {
	ID                   int64
	Nome                 string
	Concluido, Arquivado bool
}

// Error is a non-2xx answer from Gestor (or from the gateway in front of it).
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("gestor: %d %s", e.Status, e.Message) }
