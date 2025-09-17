package account

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica"
	ssoticadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/domain"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/pkg/apiErrors"
	"github.com/vfg2006/traffic-manager-api/pkg/utils"
)

type AccountService interface {
	UpdateAccount(request *domain.UpdateAdAccountRequest) (*domain.UpdateAdAccountResponse, error)
	ListAdAccounts(availableStatus []domain.AdAccountStatus) ([]*domain.AdAccountResponse, error)
	SyncAccounts() (*domain.SyncAccountsResponse, error)
}

type Service struct {
	accountRepository repository.AccountRepository
	metaService       *meta.MetaIntegrator
	renderClient      *config.RenderClient
	ssoticaService    ssotica.SSOticaIntegrator
	cfg               *config.Config
}

func NewService(
	accountRepository repository.AccountRepository,
	metaService *meta.MetaIntegrator,
	renderClient *config.RenderClient,
	ssoticaService ssotica.SSOticaIntegrator,
	cfg *config.Config,
) AccountService {
	return &Service{
		accountRepository: accountRepository,
		metaService:       metaService,
		renderClient:      renderClient,
		ssoticaService:    ssoticaService,
		cfg:               cfg,
	}
}

func (s *Service) ListAdAccounts(availableStatus []domain.AdAccountStatus) ([]*domain.AdAccountResponse, error) {
	accounts, err := s.accountRepository.ListAccounts(availableStatus)
	if err != nil {
		return nil, NewAccountError(ErrFetchAccounts, apiErrors.ErrDatabaseOperation, "Falha ao listar contas no banco de dados")
	}

	// Transforma os accounts para o formato de resposta da API
	adAccountsResponse := make([]*domain.AdAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		adAccountsResponse = append(adAccountsResponse, &domain.AdAccountResponse{
			ID:         account.ID,
			ExternalID: account.ExternalID,
			Name:       account.Name,
			Nickname:   account.Nickname,
			Status:     account.Status,
			CNPJ:       account.CNPJ,
			HasToken:   account.SecretName != nil,
		})
	}

	return adAccountsResponse, nil
}

func (s *Service) SyncAccounts() (*domain.SyncAccountsResponse, error) {
	response := &domain.SyncAccountsResponse{
		Quantity: 0,
		Message:  "Erro ao sincronizar contas",
		Error:    true,
	}

	accounts, err := s.metaService.GetAdAccounts()
	if err != nil {
		logrus.Error("Error getting ad accounts from integrator meta:", err)
		return response, NewAccountError(ErrMetaIntegration, apiErrors.ErrExternalService, "Falha ao obter contas da API do Meta")
	}

	existingAccounts, err := s.accountRepository.ListAccountsMap()
	if err != nil {
		logrus.WithField("error", err).Error("Error getting ad accounts from database")
		return response, NewAccountError(ErrFetchAccounts, apiErrors.ErrDatabaseOperation, "Falha ao consultar contas existentes no banco de dados")
	}

	bms := make([]*domain.BusinessManager, 0)
	accountsToCreate := make([]*domain.AdAccount, 0)
	for _, acc := range accounts {
		externalID := strings.Split(acc.ExternalID, "_")[1]
		compositeKey := fmt.Sprintf("%s:%s", acc.Origin, externalID)

		if _, exists := existingAccounts[compositeKey]; exists {
			continue
		}

		accountID, err := utils.GenerateID()
		if err != nil {
			return response, NewAccountError(ErrGenerateID, apiErrors.ErrInternalServer, "Falha ao gerar identificador único para conta")
		}

		acc.ID = accountID
		acc.ExternalID = externalID
		acc.Status = domain.AdAccountStatusActive

		bmID, err := utils.GenerateID()
		if err != nil {
			return response, NewAccountError(ErrGenerateID, apiErrors.ErrInternalServer, "Falha ao gerar identificador único para business manager")
		}

		accountsToCreate = append(accountsToCreate, acc)

		bms = append(bms, &domain.BusinessManager{
			ID:         bmID,
			ExternalID: acc.BusinessManagerID,
			Name:       acc.BusinessManagerName,
			Origin:     acc.Origin,
		})
	}

	businessManagerIDs, err := s.accountRepository.SaveOrUpdateBusinessManager(bms)
	if err != nil {
		return response, NewAccountError(ErrDatabaseOperation, apiErrors.ErrDatabaseOperation, "Falha ao salvar business managers")
	}

	// Agora tenta salvar as contas com os business managers resolvidos
	if len(accountsToCreate) > 0 {
		err = s.accountRepository.SaveOrUpdate(accountsToCreate, businessManagerIDs)
		if err != nil {
			return response, NewAccountError(ErrDatabaseOperation, apiErrors.ErrDatabaseOperation, "Falha ao salvar contas")
		}
	}

	quantity := len(accountsToCreate)

	logrus.Infof("%d accounts were successfully synced", quantity)

	response.Quantity = quantity
	response.Message = fmt.Sprintf("%d contas foram sincronizadas com sucesso", quantity)
	response.Error = false

	return response, nil
}

func (s *Service) UpdateAccount(request *domain.UpdateAdAccountRequest) (*domain.UpdateAdAccountResponse, error) {
	if request.ID == "" {
		return nil, ErrAccountIDRequired
	}

	// Busca a conta para verificar se existe
	account, err := s.accountRepository.GetAccountByID(request.ID)
	if err != nil {
		logrus.Error("Error getting account by id on the repository:", err)
		return nil, NewAccountError(ErrDatabaseOperation, apiErrors.ErrDatabaseOperation, "Erro ao buscar conta no banco de dados")
	}

	if account == nil {
		return nil, NewAccountErrorWithID(ErrAccountNotFound, apiErrors.ErrInvalidRequest, request.ID, "Conta não encontrada")
	}

	if request.Token != nil && *request.Token != "" {
		key := fmt.Sprintf("ssotica_bm-%s-act-%s", account.BusinessManagerID, account.ID)

		date := time.Now()
		hasConnection, err := s.ssoticaService.CheckConnection(ssoticadomain.CheckConnectionParams{
			CNPJ:      *request.CNPJ,
			Token:     *request.Token,
			StartDate: date,
			EndDate:   date,
		})
		if err != nil {
			logrus.Error("Error check connection with ssotica:", err)
			return nil, NewAccountErrorWithID(ErrSSOticaConnection, apiErrors.ErrInvalidTokenSSOtica, request.ID, "Falha ao verificar conexão com o serviço SSOtica")
		}

		if hasConnection {
			err = s.renderClient.AddOrUpdateSecret(s.cfg.Render.ServiceID, key, *request.Token)
			if err != nil {
				logrus.Error("Error updating secret on render:", err)
				return nil, NewAccountErrorWithID(ErrRenderSecretUpdate, apiErrors.ErrExternalService, request.ID, "Falha ao atualizar chave secreta no Render")
			}

			request.SecretName = &key

			s.cfg.SSOticaMultiClient[key] = config.SSOtica{
				URL:         s.cfg.SSOtica.URL,
				AccessToken: *request.Token,
			}
		} else {
			logrus.Warn("Invalid token for account:", account.ID)
			return nil, NewAccountErrorWithID(ErrInvalidToken, apiErrors.ErrInvalidToken, request.ID, "Token inválido para a conta")
		}
	}

	// Atualiza a conta no repositório
	err = s.accountRepository.UpdateAccount(request)
	if err != nil {
		logrus.Error("Error updating account on the repository:", err)
		return nil, NewAccountErrorWithID(ErrUpdateAccount, apiErrors.ErrDatabaseOperation, request.ID, "Falha ao atualizar conta no banco de dados")
	}

	return &domain.UpdateAdAccountResponse{
		ID:         request.ID,
		Nickname:   request.Nickname,
		CNPJ:       request.CNPJ,
		SecretName: request.SecretName,
		Status:     request.Status,
	}, nil
}
