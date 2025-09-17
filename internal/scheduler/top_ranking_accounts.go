// Package scheduler contém os serviços de agendamento para sincronização de dados
package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica"
	ssoticadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/domain"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

type TopRankingAccountsConfig struct {
	CronSchedule string
	SyncEnabled  bool
}

type TopRankingAccountsService struct {
	scheduler           *gocron.Scheduler
	accountRepo         repository.AccountRepository
	rankingRepo         repository.StoreRankingRepository
	config              TopRankingAccountsConfig
	salesInsightRepo    repository.SalesInsightRepository
	ssoticaService      ssotica.SSOticaIntegrator
	syncRunning         bool
	syncMutex           sync.Mutex
	lastSyncStartedAt   time.Time
	lastSyncCompletedAt time.Time
}

func NewTopRankingAccountsService(
	accountRepo repository.AccountRepository,
	rankingRepo repository.StoreRankingRepository,
	salesInsightRepo repository.SalesInsightRepository,
	ssoticaService ssotica.SSOticaIntegrator,
	cfg *config.Config,
) *TopRankingAccountsService {
	rankingConfig := TopRankingAccountsConfig{
		CronSchedule: cfg.TopRankingAccounts.CronSchedule, // Default: 6h da manhã todos os dias
		SyncEnabled:  cfg.TopRankingAccounts.SyncEnabled,  // Default: desabilitado
	}

	scheduler := gocron.NewScheduler(time.Local)

	logrus.WithFields(logrus.Fields{
		"cron_schedule": rankingConfig.CronSchedule,
	}).Info("Configuração do agendador do top ranking de contas carregada")

	return &TopRankingAccountsService{
		scheduler:        scheduler,
		accountRepo:      accountRepo,
		rankingRepo:      rankingRepo,
		salesInsightRepo: salesInsightRepo,
		ssoticaService:   ssoticaService,
		config:           rankingConfig,
	}
}

func (s *TopRankingAccountsService) Start(ctx context.Context) error {
	if !s.config.SyncEnabled {
		logrus.Info("Cron de atualização de top ranking de contas desabilitada por configuração")
		return nil
	}

	logrus.WithField("cron", s.config.CronSchedule).Info("Iniciando cron de atualização do top ranking de contas")

	// Agendar a sincronização de top ranking de contas
	_, err := s.scheduler.Cron(s.config.CronSchedule).Do(func() {
		if err := s.UpdateTopRankingAccounts(); err != nil {
			logrus.WithError(err).Error("Erro na atualização do top ranking de contas")
		}
	})
	if err != nil {
		return fmt.Errorf("erro ao agendar sincronização de top ranking de contas: %w", err)
	}

	// Executar o cron em uma goroutine separada
	s.scheduler.StartAsync()

	// Configurar o cancelamento do cron quando o contexto for cancelado
	go func() {
		<-ctx.Done()
		logrus.Info("Parando cron do top ranking de contas")
		s.scheduler.Stop()
	}()

	return nil
}

func (s *TopRankingAccountsService) UpdateTopRankingAccounts() error {
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()

	if s.syncRunning {
		logrus.Warn("Sincronização de top ranking de contas já está em execução")
		return nil
	}

	s.syncRunning = true
	s.lastSyncStartedAt = time.Now()
	defer func() {
		s.syncRunning = false
		s.lastSyncCompletedAt = time.Now()
	}()

	logrus.Info("Iniciando atualização do top ranking de contas")

	// TODO: Implementar lógica de atualização do ranking
	activeAccounts, err := s.getActiveAccounts()
	if err != nil {
		logrus.WithError(err).Error("Erro ao buscar lista de contas para atualização do top ranking de contas")
		return err
	}

	s.processTopRankingAccounts(activeAccounts)

	logrus.Info("Atualização do top ranking de contas concluída")

	return nil
}

// getActiveAccounts busca e filtra contas ativas e conecatas com a SS Otica
func (s *TopRankingAccountsService) getActiveAccounts() ([]*domain.AdAccount, error) {
	accounts, err := s.accountRepo.ListAccounts([]domain.AdAccountStatus{domain.AdAccountStatusActive})
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		logrus.Info("Nenhuma conta encontrada para atualização do top ranking de contas")
		return []*domain.AdAccount{}, nil
	}

	activeAccounts := make([]*domain.AdAccount, 0, len(accounts))
	for _, account := range accounts {
		// Apenas com CNPJ e SecretName (necessários para o SSOtica)
		if account.CNPJ != nil && *account.CNPJ != "" && account.SecretName != nil && *account.SecretName != "" {
			activeAccounts = append(activeAccounts, account)
		}
	}

	logrus.WithFields(logrus.Fields{
		"active_accounts": len(activeAccounts),
	}).Info("Contas encontradas para atualização do top ranking de contas")

	if len(activeAccounts) == 0 {
		logrus.Info("Nenhuma conta encontrada para atualização do top ranking de contas")
		return []*domain.AdAccount{}, nil
	}

	return activeAccounts, nil
}

// processTopRankingAccounts processa o top ranking de contas
func (s *TopRankingAccountsService) processTopRankingAccounts(accounts []*domain.AdAccount) {
	s.processTopRankingAccountsWithDate(accounts, time.Now())
}

// // processTopRankingAccountsWithDate processa o top ranking de contas com uma data específica
// func (s *TopRankingAccountsService) processTopRankingAccountsWithDate(accounts []*domain.AdAccount, processingDate time.Time) []*domain.StoreRankingItem {
// 	wg := sync.WaitGroup{}

// 	rankings := make(chan domain.StoreRankingItem, len(accounts))
// 	rankingBeforeUpdate := make(chan domain.StoreRankingItem, len(accounts))
// 	for _, account := range accounts {
// 		wg.Add(1)
// 		go func(account *domain.AdAccount) {
// 			defer wg.Done()

// 			yesterday := processingDate.AddDate(0, 0, -1)

// 			topRankingItem, err := s.rankingRepo.GetByAccountID(account.ID)
// 			if err != nil {
// 				logrus.WithError(err).Error("Erro ao buscar top ranking de contas")
// 				return
// 			}

// 			if topRankingItem != nil {
// 				rankingBeforeUpdate <- *topRankingItem
// 			}

// 			// Verificar se é o segundo dia do mês para criar novo registro
// 			if isSecondDayOfMonth(processingDate) {
// 				logrus.WithFields(logrus.Fields{
// 					"account_id": account.ID,
// 					"date":       processingDate.Format("2006-01-02"),
// 				}).Info("Segundo dia do mês - criando novo registro para o mês atual")

// 				currentMonth := yesterday.Format("01-2006") // Formato mm-yyyy

// 				// Criar novo item de ranking para o mês atual (não zerar o existente)
// 				topRankingItem = &domain.StoreRankingItem{
// 					AccountID:            account.ID,
// 					Month:                currentMonth,
// 					StoreName:            account.Name,
// 					SocialNetworkRevenue: 0, // Começar do zero para o novo mês
// 					Position:             0,
// 					PositionChange:       0,
// 					PreviousPosition:     0,
// 				}

// 				// Para o segundo dia do mês, calcular a data de início do mês (ontem)
// 				salesInsight, err := s.salesInsightRepo.GetByAccountIDAndDate(account.ID, yesterday)
// 				if err != nil || salesInsight == nil {
// 					// TODO pensar se vale a pena buscar ao vivo
// 					logrus.WithError(err).Error("Erro ao buscar insights de vendas")
// 					return
// 				}

// 				s.SumSocialNetworkRevenue([]*domain.SalesInsightEntry{salesInsight}, topRankingItem)

// 				rankings <- *topRankingItem
// 				return
// 			}

// 			if topRankingItem == nil {
// 				logrus.WithFields(logrus.Fields{"account_id": account.ID}).Info("Top ranking de contas não encontrado")

// 				// Criar novo item de ranking
// 				topRankingItem = &domain.StoreRankingItem{
// 					AccountID:            account.ID,
// 					Month:                yesterday.Format("01-2006"), // Formato mm-yyyy
// 					StoreName:            account.Name,
// 					SocialNetworkRevenue: 0,
// 					Position:             0,
// 					PositionChange:       0,
// 					PreviousPosition:     0,
// 				}

// 				startDate := getFirstDayOfMonth(processingDate)

// 				salesInsights, err := s.salesInsightRepo.GetByDateRange(account.ID, startDate, yesterday)
// 				if err != nil || len(salesInsights) == 0 {
// 					logrus.WithError(err).Error("Erro ao buscar insights de vendas")
// 					return
// 				}

// 				s.SumSocialNetworkRevenue(salesInsights, topRankingItem)

// 				rankings <- *topRankingItem
// 				return
// 			}

// 			// Para registros existentes, manter o Month original (preservar histórico)
// 			if EqualDate(topRankingItem.UpdatedAt, yesterday) {
// 				salesInsight, err := s.salesInsightRepo.GetByAccountIDAndDate(account.ID, yesterday)
// 				if err != nil || salesInsight == nil {
// 					// TODO pensar se vale a pena buscar ao vivo
// 					logrus.WithError(err).Error("Erro ao buscar insights de vendas")
// 					return
// 				}

// 				s.SumSocialNetworkRevenue([]*domain.SalesInsightEntry{salesInsight}, topRankingItem)

// 				rankings <- *topRankingItem
// 				return
// 			}

// 			// Para registros desatualizados, usar UpdatedAt como data de início
// 			// Manter o Month original para preservar o histórico mensal
// 			salesInsights, err := s.salesInsightRepo.GetByDateRange(account.ID, topRankingItem.UpdatedAt, yesterday)
// 			if err != nil || len(salesInsights) == 0 {
// 				logrus.WithError(err).Error("Erro ao buscar insights de vendas")
// 				return
// 			}

// 			s.SumSocialNetworkRevenue(salesInsights, topRankingItem)

// 			rankings <- *topRankingItem
// 		}(account)
// 	}

// 	wg.Wait()

// 	close(rankings)
// 	close(rankingBeforeUpdate)

// 	rankingsBeforeUpdate := make([]*domain.StoreRankingItem, 0)
// 	for ranking := range rankingBeforeUpdate {
// 		if ranking.AccountID == "" {
// 			continue
// 		}
// 		rankingsBeforeUpdate = append(rankingsBeforeUpdate, &ranking)
// 	}

// 	updatedRankings := make([]*domain.StoreRankingItem, 0)
// 	for ranking := range rankings {
// 		updatedRankings = append(updatedRankings, &ranking)
// 	}

// 	sort.Slice(updatedRankings, func(i, j int) bool {
// 		return updatedRankings[i].SocialNetworkRevenue > updatedRankings[j].SocialNetworkRevenue
// 	})

// 	for i, ranking := range updatedRankings {
// 		ranking.Position = i + 1
// 		for _, rankingBefore := range rankingsBeforeUpdate {
// 			if ranking.AccountID == rankingBefore.AccountID {
// 				ranking.PositionChange = rankingBefore.Position - ranking.Position
// 				ranking.PreviousPosition = rankingBefore.Position
// 				break
// 			}
// 		}
// 	}

// 	err := s.rankingRepo.SaveOrUpdateStoreRanking(updatedRankings)
// 	if err != nil {
// 		logrus.WithError(err).Error("Erro ao salvar top ranking de contas atualizado")
// 		return updatedRankings // Retorna mesmo com erro para não quebrar os testes
// 	}

// 	logrus.Info("Top ranking de contas atualizado")

// 	return updatedRankings
// }

// processTopRankingAccountsWithDate processa o top ranking de contas com uma data específica
func (s *TopRankingAccountsService) processTopRankingAccountsWithDate(accounts []*domain.AdAccount, processingDate time.Time) []*domain.StoreRankingItem {
	wg := sync.WaitGroup{}

	yesterday := processingDate.AddDate(0, 0, -1)
	firstDayOfMonth := getFirstDayOfMonth(yesterday)
	month := yesterday.Format("01-2006")

	rankings := make(chan domain.StoreRankingItem, len(accounts))
	rankingBeforeUpdate := make(chan domain.StoreRankingItem, len(accounts))
	for _, account := range accounts {
		wg.Add(2)

		go func(account domain.AdAccount) {
			defer wg.Done()

			// Buscar top ranking de contas anterior
			topRankingItem, err := s.rankingRepo.GetByAccountID(account.ID, month)
			if err != nil {
				logrus.WithError(err).Error("TopRankingAccountsService: Erro ao buscar top ranking de contas")
				return
			}

			if topRankingItem != nil {
				rankingBeforeUpdate <- *topRankingItem
			}
		}(*account)

		go func(account domain.AdAccount) {
			defer wg.Done()

			sales, err := s.getSalesByAccount(&account, firstDayOfMonth, yesterday)
			if err != nil {
				logrus.WithError(err).Error("TopRankingAccountsService: Erro ao buscar vendas do SSOtica")
				return
			}

			socialNetworkRevenue := ssoticadomain.GetSumNetAmountSocialNetwork(sales)

			// Criar novo item de ranking
			topRankingItem := &domain.StoreRankingItem{
				AccountID:            account.ID,
				Month:                yesterday.Format("01-2006"),
				StoreName:            account.Name,
				SocialNetworkRevenue: socialNetworkRevenue,
				Position:             0,
				PositionChange:       0,
				PreviousPosition:     0,
			}

			rankings <- *topRankingItem
		}(*account)
	}

	wg.Wait()

	close(rankings)
	close(rankingBeforeUpdate)

	rankingsBeforeUpdate := make(map[string]*domain.StoreRankingItem, 0)
	for ranking := range rankingBeforeUpdate {
		if ranking.AccountID == "" {
			continue
		}
		rankingsBeforeUpdate[ranking.AccountID] = &ranking
	}

	updatedRankings := make([]*domain.StoreRankingItem, 0)
	for ranking := range rankings {
		updatedRankings = append(updatedRankings, &ranking)
	}

	s.updatePositions(updatedRankings, rankingsBeforeUpdate)

	err := s.rankingRepo.SaveOrUpdateStoreRanking(updatedRankings)
	if err != nil {
		logrus.WithError(err).Error("Erro ao salvar top ranking de contas atualizado")
		return updatedRankings // Retorna mesmo com erro para não quebrar os testes
	}

	logrus.Info("Top ranking de contas atualizado")

	return updatedRankings
}

func (s *TopRankingAccountsService) getSalesByAccount(account *domain.AdAccount, startDate time.Time, endDate time.Time) ([]ssoticadomain.Order, error) {
	params := &ssoticadomain.GetSalesParams{
		CNPJ:       *account.CNPJ,
		SecretName: *account.SecretName,
	}

	filters := &domain.InsigthFilters{
		StartDate: &startDate,
		EndDate:   &endDate,
	}

	logrus.WithFields(logrus.Fields{
		"account_id": account.ID,
		"month":      endDate.Format("01-2006"),
		"start_date": filters.StartDate.Format(time.DateOnly),
		"end_date":   filters.EndDate.Format(time.DateOnly),
	}).Info("TopRankingAccountsService: buscando vendas do SSOtica")

	sales, err := s.ssoticaService.GetSalesByAccount(*params, filters)
	if err != nil {
		logrus.WithError(err).Error("TopRankingAccountsService: Erro ao buscar vendas do SSOtica")
		return nil, err
	}

	return sales, nil
}

func (*TopRankingAccountsService) updatePositions(
	updatedRankings []*domain.StoreRankingItem,
	rankingsBeforeUpdate map[string]*domain.StoreRankingItem,
) {
	sort.Slice(updatedRankings, func(i, j int) bool {
		return updatedRankings[i].SocialNetworkRevenue > updatedRankings[j].SocialNetworkRevenue
	})

	for i, ranking := range updatedRankings {
		ranking.Position = i + 1

		rankingBefore, exists := rankingsBeforeUpdate[ranking.AccountID]
		if exists {
			ranking.PositionChange = rankingBefore.Position - ranking.Position
			ranking.PreviousPosition = rankingBefore.Position
			continue
		}
	}
}

func (*TopRankingAccountsService) SumSocialNetworkRevenue(salesInsights []*domain.SalesInsightEntry, topRankingItem *domain.StoreRankingItem) {
	for _, salesInsight := range salesInsights {
		if salesInsight.SalesMetrics != nil {
			if socialNetworkMetrics, exists := salesInsight.SalesMetrics["SocialNetwork"]; exists {
				topRankingItem.SocialNetworkRevenue += socialNetworkMetrics.TotalRevenue
			}
		}
	}
}

// TriggerManualSync inicia manualmente uma sincronização de top ranking de contas
func (s *TopRankingAccountsService) TriggerManualSync() {
	s.syncMutex.Lock()
	if s.syncRunning {
		s.syncMutex.Unlock()
		logrus.Info("Sincronização de top ranking de contas já em andamento, ignorando solicitação manual")
		return
	}
	s.syncMutex.Unlock()

	logrus.Info("Iniciando sincronização manual de top ranking de contas")
	go s.UpdateTopRankingAccounts()
}

// GetStatus retorna o status atual do agendador
func (s *TopRankingAccountsService) GetStatus() map[string]any {
	return map[string]any{
		"sync_enabled":           s.config.SyncEnabled,
		"sync_cron":              s.config.CronSchedule,
		"last_sync_started_at":   s.lastSyncStartedAt,
		"last_sync_completed_at": s.lastSyncCompletedAt,
	}
}

func EqualDate(date1, date2 time.Time) bool {
	return date1.Year() == date2.Year() && date1.Month() == date2.Month() && date1.Day() == date2.Day()
}

func getFirstDayOfMonth(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
}

// isSecondDayOfMonth verifica se a data é o segundo dia do mês
func isSecondDayOfMonth(date time.Time) bool {
	return date.Day() == 2
}
