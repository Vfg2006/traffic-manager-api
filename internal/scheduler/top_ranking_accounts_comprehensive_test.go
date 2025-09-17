package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	ssoticadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/domain"
	ssoticamocks "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/mocks"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository/mocks"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"go.uber.org/mock/gomock"
)

// TestTopRankingAccountsService_CronjobScenarios testa os diferentes cenários de execução do cronjob
func TestTopRankingAccountsService_CronjobScenarios(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	accountsMock := []*domain.AdAccount{
		{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
		{ID: "ACC002", Name: "Loja B", CNPJ: stringPtr("12345678902"), SecretName: stringPtr("secret2")},
	}

	tests := []struct {
		name             string
		executionDate    time.Time
		accounts         []*domain.AdAccount
		existingRankings map[string]*domain.StoreRankingItem
		setup            func(*mocks.MockAccountRepository, *mocks.MockStoreRankingRepository, *ssoticamocks.MockSSOticaIntegrator)
		validate         func(t *testing.T, result []*domain.StoreRankingItem, executionDate time.Time)
	}{
		{
			name:          "Execução no meio do mês - deve atualizar ranking do mês atual e manter na mesma posição",
			executionDate: time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC), // 15 de janeiro às 6h
			accounts:      accountsMock,
			existingRankings: map[string]*domain.StoreRankingItem{
				"ACC001": {
					AccountID:            "ACC001",
					Month:                "01-2024",
					StoreName:            "Loja A",
					SocialNetworkRevenue: 5000.0,
					Position:             1,
					UpdatedAt:            time.Date(2024, 1, 14, 6, 0, 0, 0, time.UTC),
				},
				"ACC002": {
					AccountID:            "ACC002",
					Month:                "01-2024",
					StoreName:            "Loja B",
					SocialNetworkRevenue: 3000.0,
					Position:             2,
					UpdatedAt:            time.Date(2024, 1, 14, 6, 0, 0, 0, time.UTC),
				},
			},
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				// Mock para buscar ranking anterior
				rankingRepo.EXPECT().GetByAccountID("ACC001", "01-2024").Return(&domain.StoreRankingItem{
					AccountID:            "ACC001",
					Month:                "01-2024",
					StoreName:            "Loja A",
					SocialNetworkRevenue: 5000.0,
					Position:             1,
					UpdatedAt:            time.Date(2024, 1, 14, 6, 0, 0, 0, time.UTC),
				}, nil)

				rankingRepo.EXPECT().GetByAccountID("ACC002", "01-2024").Return(&domain.StoreRankingItem{
					AccountID:            "ACC002",
					Month:                "01-2024",
					StoreName:            "Loja B",
					SocialNetworkRevenue: 3000.0,
					Position:             2,
					UpdatedAt:            time.Date(2024, 1, 14, 6, 0, 0, 0, time.UTC),
				}, nil)

				// Mock para vendas do SSOtica (receita total do mês até ontem)
				ssoticaService.
					EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					DoAndReturn(func(params ssoticadomain.GetSalesParams, filters *domain.InsigthFilters) ([]ssoticadomain.Order, error) {
						orders := []ssoticadomain.Order{}

						if params.CNPJ == *accountsMock[0].CNPJ && params.SecretName == *accountsMock[0].SecretName {
							// ACC001
							orders = append(orders, ssoticadomain.Order{
								NetAmount:       6000.0,
								CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
							})
						} else if params.CNPJ == *accountsMock[1].CNPJ && params.SecretName == *accountsMock[1].SecretName {
							// ACC002
							orders = append(orders, ssoticadomain.Order{
								NetAmount:       4000.0,
								CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
							})
						}

						return orders, nil
					}).
					AnyTimes()

				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem, executionDate time.Time) {
				assert.Len(t, result, 2)

				// Verificar que o mês permanece o mesmo (01-2024)
				for _, ranking := range result {
					assert.Equal(t, "01-2024", ranking.Month)
				}

				// ACC001 deve estar em 1º lugar (6000 > 4000)
				acc001 := result[0]
				assert.Equal(t, "ACC001", acc001.AccountID)
				assert.Equal(t, 6000.0, acc001.SocialNetworkRevenue)
				assert.Equal(t, 1, acc001.Position)
				assert.Equal(t, 0, acc001.PositionChange) // Permaneceu em 1º lugar
				assert.Equal(t, 1, acc001.PreviousPosition)

				// ACC002 deve estar em 2º lugar
				acc002 := result[1]
				assert.Equal(t, "ACC002", acc002.AccountID)
				assert.Equal(t, 4000.0, acc002.SocialNetworkRevenue)
				assert.Equal(t, 2, acc002.Position)
				assert.Equal(t, 0, acc002.PositionChange) // Permaneceu em 2º lugar
				assert.Equal(t, 2, acc002.PreviousPosition)
			},
		},
		{
			name:          "Execução no último dia do mês - deve atualizar ranking do mês atual",
			executionDate: time.Date(2024, 1, 31, 6, 0, 0, 0, time.UTC), // 31 de janeiro às 6h
			accounts: []*domain.AdAccount{
				{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
			},
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				// Mock para buscar ranking anterior
				rankingRepo.EXPECT().GetByAccountID("ACC001", "01-2024").Return(&domain.StoreRankingItem{
					AccountID:            "ACC001",
					Month:                "01-2024",
					StoreName:            "Loja A",
					SocialNetworkRevenue: 15000.0,
					Position:             1,
					UpdatedAt:            time.Date(2024, 1, 30, 6, 0, 0, 0, time.UTC),
				}, nil)

				// Mock para vendas do SSOtica (receita total do mês até ontem - 30 de janeiro)
				ssoticaService.EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return([]ssoticadomain.Order{
					{NetAmount: 20000.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
				}, nil)

				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem, executionDate time.Time) {
				assert.Len(t, result, 1)

				acc001 := result[0]
				assert.Equal(t, "ACC001", acc001.AccountID)
				assert.Equal(t, "01-2024", acc001.Month) // Ainda é janeiro
				assert.Equal(t, 20000.0, acc001.SocialNetworkRevenue)
				assert.Equal(t, 1, acc001.Position)
			},
		},
		{
			name:          "Execução no primeiro dia do mês - deve atualizar ranking do mês anterior",
			executionDate: time.Date(2024, 2, 1, 6, 0, 0, 0, time.UTC), // 1 de fevereiro às 6h
			accounts: []*domain.AdAccount{
				{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
			},
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				// Mock para buscar ranking anterior (ainda do mês anterior - janeiro)
				rankingRepo.EXPECT().GetByAccountID("ACC001", "01-2024").Return(&domain.StoreRankingItem{
					AccountID:            "ACC001",
					Month:                "01-2024",
					StoreName:            "Loja A",
					SocialNetworkRevenue: 25000.0,
					Position:             1,
					UpdatedAt:            time.Date(2024, 1, 31, 6, 0, 0, 0, time.UTC),
				}, nil)

				// Mock para vendas do SSOtica (receita total de janeiro até 31 de janeiro)
				ssoticaService.EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return([]ssoticadomain.Order{
					{NetAmount: 30000.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
				}, nil)

				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem, executionDate time.Time) {
				assert.Len(t, result, 1)

				acc001 := result[0]
				assert.Equal(t, "ACC001", acc001.AccountID)
				assert.Equal(t, "01-2024", acc001.Month) // Ainda é janeiro (mês anterior)
				assert.Equal(t, 30000.0, acc001.SocialNetworkRevenue)
				assert.Equal(t, 1, acc001.Position)
			},
		},
		{
			name:          "Execução no segundo dia do mês - deve gerar novo ranking do mês atual",
			executionDate: time.Date(2024, 2, 2, 6, 0, 0, 0, time.UTC), // 2 de fevereiro às 6h
			accounts:      accountsMock,
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				// Mock para buscar ranking anterior (do mês anterior - janeiro)
				rankingRepo.EXPECT().GetByAccountID("ACC001", "02-2024").Return(nil, nil)

				rankingRepo.EXPECT().GetByAccountID("ACC002", "02-2024").Return(nil, nil)

				// Mock para vendas do SSOtica (receita total de janeiro até 1 de fevereiro)
				ssoticaService.
					EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					DoAndReturn(func(params ssoticadomain.GetSalesParams, filters *domain.InsigthFilters) ([]ssoticadomain.Order, error) {
						orders := []ssoticadomain.Order{}

						if params.CNPJ == *accountsMock[0].CNPJ && params.SecretName == *accountsMock[0].SecretName {
							// ACC001
							orders = append(orders, ssoticadomain.Order{
								NetAmount:       31000.0,
								CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
							})
						} else if params.CNPJ == *accountsMock[1].CNPJ && params.SecretName == *accountsMock[1].SecretName {
							// ACC002
							orders = append(orders, ssoticadomain.Order{
								NetAmount:       21500.0,
								CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
							})
						}

						return orders, nil
					}).
					AnyTimes()

				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem, executionDate time.Time) {
				assert.Len(t, result, 2)

				// Verificar que o mês permanece janeiro (mês anterior)
				for _, ranking := range result {
					assert.Equal(t, "02-2024", ranking.Month) // Ainda é janeiro
				}

				// ACC001 deve estar em 1º lugar (31000 > 21500)
				acc001 := result[0]
				assert.Equal(t, "ACC001", acc001.AccountID)
				assert.Equal(t, "02-2024", acc001.Month)
				assert.Equal(t, 31000.0, acc001.SocialNetworkRevenue)
				assert.Equal(t, 1, acc001.Position)
				assert.Equal(t, 0, acc001.PositionChange) // Permaneceu em 1º lugar
				assert.Equal(t, 0, acc001.PreviousPosition)

				// ACC002 deve estar em 2º lugar
				acc002 := result[1]
				assert.Equal(t, "ACC002", acc002.AccountID)
				assert.Equal(t, "02-2024", acc002.Month)
				assert.Equal(t, 21500.0, acc002.SocialNetworkRevenue)
				assert.Equal(t, 2, acc002.Position)
				assert.Equal(t, 0, acc002.PositionChange) // Permaneceu em 2º lugar
				assert.Equal(t, 0, acc002.PreviousPosition)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup dos mocks
			mockAccountRepo := mocks.NewMockAccountRepository(ctrl)
			mockRankingRepo := mocks.NewMockStoreRankingRepository(ctrl)
			mockSSOticaService := ssoticamocks.NewMockSSOticaIntegrator(ctrl)

			service := &TopRankingAccountsService{
				accountRepo:    mockAccountRepo,
				rankingRepo:    mockRankingRepo,
				ssoticaService: mockSSOticaService,
			}

			tt.setup(mockAccountRepo, mockRankingRepo, mockSSOticaService)

			// Executar o método com a data específica
			result := service.processTopRankingAccountsWithDate(tt.accounts, tt.executionDate)

			// Validações específicas
			if tt.validate != nil {
				tt.validate(t, result, tt.executionDate)
			}
		})
	}
}

// TestTopRankingAccountsService_PositionAccuracy testa a precisão das posições após múltiplas execuções
func TestTopRankingAccountsService_PositionAccuracy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Simular múltiplas execuções do cronjob ao longo do mês
	executions := []struct {
		day           int
		expectedMonth string
		salesData     map[string]float64 // AccountID -> receita acumulada
	}{
		{2, "02-2024", map[string]float64{"ACC001": 1000, "ACC002": 1500, "ACC003": 800}},    // 2 de fevereiro
		{5, "02-2024", map[string]float64{"ACC001": 2500, "ACC002": 3000, "ACC003": 1200}},   // 5 de fevereiro
		{10, "02-2024", map[string]float64{"ACC001": 5000, "ACC002": 4500, "ACC003": 2000}},  // 10 de fevereiro
		{15, "02-2024", map[string]float64{"ACC001": 8000, "ACC002": 6000, "ACC003": 3500}},  // 15 de fevereiro
		{28, "02-2024", map[string]float64{"ACC001": 12000, "ACC002": 9000, "ACC003": 5000}}, // 28 de fevereiro
	}

	accounts := []*domain.AdAccount{
		{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
		{ID: "ACC002", Name: "Loja B", CNPJ: stringPtr("12345678902"), SecretName: stringPtr("secret2")},
		{ID: "ACC003", Name: "Loja C", CNPJ: stringPtr("12345678903"), SecretName: stringPtr("secret3")},
	}

	for i, execution := range executions {
		t.Run(fmt.Sprintf("Execução dia %d - posições devem estar corretas", execution.day), func(t *testing.T) {
			mockAccountRepo := mocks.NewMockAccountRepository(ctrl)
			mockRankingRepo := mocks.NewMockStoreRankingRepository(ctrl)
			mockSSOticaService := ssoticamocks.NewMockSSOticaIntegrator(ctrl)

			service := &TopRankingAccountsService{
				accountRepo:    mockAccountRepo,
				rankingRepo:    mockRankingRepo,
				ssoticaService: mockSSOticaService,
			}

			executionDate := time.Date(2024, 2, execution.day, 6, 0, 0, 0, time.UTC)

			// Setup dos mocks para cada execução
			for _, account := range accounts {
				// Mock para buscar ranking anterior (se não for a primeira execução)
				if i > 0 {
					previousRanking := &domain.StoreRankingItem{
						AccountID:            account.ID,
						Month:                execution.expectedMonth,
						StoreName:            account.Name,
						SocialNetworkRevenue: executions[i-1].salesData[account.ID],
						Position:             getExpectedPosition(account.ID, executions[i-1].salesData),
						UpdatedAt:            time.Date(2024, 2, executions[i-1].day, 6, 0, 0, 0, time.UTC),
					}
					mockRankingRepo.EXPECT().GetByAccountID(account.ID, execution.expectedMonth).Return(previousRanking, nil)
				} else {
					// Primeira execução - buscar primeiro dia do mês
					mockRankingRepo.EXPECT().GetByAccountID(account.ID, "02-2024").Return(nil, nil)
				}

				// Mock para vendas do SSOtica
				mockSSOticaService.
					EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return([]ssoticadomain.Order{
					{NetAmount: execution.salesData[account.ID], CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
				}, nil)
			}

			mockRankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)

			// Executar
			result := service.processTopRankingAccountsWithDate(accounts, executionDate)

			// Validar posições
			assert.Len(t, result, 3)

			// Verificar se todas as contas têm o mês correto
			for _, ranking := range result {
				assert.Equal(t, execution.expectedMonth, ranking.Month)
			}

			// Verificar se as posições estão corretas baseadas na receita
			expectedPositions := getExpectedPositions(execution.salesData)
			for i, ranking := range result {
				expectedPosition := expectedPositions[i]
				assert.Equal(t, expectedPosition.AccountID, ranking.AccountID)
				assert.Equal(t, expectedPosition.Position, ranking.Position)
				assert.Equal(t, expectedPosition.SocialNetworkRevenue, ranking.SocialNetworkRevenue)
			}
		})
	}
}

// TestTopRankingAccountsService_EdgeCases testa casos extremos e situações de stress
func TestTopRankingAccountsService_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name     string
		setup    func(*mocks.MockAccountRepository, *mocks.MockStoreRankingRepository, *ssoticamocks.MockSSOticaIntegrator)
		accounts []*domain.AdAccount
		date     time.Time
		validate func(t *testing.T, result []*domain.StoreRankingItem)
	}{
		{
			name: "Conta sem vendas - deve ter receita zero",
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				rankingRepo.EXPECT().GetByAccountID("ACC001", "01-2024").Return(nil, nil)
				ssoticaService.EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return([]ssoticadomain.Order{}, nil)
				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			accounts: []*domain.AdAccount{
				{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
			},
			date: time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC),
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				assert.Len(t, result, 1)
				assert.Equal(t, 0.0, result[0].SocialNetworkRevenue)
				assert.Equal(t, 1, result[0].Position)
			},
		},
		{
			name: "Erro no SSOtica - deve continuar com outras contas",
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				// ACC001 falha
				rankingRepo.EXPECT().GetByAccountID("ACC001", "01-2024").Return(nil, nil)
				ssoticaService.EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

				// ACC002 funciona
				rankingRepo.EXPECT().GetByAccountID("ACC002", "01-2024").Return(nil, nil)
				ssoticaService.EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return([]ssoticadomain.Order{
					{NetAmount: 1000.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
				}, nil)

				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			accounts: []*domain.AdAccount{
				{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
				{ID: "ACC002", Name: "Loja B", CNPJ: stringPtr("12345678902"), SecretName: stringPtr("secret2")},
			},
			date: time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC),
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				// Deve processar apenas ACC002 (ACC001 falhou)
				assert.Len(t, result, 1)
				assert.Equal(t, "ACC002", result[0].AccountID)
				assert.Equal(t, 1000.0, result[0].SocialNetworkRevenue)
			},
		},
		{
			name: "Mudança de ano - deve criar novo ranking",
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				// Buscar ranking do ano anterior
				rankingRepo.EXPECT().GetByAccountID("ACC001", "12-2023").Return(&domain.StoreRankingItem{
					AccountID:            "ACC001",
					Month:                "12-2023",
					StoreName:            "Loja A",
					SocialNetworkRevenue: 50000.0,
					Position:             1,
					UpdatedAt:            time.Date(2024, 1, 1, 6, 0, 0, 0, time.UTC),
				}, nil)

				ssoticaService.EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return([]ssoticadomain.Order{
					{NetAmount: 1000.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
				}, nil)

				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			accounts: []*domain.AdAccount{
				{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
			},
			date: time.Date(2024, 1, 2, 6, 0, 0, 0, time.UTC), // 2 de janeiro de 2024
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				assert.Len(t, result, 1)
				assert.Equal(t, "01-2024", result[0].Month) // Novo ano, novo mês
				assert.Equal(t, 1000.0, result[0].SocialNetworkRevenue)
			},
		},
		{
			name: "Muitas contas - deve processar todas corretamente",
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				// Criar 10 contas
				for i := 1; i <= 10; i++ {
					accountID := fmt.Sprintf("ACC%03d", i)
					rankingRepo.EXPECT().GetByAccountID(accountID, "01-2024").Return(nil, nil)
					ssoticaService.EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return([]ssoticadomain.Order{
						{NetAmount: float64(i * 1000), CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
					}, nil)
				}
				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			accounts: func() []*domain.AdAccount {
				accounts := make([]*domain.AdAccount, 10)
				for i := 1; i <= 10; i++ {
					cnpj := fmt.Sprintf("123456789%02d", i)
					secret := fmt.Sprintf("secret%d", i)
					accounts[i-1] = &domain.AdAccount{
						ID:         fmt.Sprintf("ACC%03d", i),
						Name:       fmt.Sprintf("Loja %d", i),
						CNPJ:       stringPtr(cnpj),
						SecretName: stringPtr(secret),
					}
				}
				return accounts
			}(),
			date: time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC),
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				assert.Len(t, result, 10)

				// Verificar se estão ordenadas corretamente (maior para menor receita)
				for i := 0; i < len(result)-1; i++ {
					assert.GreaterOrEqual(t, result[i].SocialNetworkRevenue, result[i+1].SocialNetworkRevenue)
					assert.Equal(t, i+1, result[i].Position)
				}
				assert.Equal(t, 10, result[9].Position)
			},
		},
		{
			name: "Receitas iguais - deve manter ordem estável",
			setup: func(accountRepo *mocks.MockAccountRepository, rankingRepo *mocks.MockStoreRankingRepository, ssoticaService *ssoticamocks.MockSSOticaIntegrator) {
				// Todas as contas com a mesma receita
				for _, account := range []string{"ACC001", "ACC002", "ACC003"} {
					rankingRepo.EXPECT().GetByAccountID(account, "01-2024").Return(nil, nil)
					ssoticaService.EXPECT().GetSalesByAccount(gomock.Any(), gomock.Any()).Return([]ssoticadomain.Order{
						{NetAmount: 1000.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
					}, nil)
				}
				rankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			accounts: []*domain.AdAccount{
				{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
				{ID: "ACC002", Name: "Loja B", CNPJ: stringPtr("12345678902"), SecretName: stringPtr("secret2")},
				{ID: "ACC003", Name: "Loja C", CNPJ: stringPtr("12345678903"), SecretName: stringPtr("secret3")},
			},
			date: time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC),
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				assert.Len(t, result, 3)

				// Todas devem ter a mesma receita
				for _, ranking := range result {
					assert.Equal(t, 1000.0, ranking.SocialNetworkRevenue)
				}

				// Posições devem ser atribuídas (mesmo com receitas iguais)
				positions := make(map[int]bool)
				for _, ranking := range result {
					assert.True(t, ranking.Position >= 1 && ranking.Position <= 3)
					assert.False(t, positions[ranking.Position], "Posição duplicada: %d", ranking.Position)
					positions[ranking.Position] = true
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAccountRepo := mocks.NewMockAccountRepository(ctrl)
			mockRankingRepo := mocks.NewMockStoreRankingRepository(ctrl)
			mockSSOticaService := ssoticamocks.NewMockSSOticaIntegrator(ctrl)

			service := &TopRankingAccountsService{
				accountRepo:    mockAccountRepo,
				rankingRepo:    mockRankingRepo,
				ssoticaService: mockSSOticaService,
			}

			tt.setup(mockAccountRepo, mockRankingRepo, mockSSOticaService)

			result := service.processTopRankingAccountsWithDate(tt.accounts, tt.date)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// Funções auxiliares para os testes
func getExpectedPosition(accountID string, salesData map[string]float64) int {
	// Criar slice de pares (accountID, receita) e ordenar por receita
	type accountRevenue struct {
		accountID string
		revenue   float64
	}

	var accounts []accountRevenue
	for id, revenue := range salesData {
		accounts = append(accounts, accountRevenue{id, revenue})
	}

	// Ordenar por receita (maior para menor)
	for i := 0; i < len(accounts)-1; i++ {
		for j := i + 1; j < len(accounts); j++ {
			if accounts[i].revenue < accounts[j].revenue {
				accounts[i], accounts[j] = accounts[j], accounts[i]
			}
		}
	}

	// Encontrar posição da conta
	for i, acc := range accounts {
		if acc.accountID == accountID {
			return i + 1
		}
	}
	return 0
}

func getExpectedPositions(salesData map[string]float64) []*domain.StoreRankingItem {
	// Criar slice de pares (accountID, receita) e ordenar por receita
	type accountRevenue struct {
		accountID string
		revenue   float64
	}

	var accounts []accountRevenue
	for id, revenue := range salesData {
		accounts = append(accounts, accountRevenue{id, revenue})
	}

	// Ordenar por receita (maior para menor)
	for i := 0; i < len(accounts)-1; i++ {
		for j := i + 1; j < len(accounts); j++ {
			if accounts[i].revenue < accounts[j].revenue {
				accounts[i], accounts[j] = accounts[j], accounts[i]
			}
		}
	}

	// Criar rankings ordenados
	var rankings []*domain.StoreRankingItem
	for i, acc := range accounts {
		rankings = append(rankings, &domain.StoreRankingItem{
			AccountID:            acc.accountID,
			SocialNetworkRevenue: acc.revenue,
			Position:             i + 1,
		})
	}

	return rankings
}
