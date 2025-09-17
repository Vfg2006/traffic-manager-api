package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	ssoticadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/domain"
	ssoticamocks "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/mocks"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository/mocks"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"go.uber.org/mock/gomock"
)

func TestTopRankingAccountsService_processTopRankingAccounts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mocks
	mockAccountRepo := mocks.NewMockAccountRepository(ctrl)
	mockRankingRepo := mocks.NewMockStoreRankingRepository(ctrl)
	mockSSOticaService := ssoticamocks.NewMockSSOticaIntegrator(ctrl)

	// Service
	service := &TopRankingAccountsService{
		accountRepo:    mockAccountRepo,
		rankingRepo:    mockRankingRepo,
		ssoticaService: mockSSOticaService,
	}

	// Datas de referência (baseadas na data de referência do teste: 16 de janeiro)
	yesterday := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC) // 15 de janeiro (ontem do dia 16)
	month := yesterday.Format("01-2006")

	tests := []struct {
		name     string
		accounts []*domain.AdAccount
		setup    func()
		validate func(t *testing.T, result []*domain.StoreRankingItem)
	}{
		{
			name: "Conta nova sem ranking anterior - deve calcular receita total do mês",
			accounts: []*domain.AdAccount{
				{
					ID:         "ACC001",
					Name:       "Loja A",
					CNPJ:       stringPtr("12345678901"),
					SecretName: stringPtr("secret1"),
				},
			},
			setup: func() {
				// Mock: GetByAccountID retorna nil (conta nova)
				mockRankingRepo.EXPECT().
					GetByAccountID("ACC001", month).
					Return(nil, nil)

				// Mock: SSOtica retorna vendas do mês inteiro
				mockSSOticaService.EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					Return([]ssoticadomain.Order{
						{
							NetAmount:       1000.0,
							CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
						},
						{
							NetAmount:       1500.0,
							CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
						},
					}, nil)

				// Mock: SaveOrUpdateStoreRanking
				mockRankingRepo.EXPECT().
					SaveOrUpdateStoreRanking(gomock.Any()).
					Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				// Validar que a receita total foi calculada corretamente (1000 + 1500 = 2500)
				assert.Len(t, result, 1)
				assert.Equal(t, "ACC001", result[0].AccountID)
				assert.Equal(t, "01-2024", result[0].Month)
				assert.Equal(t, "Loja A", result[0].StoreName)
				assert.Equal(t, 2500.0, result[0].SocialNetworkRevenue)
				assert.Equal(t, 1, result[0].Position)
				assert.Equal(t, 0, result[0].PositionChange)   // Resetado para nova conta
				assert.Equal(t, 0, result[0].PreviousPosition) // Resetado para nova conta
			},
		},
		{
			name: "Conta com ranking anterior - deve calcular receita total do mês e comparar posições",
			accounts: []*domain.AdAccount{
				{
					ID:         "ACC002",
					Name:       "Loja B",
					CNPJ:       stringPtr("12345678902"),
					SecretName: stringPtr("secret2"),
				},
			},
			setup: func() {
				existingRanking := &domain.StoreRankingItem{
					AccountID:            "ACC002",
					Month:                "01-2024",
					StoreName:            "Loja B",
					SocialNetworkRevenue: 5000.0,
					Position:             2,
					UpdatedAt:            yesterday,
				}

				// Mock: GetByAccountID retorna ranking existente
				mockRankingRepo.EXPECT().
					GetByAccountID("ACC002", month).
					Return(existingRanking, nil)

				// Mock: SSOtica retorna vendas do mês inteiro
				mockSSOticaService.EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					Return([]ssoticadomain.Order{
						{
							NetAmount:       800.0,
							CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
						},
					}, nil)

				// Mock: SaveOrUpdateStoreRanking
				mockRankingRepo.EXPECT().
					SaveOrUpdateStoreRanking(gomock.Any()).
					Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				// Validar que a receita total do mês foi calculada (800)
				assert.Len(t, result, 1)
				assert.Equal(t, "ACC002", result[0].AccountID)
				assert.Equal(t, "01-2024", result[0].Month)
				assert.Equal(t, "Loja B", result[0].StoreName)
				assert.Equal(t, 800.0, result[0].SocialNetworkRevenue)
				assert.Equal(t, 1, result[0].Position)
			},
		},
		{
			name: "Conta com ranking anterior - deve calcular receita total do mês",
			accounts: []*domain.AdAccount{
				{
					ID:         "ACC003",
					Name:       "Loja C",
					CNPJ:       stringPtr("12345678903"),
					SecretName: stringPtr("secret3"),
				},
			},
			setup: func() {
				// Ranking anterior (será usado para calcular mudança de posição)
				existingRanking := &domain.StoreRankingItem{
					AccountID:            "ACC003",
					Month:                "01-2024",
					StoreName:            "Loja C",
					SocialNetworkRevenue: 3000.0,
					Position:             3,
					UpdatedAt:            yesterday,
				}

				// Mock: GetByAccountID retorna ranking anterior
				mockRankingRepo.EXPECT().
					GetByAccountID("ACC003", month).
					Return(existingRanking, nil)

				// Mock: SSOtica retorna vendas do mês inteiro
				mockSSOticaService.EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					Return([]ssoticadomain.Order{
						{
							NetAmount:       600.0,
							CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
						},
						{
							NetAmount:       700.0,
							CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin},
						},
					}, nil)

				// Mock: SaveOrUpdateStoreRanking
				mockRankingRepo.EXPECT().
					SaveOrUpdateStoreRanking(gomock.Any()).
					Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				// Validar que a receita total do mês foi calculada (600 + 700 = 1300)
				assert.Len(t, result, 1)
				assert.Equal(t, "ACC003", result[0].AccountID)
				assert.Equal(t, "01-2024", result[0].Month)
				assert.Equal(t, "Loja C", result[0].StoreName)
				assert.Equal(t, 1300.0, result[0].SocialNetworkRevenue)
				assert.Equal(t, 1, result[0].Position)
			},
		},
		{
			name: "Múltiplas contas - deve calcular posições corretamente",
			accounts: []*domain.AdAccount{
				{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
				{ID: "ACC002", Name: "Loja B", CNPJ: stringPtr("12345678902"), SecretName: stringPtr("secret2")},
				{ID: "ACC003", Name: "Loja C", CNPJ: stringPtr("12345678903"), SecretName: stringPtr("secret3")},
			},
			setup: func() {
				// Setup para ACC001 (receita total: 2500)
				mockRankingRepo.EXPECT().GetByAccountID("ACC001", month).Return(nil, nil)

				// Setup para ACC002 (receita total: 3000)
				mockRankingRepo.EXPECT().GetByAccountID("ACC002", month).Return(nil, nil)

				// Setup para ACC003 (receita total: 1500)
				mockRankingRepo.EXPECT().GetByAccountID("ACC003", month).Return(nil, nil)

				// Mock: SSOtica retorna vendas diferentes para cada conta
				mockSSOticaService.EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					Return([]ssoticadomain.Order{
						{NetAmount: 2500.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
					}, nil).Times(1) // ACC001

				mockSSOticaService.EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					Return([]ssoticadomain.Order{
						{NetAmount: 3000.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
					}, nil).Times(1) // ACC002

				mockSSOticaService.EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					Return([]ssoticadomain.Order{
						{NetAmount: 1500.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
					}, nil).Times(1) // ACC003

				// Mock: SaveOrUpdateStoreRanking (uma única chamada com slice)
				mockRankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				// Validar ordenação: ACC002 (1º), ACC001 (2º), ACC003 (3º)
				// Todas são contas novas, então terão Month = "01-2024"
				assert.Len(t, result, 3)

				// Verificar ordenação por receita (maior para menor)
				assert.Equal(t, "ACC002", result[0].AccountID) // 3000 - 1º lugar
				assert.Equal(t, "01-2024", result[0].Month)    // Conta nova
				assert.Equal(t, 3000.0, result[0].SocialNetworkRevenue)
				assert.Equal(t, 1, result[0].Position)

				assert.Equal(t, "ACC001", result[1].AccountID) // 2500 - 2º lugar
				assert.Equal(t, "01-2024", result[1].Month)    // Conta nova
				assert.Equal(t, 2500.0, result[1].SocialNetworkRevenue)
				assert.Equal(t, 2, result[1].Position)

				assert.Equal(t, "ACC003", result[2].AccountID) // 1500 - 3º lugar
				assert.Equal(t, "01-2024", result[2].Month)    // Conta nova
				assert.Equal(t, 1500.0, result[2].SocialNetworkRevenue)
				assert.Equal(t, 3, result[2].Position)
			},
		},
		{
			name: "Mudança de posição - deve calcular PositionChange corretamente",
			accounts: []*domain.AdAccount{
				{ID: "ACC001", Name: "Loja A", CNPJ: stringPtr("12345678901"), SecretName: stringPtr("secret1")},
				{ID: "ACC002", Name: "Loja B", CNPJ: stringPtr("12345678902"), SecretName: stringPtr("secret2")},
			},
			setup: func() {
				// ACC001 estava em 2º lugar, agora vai para 1º
				existingRanking1 := &domain.StoreRankingItem{
					AccountID:            "ACC001",
					Month:                "01-2024",
					StoreName:            "Loja A",
					SocialNetworkRevenue: 2000.0,
					Position:             2,
					UpdatedAt:            yesterday,
				}
				mockRankingRepo.EXPECT().GetByAccountID("ACC001", month).Return(existingRanking1, nil)

				// ACC002 estava em 1º lugar, agora vai para 2º
				existingRanking2 := &domain.StoreRankingItem{
					AccountID:            "ACC002",
					Month:                "01-2024",
					StoreName:            "Loja B",
					SocialNetworkRevenue: 3000.0,
					Position:             1,
					UpdatedAt:            yesterday,
				}
				mockRankingRepo.EXPECT().GetByAccountID("ACC002", month).Return(existingRanking2, nil)

				// Mock: SSOtica retorna vendas diferentes para cada conta
				// ACC001: receita total do mês até ontem (15 de janeiro) = 1500
				mockSSOticaService.EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					Return([]ssoticadomain.Order{
						{NetAmount: 1500.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
					}, nil).Times(1)

				// ACC002: receita total do mês até ontem (15 de janeiro) = 200
				mockSSOticaService.EXPECT().
					GetSalesByAccount(gomock.Any(), gomock.Any()).
					Return([]ssoticadomain.Order{
						{NetAmount: 200.0, CustomerOrigins: []ssoticadomain.Origin{ssoticadomain.SocialNetworkOrigin}},
					}, nil).Times(1)

				// Mock: SaveOrUpdateStoreRanking (uma única chamada com slice)
				mockRankingRepo.EXPECT().SaveOrUpdateStoreRanking(gomock.Any()).Return(nil)
			},
			validate: func(t *testing.T, result []*domain.StoreRankingItem) {
				// ACC001: Position=1, PositionChange=+1 (subiu), PreviousPosition=2
				// ACC002: Position=2, PositionChange=-1 (desceu), PreviousPosition=1
				// Ambos mantêm o Month original (01-2024)
				assert.Len(t, result, 2)

				// ACC001 subiu para 1º lugar (1500 > 200)
				acc001 := result[0]
				assert.Equal(t, "ACC001", acc001.AccountID)
				assert.Equal(t, "01-2024", acc001.Month) // Mantém o Month original
				assert.Equal(t, 1500.0, acc001.SocialNetworkRevenue)
				assert.Equal(t, 1, acc001.Position)
				assert.Equal(t, 1, acc001.PositionChange) // subiu 1 posição
				assert.Equal(t, 2, acc001.PreviousPosition)

				// ACC002 desceu para 2º lugar
				acc002 := result[1]
				assert.Equal(t, "ACC002", acc002.AccountID)
				assert.Equal(t, "01-2024", acc002.Month) // Mantém o Month original
				assert.Equal(t, 200.0, acc002.SocialNetworkRevenue)
				assert.Equal(t, 2, acc002.Position)
				assert.Equal(t, -1, acc002.PositionChange) // desceu 1 posição
				assert.Equal(t, 1, acc002.PreviousPosition)
			},
		},
	}

	for _, tt := range tests {
		// if tt.name != "Mudança de posição - deve calcular PositionChange corretamente" {
		// 	continue
		// }
		t.Run(tt.name, func(t *testing.T) {
			// Setup dos mocks
			tt.setup()

			// Executar o método com data específica (16 de janeiro)
			referenceDate := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
			result := service.processTopRankingAccountsWithDate(tt.accounts, referenceDate)

			// Validações específicas
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestTopRankingAccountsService_getActiveAccounts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAccountRepo := mocks.NewMockAccountRepository(ctrl)
	service := &TopRankingAccountsService{
		accountRepo: mockAccountRepo,
	}

	tests := []struct {
		name     string
		setup    func()
		expected int
		hasError bool
	}{
		{
			name: "Deve filtrar apenas contas ativas com CNPJ e SecretName",
			setup: func() {
				cnpj1 := "12345678901"
				secret1 := "secret1"
				cnpj2 := "98765432101"
				secret2 := "secret2"
				emptyCNPJ := ""

				accounts := []*domain.AdAccount{
					{ID: "ACC001", CNPJ: &cnpj1, SecretName: &secret1},     // Válida
					{ID: "ACC002", CNPJ: &cnpj2, SecretName: &secret2},     // Válida
					{ID: "ACC003", CNPJ: nil, SecretName: &secret1},        // Inválida (CNPJ nil)
					{ID: "ACC004", CNPJ: &emptyCNPJ, SecretName: &secret1}, // Inválida (CNPJ vazio)
					{ID: "ACC005", CNPJ: &cnpj1, SecretName: nil},          // Inválida (SecretName nil)
				}

				mockAccountRepo.EXPECT().
					ListAccounts([]domain.AdAccountStatus{domain.AdAccountStatusActive}).
					Return(accounts, nil)
			},
			expected: 2,
			hasError: false,
		},
		{
			name: "Deve retornar erro quando repository falha",
			setup: func() {
				mockAccountRepo.EXPECT().
					ListAccounts([]domain.AdAccountStatus{domain.AdAccountStatusActive}).
					Return(nil, assert.AnError)
			},
			expected: 0,
			hasError: true,
		},
		{
			name: "Deve retornar lista vazia quando não há contas",
			setup: func() {
				mockAccountRepo.EXPECT().
					ListAccounts([]domain.AdAccountStatus{domain.AdAccountStatusActive}).
					Return([]*domain.AdAccount{}, nil)
			},
			expected: 0,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			accounts, err := service.getActiveAccounts()

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, accounts, tt.expected)
			}
		})
	}
}

func TestEqualDate(t *testing.T) {
	tests := []struct {
		name     string
		date1    time.Time
		date2    time.Time
		expected bool
	}{
		{
			name:     "Datas iguais devem retornar true",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2024, 1, 15, 20, 45, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "Datas diferentes devem retornar false",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2024, 1, 16, 10, 30, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Anos diferentes devem retornar false",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Meses diferentes devem retornar false",
			date1:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			date2:    time.Date(2024, 2, 15, 10, 30, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EqualDate(tt.date1, tt.date2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSecondDayOfMonth(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected bool
	}{
		{
			name:     "Segundo dia do mês deve retornar true",
			input:    time.Date(2024, 1, 2, 10, 30, 45, 123, time.UTC),
			expected: true,
		},
		{
			name:     "Primeiro dia do mês deve retornar false",
			input:    time.Date(2024, 1, 1, 5, 15, 30, 0, time.Local),
			expected: false,
		},
		{
			name:     "Terceiro dia do mês deve retornar false",
			input:    time.Date(2024, 1, 3, 15, 45, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Último dia do mês deve retornar false",
			input:    time.Date(2024, 12, 31, 23, 59, 59, 999, time.UTC),
			expected: false,
		},
		{
			name:     "Meio do mês deve retornar false",
			input:    time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSecondDayOfMonth(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFirstDayOfMonth(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "Deve retornar primeiro dia do mês",
			input:    time.Date(2024, 1, 15, 10, 30, 45, 123, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Já é primeiro dia do mês",
			input:    time.Date(2024, 2, 1, 5, 15, 30, 0, time.Local),
			expected: time.Date(2024, 2, 1, 0, 0, 0, 0, time.Local),
		},
		{
			name:     "Último dia do mês",
			input:    time.Date(2024, 12, 31, 23, 59, 59, 999, time.UTC),
			expected: time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFirstDayOfMonth(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
