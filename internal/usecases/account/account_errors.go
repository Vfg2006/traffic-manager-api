package account

import (
	"errors"
	"fmt"
)

// Erros específicos para o contexto de contas
var (
	// Erros de validação
	ErrAccountIDRequired     = errors.New("account ID is required")
	ErrAccountNotFound       = errors.New("account not found")
	ErrInvalidToken          = errors.New("invalid token")
	ErrTokenValidationFailed = errors.New("token validation failed")

	// Erros de serviços externos
	ErrSSOticaConnection  = errors.New("error connecting to SSOtica")
	ErrRenderSecretUpdate = errors.New("error updating secret on render")
	ErrMetaIntegration    = errors.New("error fetching accounts from Meta")

	// Erros de banco de dados
	ErrDatabaseOperation = errors.New("database operation error")
	ErrUpdateAccount     = errors.New("error updating account")
	ErrFetchAccounts     = errors.New("error fetching accounts from database")

	// Erros de sincronização
	ErrGenerateID = errors.New("error generating UUID")
)

// AccountError é um erro com contexto adicional para contas
type AccountError struct {
	Err       error  // Erro base
	Code      string // Código de erro para API
	AccountID string // ID da conta envolvida (quando aplicável)
	Details   string // Detalhes adicionais
}

// Error implementa a interface error
func (e *AccountError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Err.Error(), e.Details)
	}
	return e.Err.Error()
}

// Unwrap retorna o erro subjacente
func (e *AccountError) Unwrap() error {
	return e.Err
}

// NewAccountError cria um novo AccountError
func NewAccountError(err error, code string, details string) *AccountError {
	return &AccountError{
		Err:     err,
		Code:    code,
		Details: details,
	}
}

// NewAccountErrorWithID cria um novo AccountError com ID da conta
func NewAccountErrorWithID(err error, code string, accountID string, details string) *AccountError {
	return &AccountError{
		Err:       err,
		Code:      code,
		AccountID: accountID,
		Details:   details,
	}
}
