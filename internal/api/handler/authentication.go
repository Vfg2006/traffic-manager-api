package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/authenticating"
	"github.com/vfg2006/traffic-manager-api/pkg/apiErrors"
	"github.com/vfg2006/traffic-manager-api/pkg/middleware"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GeneratePasswordResponse struct {
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func Login(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest

		// Decodificar o corpo da requisição
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Formato de requisição inválido", nil)
			return
		}

		// Tentar realizar o login
		token, err := service.LoginUser(req.Email, req.Password)
		if err != nil {
			handleLoginError(w, err)
			return
		}

		// Sucesso: retornar o token
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token": token,
		})
	}
}

// GetMe retorna as informações do usuário logado
func GetMe(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Obter o token do usuário do contexto
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok {
			apiErrors.WriteError(w, apiErrors.ErrInvalidToken, "Usuário não autenticado", nil)
			return
		}

		// Obter o perfil completo do usuário através do ID presente no token
		user, err := service.GetUserProfile(userClaims.UserID)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao obter dados do usuário", nil)
			return
		}

		// Retornar o usuário como resposta
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(user)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
			return
		}
	}
}

// handleLoginError trata erros específicos de login e retorna a resposta apropriada
func handleLoginError(w http.ResponseWriter, err error) {
	// Tentar fazer cast para AuthError para obter mais detalhes
	var authErr *authenticating.AuthError
	if errors.As(err, &authErr) {
		// Já temos o código no AuthError
		apiErrors.WriteError(w, authErr.Code, authErr.Error(), map[string]any{
			"user_id": authErr.UserID,
		})
		return
	}

	// Verificar tipos específicos de erros
	switch {
	case errors.Is(err, authenticating.ErrInvalidCredentials):
		apiErrors.WriteError(w, apiErrors.ErrInvalidCredentials, "Credenciais inválidas", nil)

	case errors.Is(err, authenticating.ErrUserDisabled):
		apiErrors.WriteError(w, apiErrors.ErrUserDisabled, "Usuário desativado", nil)

	case errors.Is(err, authenticating.ErrUserNotFound):
		apiErrors.WriteError(w, apiErrors.ErrUserNotFound, "Usuário não encontrado", nil)

	case errors.Is(err, authenticating.ErrUserLocked):
		apiErrors.WriteError(w, apiErrors.ErrUserLocked, "Usuário bloqueado temporariamente", nil)

	default:
		// Erro genérico se não conseguirmos identificar especificamente
		apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro interno ao realizar login", nil)
	}
}

// ChangePassword é um handler para permitir que o usuário altere sua própria senha
// Requer que o usuário esteja autenticado
func ChangePassword(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("INIT - ChangePassword")

		// Obter ID do usuário alvo da URL
		targetUserIDStr := httprouter.ParamsFromContext(r.Context()).ByName("id")
		if targetUserIDStr == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "ID do usuário não fornecido", nil)
			return
		}

		targetUserID, err := strconv.Atoi(targetUserIDStr)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidFormat, "ID do usuário inválido", nil)
			return
		}

		// Obter claims do usuário que faz a requisição
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok {
			apiErrors.WriteError(w, apiErrors.ErrInvalidToken, "Não autorizado", nil)
			return
		}

		// Decodificar o corpo da requisição
		var req ChangePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Erro ao decodificar requisição", nil)
			return
		}

		// Verificar se o usuário alvo é o mesmo que o usuário que está fazendo a requisição
		if userClaims.UserID != targetUserID {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Não autorizado a alterar a senha de outro usuário", nil)
			return
		}

		// Alterar a senha
		err = service.ChangePassword(targetUserID, req.CurrentPassword, req.NewPassword)
		if err != nil {
			logrus.Error(err)

			// Verificar o tipo de erro
			errorMsg := err.Error()
			switch {
			case errorMsg == "usuário não encontrado" || errorMsg == "dados do usuário não encontrados":
				apiErrors.WriteError(w, apiErrors.ErrUserNotFound, errorMsg, nil)

			case errorMsg == "senha atual incorreta":
				apiErrors.WriteError(w, apiErrors.ErrInvalidCredentials, errorMsg, nil)

			case strings.Contains(errorMsg, "a senha deve conter"):
				apiErrors.WriteError(w, apiErrors.ErrInvalidFormat, errorMsg, nil)

			default:
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao alterar senha", nil)
			}
			return
		}

		// Retornar resposta de sucesso
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

// GeneratePassword é um handler para gerar uma senha forte para um usuário
// Requer que o usuário que faz a requisição seja um administrador (role_id = 1)
func GeneratePassword(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("INIT - GeneratePassword")

		// Obter claims do usuário que faz a requisição
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok {
			apiErrors.WriteError(w, apiErrors.ErrInvalidToken, "Não autorizado", nil)
			return
		}

		// Obter ID do usuário alvo da URL
		targetUserIDStr := httprouter.ParamsFromContext(r.Context()).ByName("id")
		if targetUserIDStr == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "ID do usuário não fornecido", nil)
			return
		}

		targetUserID, err := strconv.Atoi(targetUserIDStr)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidFormat, "ID do usuário inválido", nil)
			return
		}

		// Gerar nova senha forte
		newPassword, err := service.GenerateStrongPassword(userClaims.UserID, targetUserID)
		if err != nil {
			logrus.Error(err)

			errorMsg := err.Error()
			switch {
			case errorMsg == "apenas administradores podem gerar novas senhas":
				apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, errorMsg, nil)

			case errorMsg == "usuário alvo não encontrado" || errorMsg == "usuário solicitante não encontrado":
				apiErrors.WriteError(w, apiErrors.ErrUserNotFound, errorMsg, nil)

			default:
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao gerar senha", nil)
			}
			return
		}

		// Montar e retornar a resposta
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GeneratePasswordResponse{
			Password: newPassword,
		})
	}
}
