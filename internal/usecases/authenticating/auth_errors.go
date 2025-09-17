package authenticating

import (
	"errors"
	"fmt"
)

// Tipos de erros de autenticação personalizados
var (
	// Erros de autenticação
	ErrInvalidCredentials    = errors.New("credenciais inválidas")
	ErrUserDisabled          = errors.New("usuário desativado")
	ErrUserNotFound          = errors.New("usuário não encontrado")
	ErrUserLocked            = errors.New("usuário bloqueado temporariamente")
	ErrPasswordExpired       = errors.New("senha expirada")
	ErrInvalidToken          = errors.New("token inválido")
	ErrExpiredToken          = errors.New("token expirado")
	ErrInsufficientPrivilege = errors.New("privilégios insuficientes")
	ErrUserAlreadyExists     = errors.New("usuário já existe")

	// Erros de validação
	ErrInvalidRequest      = errors.New("requisição inválida")
	ErrMissingRequiredData = errors.New("dados obrigatórios ausentes")
	ErrInvalidFormat       = errors.New("formato de dados inválido")

	// Erros relacionados a senha
	ErrWeakPassword      = errors.New("senha fraca")
	ErrPasswordMismatch  = errors.New("senhas não conferem")
	ErrSamePassword      = errors.New("nova senha deve ser diferente da atual")
	ErrNoAdminPrivileges = errors.New("apenas administradores podem realizar esta ação")

	// Erros de banco de dados
	ErrDatabaseOperation = errors.New("erro ao realizar operação no banco de dados")
)

// AuthError é um erro com contexto adicional para autenticação
type AuthError struct {
	Err     error  // Erro base
	Code    string // Código de erro para API
	UserID  int    // ID do usuário envolvido (quando aplicável)
	Details string // Detalhes adicionais
}

// Error implementa a interface error
func (e *AuthError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Err.Error(), e.Details)
	}
	return e.Err.Error()
}

// Unwrap retorna o erro subjacente
func (e *AuthError) Unwrap() error {
	return e.Err
}

// IsCredentialsError verifica se o erro está relacionado a credenciais inválidas
func IsCredentialsError(err error) bool {
	return errors.Is(err, ErrInvalidCredentials) ||
		errors.Is(err, ErrUserDisabled) ||
		errors.Is(err, ErrUserLocked) ||
		errors.Is(err, ErrPasswordExpired)
}

// IsAuthorizationError verifica se o erro está relacionado a problemas de autorização
func IsAuthorizationError(err error) bool {
	return errors.Is(err, ErrInsufficientPrivilege) ||
		errors.Is(err, ErrInvalidToken) ||
		errors.Is(err, ErrExpiredToken) ||
		errors.Is(err, ErrNoAdminPrivileges)
}

// NewAuthError cria um novo erro de autenticação
func NewAuthError(baseErr error, code string, details string) *AuthError {
	return &AuthError{
		Err:     baseErr,
		Code:    code,
		Details: details,
	}
}

// NewUserAuthError cria um novo erro de autenticação com contexto de usuário
func NewUserAuthError(baseErr error, code string, userID int, details string) *AuthError {
	return &AuthError{
		Err:     baseErr,
		Code:    code,
		UserID:  userID,
		Details: details,
	}
}
