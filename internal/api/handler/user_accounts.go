package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	apiErrors "github.com/vfg2006/traffic-manager-api/internal/api/errors"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/authenticating"
	"github.com/vfg2006/traffic-manager-api/pkg/middleware"
)

type UserAccountsRequest struct {
	AccountIDs []string `json:"account_ids"`
}

// GetUserAccounts retorna as contas vinculadas a um usuário
func GetUserAccounts(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verificar permissões: o usuário pode ver apenas suas próprias contas, a menos que seja admin
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Você não tem permissão para ver as contas deste usuário", nil)
			return
		}

		// Buscar contas vinculadas
		accounts, err := service.GetUserLinkedAccounts(userClaims.UserID)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao buscar contas vinculadas", nil)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(accounts); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
		}
	}
}

// UpdateUserAccounts atualiza as contas vinculadas a um usuário
func UpdateUserAccounts(service authenticating.Authenticator) http.HandlerFunc {
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

		// Verificar permissões: apenas administradores podem alterar contas vinculadas
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok || userClaims.UserRoleID != 1 {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Apenas administradores podem alterar as contas vinculadas", nil)
			return
		}

		// Decodificar o corpo da requisição
		var req UserAccountsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Erro ao decodificar requisição", nil)
			return
		}

		// Atualizar contas vinculadas
		err = service.ManageUserAccounts(id, req.AccountIDs)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao atualizar contas vinculadas", nil)
			return
		}

		// Resposta de sucesso
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"message":     "Contas vinculadas atualizadas com sucesso",
			"user_id":     id,
			"account_ids": req.AccountIDs,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
		}
	}
}

// LinkUserAccount adiciona múltiplas contas vinculadas a um usuário.
// Recebe uma lista de IDs de contas no corpo da requisição para vincular a um único usuário.
// Apenas administradores podem realizar esta operação.
func LinkUserAccount(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extrair ID do usuário da URL
		userIDStr := httprouter.ParamsFromContext(r.Context()).ByName("id")

		if userIDStr == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "ID do usuário é obrigatório", nil)
			return
		}

		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidFormat, "ID do usuário inválido", nil)
			return
		}

		// Verificar permissões: apenas administradores podem vincular contas
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok || userClaims.UserRoleID != 1 {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Apenas administradores podem vincular contas", nil)
			return
		}

		// Decodificar o corpo da requisição para obter a lista de IDs de contas
		var req UserAccountsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Erro ao decodificar requisição", nil)
			return
		}

		if len(req.AccountIDs) == 0 {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "Lista de IDs de contas não pode estar vazia", nil)
			return
		}

		// Vincular cada conta da lista ao usuário
		var successfulLinks []string
		var failedLinks []string

		for _, accountID := range req.AccountIDs {
			err = service.LinkUserAccount(userID, accountID)
			if err != nil {
				logrus.Warnf("Erro ao vincular conta %s ao usuário %d: %v", accountID, userID, err)
				failedLinks = append(failedLinks, accountID)
			} else {
				successfulLinks = append(successfulLinks, accountID)
			}
		}

		// Resposta de sucesso
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"message":          "Contas vinculadas processadas",
			"user_id":          userID,
			"successful_links": successfulLinks,
		}

		if len(failedLinks) > 0 {
			response["failed_links"] = failedLinks
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
		}
	}
}

// UnlinkUserAccount remove uma conta vinculada de um usuário
func UnlinkUserAccount(service authenticating.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extrair ID do usuário da URL
		params := httprouter.ParamsFromContext(r.Context())
		userIDStr := params.ByName("id")
		accountID := params.ByName("account_id")

		if userIDStr == "" || accountID == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "ID do usuário e ID da conta são obrigatórios", nil)
			return
		}

		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInvalidFormat, "ID do usuário inválido", nil)
			return
		}

		// Verificar permissões: apenas administradores podem desvincular contas
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok || userClaims.UserRoleID != 1 {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Apenas administradores podem desvincular contas", nil)
			return
		}

		// Desvincular conta
		err = service.UnlinkUserAccount(userID, accountID)
		if err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao desvincular conta", nil)
			return
		}

		// Resposta de sucesso
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"message":    "Conta desvinculada com sucesso",
			"user_id":    userID,
			"account_id": accountID,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logrus.Error(err)
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao enviar resposta", nil)
		}
	}
}
