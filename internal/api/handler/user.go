package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/authenticating"
	"github.com/vfg2006/traffic-manager-api/pkg/apiErrors"
	"github.com/vfg2006/traffic-manager-api/pkg/middleware"
)

// GetUser retorna informações do usuário por ID
func GetUser(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extrair ID do usuário da URL
		idStr := httprouter.ParamsFromContext(r.Context()).ByName("id")
		if idStr == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "ID do usuário não fornecido", nil)
			return
		}

		id, err := strconv.Atoi(idStr)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidFormat, "ID do usuário inválido", nil)
			return
		}

		// Buscar informações do usuário
		user, err := service.GetUserProfile(id)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao buscar usuário", nil)
			return
		}

		if user == nil {
			apiErrors.WriteError(w, apiErrors.ErrUserNotFound, "Usuário não encontrado", nil)
			return
		}

		// Enviar resposta
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(user)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
			return
		}
	}
}

// CreateUser cria um novo usuário
func CreateUser(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("INIT - CreateUser")

		var user *domain.User

		// Decodificar o corpo da requisição
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Erro ao decodificar requisição", nil)
			return
		}

		// Validar campos obrigatórios
		if user.Name == "" || user.Email == "" || user.PasswordHash == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "Nome, email e senha são obrigatórios", nil)
			return
		}

		// Criar o usuário
		user, err := service.CreateUser(user)
		if err != nil {
			logrus.Error(err)

			// Verificar cada tipo específico de erro
			if errors.Is(err, authenticating.ErrUserAlreadyExists) {
				apiErrors.WriteError(w, apiErrors.ErrUserAlreadyExists, "Email já cadastrado", nil)
				return
			} else if errors.Is(err, authenticating.ErrMissingRequiredData) {
				apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, err.Error(), nil)
				return
			} else if errors.Is(err, authenticating.ErrDatabaseOperation) {
				apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao criar usuário", nil)
				return
			}

			// Verificar se é um AuthError (usando type assertion para ponteiro)
			var authErr *authenticating.AuthError
			if errors.As(err, &authErr) {
				apiErrors.WriteError(w, authErr.Code, authErr.Details, nil)
				return
			}

			// Para outros tipos de erro
			apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao criar usuário", nil)
			return
		}

		// Resposta de sucesso
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(w).Encode(user)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
			return
		}
	}
}

// ListUsers lista todos os usuários
func ListUsers(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verificar se o usuário que faz a requisição é um administrador
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok || userClaims.UserRoleID != 1 {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Apenas administradores podem listar todos os usuários", nil)
			return
		}

		// Buscar lista de usuários
		users, err := service.ListUser()
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao buscar usuários", nil)
			return
		}

		// Enviar resposta
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(users)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
			return
		}
	}
}

// UpdateUser atualiza informações do usuário
func UpdateUser(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("INIT - UpdateUser")

		// Extrair ID do usuário da URL
		idStr := httprouter.ParamsFromContext(r.Context()).ByName("id")
		if idStr == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "ID do usuário não fornecido", nil)
			return
		}

		id, err := strconv.Atoi(idStr)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidFormat, "ID do usuário inválido", nil)
			return
		}

		// Verificar permissões: o usuário pode editar apenas seu próprio perfil, a menos que seja admin
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok || (userClaims.UserID != id && userClaims.UserRoleID != 1) {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Você não tem permissão para editar este usuário", nil)
			return
		}

		// Decodificar o corpo da requisição
		var updateReq domain.UpdateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Erro ao decodificar requisição", nil)
			return
		}

		// Definir o ID do usuário a ser atualizado
		updateReq.ID = id

		// Restringir alterações de RoleID apenas para administradores
		if updateReq.RoleID != nil && userClaims.UserRoleID != 1 {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Apenas administradores podem alterar o tipo de usuário", nil)
			return
		}

		// Atualizar o usuário
		err = service.UpdateUser(&updateReq)
		if err != nil {
			logrus.Error(err)
			if err.Error() == "email already exists" {
				apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Email já cadastrado", nil)
				return
			}
			apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao atualizar usuário", nil)
			return
		}

		// Resposta de sucesso
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}
