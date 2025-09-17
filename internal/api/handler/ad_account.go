package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/account"
	"github.com/vfg2006/traffic-manager-api/pkg/apiErrors"
)

func AdAccountList(service account.AccountService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filterStatus := r.URL.Query().Get("status")

		var availableStatusList []string
		availableStatus := make([]domain.AdAccountStatus, 0)
		if filterStatus != "" {
			availableStatusList = strings.Split(filterStatus, ",")

			for _, status := range availableStatusList {
				availableStatus = append(availableStatus, domain.AdAccountStatus(status))
			}
		}

		adAccounts, err := service.ListAdAccounts(availableStatus)
		if err != nil {
			logrus.Error("Error listing accounts:", err)

			// Verificar se é um AccountError para obter detalhes específicos do erro
			var accountErr *account.AccountError
			if errors.As(err, &accountErr) {
				apiErrors.WriteError(w, accountErr.Code, accountErr.Error(), nil)
				return
			}

			// Caso não seja um AccountError específico, verificar erros comuns
			if errors.Is(err, account.ErrFetchAccounts) {
				apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao consultar contas no banco de dados", nil)
			} else {
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao listar contas", nil)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(adAccounts); err != nil {
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao codificar resposta", nil)
		}
	})
}

func SyncAccounts(service account.AccountService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("INIT - SyncAccounts")

		resp, err := service.SyncAccounts()
		if err != nil {
			logrus.Error("Error syncing accounts:", err)

			// Verificar se é um AccountError para obter detalhes específicos do erro
			var accountErr *account.AccountError
			if errors.As(err, &accountErr) {
				apiErrors.WriteError(w, accountErr.Code, accountErr.Error(), nil)
				return
			}

			// Caso não seja um AccountError específico, verificar erros comuns
			switch {
			case errors.Is(err, account.ErrMetaIntegration):
				apiErrors.WriteError(w, apiErrors.ErrExternalService, "Erro ao obter contas do serviço Meta", nil)

			case errors.Is(err, account.ErrFetchAccounts) || errors.Is(err, account.ErrDatabaseOperation):
				apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao consultar contas no banco de dados", nil)

			case errors.Is(err, account.ErrGenerateID):
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao gerar identificadores únicos", nil)

			default:
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao sincronizar contas", nil)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao codificar resposta", nil)
		}
	})
}

// TODO talvez adicionar qual usuário está modificando a conta a partir do token
func UpdateAdAccount(service account.AccountService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("INIT - UpdateAdAccount")

		// Define o tipo de conteúdo da resposta
		w.Header().Set("Content-Type", "application/json")

		id := httprouter.ParamsFromContext(r.Context()).ByName("id")
		if id == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "ID da conta é obrigatório", nil)
			return
		}

		// Decodifica o corpo da requisição
		var updateRequest domain.UpdateAdAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&updateRequest); err != nil {
			apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Corpo da requisição inválido: "+err.Error(), nil)
			return
		}

		// Garante que o ID da URL seja usado
		updateRequest.ID = id

		// Atualiza a conta
		resp, err := service.UpdateAccount(&updateRequest)
		if err != nil {
			logrus.Error("Error updating account:", err)

			// Verificar se é um AccountError para obter detalhes específicos do erro
			var accountErr *account.AccountError
			if errors.As(err, &accountErr) {
				apiErrors.WriteError(w, accountErr.Code, accountErr.Error(), map[string]interface{}{
					"account_id": accountErr.AccountID,
					"error_type": accountErr.Err.Error(),
				})
				return
			}

			// Caso não seja um AccountError específico, verificar erros comuns
			switch {
			case errors.Is(err, account.ErrAccountIDRequired):
				apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "ID da conta é obrigatório", nil)

			case errors.Is(err, account.ErrAccountNotFound):
				apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Conta não encontrada", map[string]interface{}{
					"account_id": id,
					"error_type": "account_not_found",
				})

			case errors.Is(err, account.ErrInvalidToken):
				apiErrors.WriteError(w, apiErrors.ErrInvalidTokenSSOtica, "Token inválido para a integração", nil)

			case errors.Is(err, account.ErrDatabaseOperation) || errors.Is(err, account.ErrUpdateAccount):
				apiErrors.WriteError(w, apiErrors.ErrDatabaseOperation, "Erro ao atualizar conta no banco de dados", nil)

			case errors.Is(err, account.ErrRenderSecretUpdate):
				apiErrors.WriteError(w, apiErrors.ErrExternalService, "Erro ao atualizar chave secreta no Render", nil)

			case errors.Is(err, account.ErrSSOticaConnection):
				apiErrors.WriteError(w, apiErrors.ErrExternalService, "Erro ao verificar conexão com o serviço SSOtica", nil)

			default:
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro interno ao atualizar conta", nil)
			}
			return
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Erro ao codificar resposta", nil)
		}
	})
}
