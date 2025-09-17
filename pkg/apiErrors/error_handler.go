package apiErrors

import (
	"encoding/json"
	"net/http"
)

// Códigos de erro para autenticação
const (
	// Erros de autenticação (1000-1999)
	ErrInvalidCredentials    = "AUTH_001" // Credenciais inválidas
	ErrUserDisabled          = "AUTH_002" // Usuário desativado
	ErrUserNotFound          = "AUTH_003" // Usuário não encontrado
	ErrUserLocked            = "AUTH_004" // Usuário bloqueado temporariamente
	ErrPasswordExpired       = "AUTH_005" // Senha expirada
	ErrInvalidToken          = "AUTH_006" // Token inválido
	ErrExpiredToken          = "AUTH_007" // Token expirado
	ErrInsufficientPrivilege = "AUTH_008" // Privilégios insuficientes
	ErrUserAlreadyExists     = "AUTH_009" // Usuário já existe
	ErrInvalidTokenSSOtica   = "AUTH_010" // Token inválido para a integração SSOtica

	// Erros de validação (2000-2999)
	ErrInvalidRequest      = "VAL_001" // Requisição inválida
	ErrMissingRequiredData = "VAL_002" // Dados obrigatórios ausentes
	ErrInvalidFormat       = "VAL_003" // Formato de dados inválido

	// Erros do servidor (5000-5999)
	ErrInternalServer    = "SRV_001" // Erro interno do servidor
	ErrDatabaseOperation = "SRV_002" // Erro de operação de banco de dados
	ErrExternalService   = "SRV_003" // Erro em serviço externo
	ErrCommunication     = "SRV_004" // Erro de comunicação
)

// Mapeamento de códigos de erro para status HTTP
var httpStatusMap = map[string]int{
	ErrInvalidCredentials:    http.StatusUnauthorized,
	ErrUserDisabled:          http.StatusForbidden,
	ErrUserNotFound:          http.StatusNotFound,
	ErrUserLocked:            http.StatusForbidden,
	ErrPasswordExpired:       http.StatusUnauthorized,
	ErrInvalidToken:          http.StatusUnauthorized,
	ErrExpiredToken:          http.StatusUnauthorized,
	ErrInsufficientPrivilege: http.StatusForbidden,
	ErrInvalidRequest:        http.StatusBadRequest,
	ErrMissingRequiredData:   http.StatusBadRequest,
	ErrInvalidFormat:         http.StatusBadRequest,
	ErrUserAlreadyExists:     http.StatusBadRequest,
	ErrInternalServer:        http.StatusInternalServerError,
	ErrDatabaseOperation:     http.StatusInternalServerError,
	ErrExternalService:       http.StatusBadGateway,
	ErrCommunication:         http.StatusServiceUnavailable,
}

// APIError representa um erro de API padronizado
type APIError struct {
	Code    string `json:"code"`              // Código de erro para o cliente
	Message string `json:"message,omitempty"` // Mensagem descritiva (opcional)
	Details any    `json:"details,omitempty"` // Detalhes adicionais (opcional)
}

// WriteError escreve o erro padronizado para a resposta HTTP
func WriteError(w http.ResponseWriter, code string, message string, details any) {
	status, exists := httpStatusMap[code]
	if !exists {
		status = http.StatusInternalServerError
	}

	apiErr := APIError{
		Code:    code,
		Message: message,
		Details: details,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiErr)
}

// FromError cria um erro de API a partir de um erro Go
// Útil para quando você quer envolver um erro existente em um erro de API
func FromError(err error, code string) APIError {
	if err == nil {
		return APIError{
			Code:    ErrInternalServer,
			Message: "Erro desconhecido",
		}
	}

	return APIError{
		Code:    code,
		Message: err.Error(),
	}
}
