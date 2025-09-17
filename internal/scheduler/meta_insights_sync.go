package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/insighting"
)

// MetaInsightSyncConfig representa a configuração do agendador de insights do Meta
type MetaInsightSyncConfig struct {
	CronSchedule        string
	LookbackDays        int
	RequestDelaySeconds int
	MaxConcurrentJobs   int
	SyncEnabled         bool
}

// MetaInsightSyncService gerencia o agendamento e execução da sincronização de insights do Meta
type MetaInsightSyncService struct {
	scheduler           *gocron.Scheduler
	config              MetaInsightSyncConfig
	appConfig           *config.Config
	accountRepo         repository.AccountRepository
	adInsightRepo       repository.AdInsightRepository
	metaService         insighting.MetaInsighter
	syncRunning         bool
	syncMutex           sync.Mutex
	lastSyncStartedAt   time.Time
	lastSyncCompletedAt time.Time
}

// NewMetaInsightSyncService cria uma nova instância do serviço de sincronização de insights do Meta
func NewMetaInsightSyncService(
	accountRepo repository.AccountRepository,
	adInsightRepo repository.AdInsightRepository,
	metaService insighting.MetaInsighter,
	appConfig *config.Config,
) *MetaInsightSyncService {
	// Criar a configuração com base na config global
	insightConfig := MetaInsightSyncConfig{
		CronSchedule:        appConfig.MetaInsightSync.CronSchedule,
		LookbackDays:        appConfig.MetaInsightSync.LookbackDays,
		RequestDelaySeconds: appConfig.MetaInsightSync.RequestDelaySeconds,
		MaxConcurrentJobs:   appConfig.MetaInsightSync.MaxConcurrentJobs,
		SyncEnabled:         appConfig.MetaInsightSync.Enabled,
	}

	// Criar o agendador
	scheduler := gocron.NewScheduler(time.Local)

	logrus.WithFields(logrus.Fields{
		"cron_schedule":         insightConfig.CronSchedule,
		"lookback_days":         insightConfig.LookbackDays,
		"request_delay_seconds": insightConfig.RequestDelaySeconds,
		"max_concurrent_jobs":   insightConfig.MaxConcurrentJobs,
		"sync_enabled":          insightConfig.SyncEnabled,
	}).Info("Configuração do agendador de insights do Meta carregada")

	return &MetaInsightSyncService{
		scheduler:     scheduler,
		config:        insightConfig,
		appConfig:     appConfig,
		accountRepo:   accountRepo,
		adInsightRepo: adInsightRepo,
		metaService:   metaService,
		syncRunning:   false,
	}
}

// Start inicia o agendador
func (s *MetaInsightSyncService) Start(ctx context.Context) error {
	if !s.config.SyncEnabled {
		logrus.Info("Sincronização de insights do Meta desabilitada por configuração")
		return nil
	}

	logrus.WithField("cron", s.config.CronSchedule).Info("Iniciando agendador de sincronização de insights do Meta")

	// Agendar a sincronização de insights
	_, err := s.scheduler.Cron(s.config.CronSchedule).Do(func() {
		s.syncAllMetaInsights()
	})
	if err != nil {
		return fmt.Errorf("erro ao agendar sincronização de insights do Meta: %w", err)
	}

	// Executar o agendador em uma goroutine separada
	s.scheduler.StartAsync()

	// Configurar o cancelamento do agendador quando o contexto for cancelado
	go func() {
		<-ctx.Done()
		logrus.Info("Parando agendador de sincronização de insights do Meta")
		s.scheduler.Stop()
	}()

	return nil
}

// syncAllMetaInsights sincroniza os insights do Meta de todas as contas ativas
func (s *MetaInsightSyncService) syncAllMetaInsights() {
	s.syncMutex.Lock()
	if s.syncRunning {
		s.syncMutex.Unlock()
		logrus.Info("Sincronização de insights do Meta já em andamento, ignorando")
		return
	}
	s.syncRunning = true
	s.syncMutex.Unlock()

	startTime := time.Now()
	s.lastSyncStartedAt = startTime

	defer func() {
		s.syncMutex.Lock()
		s.syncRunning = false
		s.syncMutex.Unlock()
	}()

	logrus.Info("Iniciando sincronização de insights do Meta para todas as contas ativas")

	// Buscar todas as contas ativas
	activeAccounts, err := s.getActiveAccounts()
	if err != nil {
		logrus.WithError(err).Error("Erro ao buscar lista de contas para sincronização de insights do Meta")
		return
	}

	if len(activeAccounts) == 0 {
		logrus.Info("Nenhuma conta ativa encontrada para sincronização de insights do Meta")
		return
	}

	// Criar datas para processamento
	dates := s.getDatesToProcess()
	logrus.WithFields(logrus.Fields{
		"days":       s.config.LookbackDays,
		"start_date": dates[len(dates)-1].Format(time.DateOnly),
		"end_date":   dates[0].Format(time.DateOnly),
	}).Info("Período para sincronização de insights do Meta")

	// Processar insights
	s.processMetaInsightsForDates(activeAccounts, dates)

	duration := time.Since(startTime)
	logrus.WithFields(logrus.Fields{
		"duration": duration.String(),
		"accounts": len(activeAccounts),
		"days":     s.config.LookbackDays,
	}).Info("Sincronização de insights do Meta concluída")

	s.lastSyncCompletedAt = time.Now()
}

// getActiveAccounts busca e filtra contas ativas
func (s *MetaInsightSyncService) getActiveAccounts() ([]*domain.AdAccount, error) {
	activeAccounts, err := s.accountRepo.ListAccounts([]domain.AdAccountStatus{domain.AdAccountStatusActive})
	if err != nil {
		return nil, err
	}

	if activeAccounts == nil || len(activeAccounts) == 0 {
		logrus.Info("Nenhuma conta encontrada para sincronização de insights do Meta")
		return []*domain.AdAccount{}, nil
	}

	logrus.WithFields(logrus.Fields{
		"active_accounts": len(activeAccounts),
	}).Info("Contas encontradas para sincronização de insights do Meta")

	return activeAccounts, nil
}

// getDatesToProcess cria um conjunto de datas para processar
func (s *MetaInsightSyncService) getDatesToProcess() []time.Time {
	dates := make([]time.Time, s.config.LookbackDays)
	for i := 0; i < s.config.LookbackDays; i++ {
		dates[i] = time.Now().AddDate(0, 0, -i-1) // Começar de ontem e ir para trás
	}
	return dates
}

// processMetaInsightsForDates processa insights do Meta para cada conta e todas as suas datas
func (s *MetaInsightSyncService) processMetaInsightsForDates(accounts []*domain.AdAccount, dates []time.Time) {
	// Criar um canal para controlar o número de workers concorrentes
	semaphore := make(chan struct{}, s.config.MaxConcurrentJobs)
	var wg sync.WaitGroup

	// Para cada conta, processar todas as datas em sequência
	for _, account := range accounts {
		// Se a conta não tiver external_id, pular
		if account.ExternalID == "" {
			logrus.WithField("account_id", account.ID).Warn("Conta sem external_id. Pulando.")
			continue
		}

		// Adicionar uma tarefa ao grupo de espera
		wg.Add(1)
		semaphore <- struct{}{} // Adquirir semáforo

		go func(acc *domain.AdAccount) {
			defer func() {
				<-semaphore // Liberar semáforo
				wg.Done()
			}()

			logrus.WithFields(logrus.Fields{
				"account_id":   acc.ID,
				"external_id":  acc.ExternalID,
				"account_name": acc.Name,
				"total_dates":  len(dates),
			}).Info("Processando insights do Meta para conta")

			// Processar todas as datas para esta conta
			s.processAccountForAllDates(acc, dates)
		}(account)
	}

	// Aguardar todos os workers terminarem
	wg.Wait()
}

// processAccountForAllDates processa os insights do Meta para uma conta em todas as datas
func (s *MetaInsightSyncService) processAccountForAllDates(acc *domain.AdAccount, dates []time.Time) {
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})

	for _, date := range dates {
		s.processAccountMetaInsights(acc, date)

		// Aguardar antes da próxima requisição para evitar sobrecarga na API
		time.Sleep(time.Duration(s.config.RequestDelaySeconds) * time.Second)
	}
}

// processAccountMetaInsights processa os insights do Meta para uma conta e data específicas
func (s *MetaInsightSyncService) processAccountMetaInsights(acc *domain.AdAccount, date time.Time) {
	// Criar filtros para a data específica
	filters := &domain.InsigthFilters{
		StartDate: &date,
		EndDate:   &date,
	}

	logrus.WithFields(logrus.Fields{
		"account_id":   acc.ID,
		"external_id":  acc.ExternalID,
		"account_name": acc.Name,
		"date":         date.Format(time.DateOnly),
	}).Info("Obtendo insights do Meta para conta e data")

	// Obter insights do Meta para a conta e data
	adMetrics, err := s.metaService.GetAdAccountMetrics(acc.ExternalID, filters)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"account_id":  acc.ID,
			"external_id": acc.ExternalID,
			"date":        date.Format(time.DateOnly),
			"error":       err.Error(),
		}).Error("Erro ao obter insights do Meta para conta e data")
		return
	}

	if adMetrics == nil {
		logrus.WithFields(logrus.Fields{
			"account_id":  acc.ID,
			"external_id": acc.ExternalID,
			"date":        date.Format(time.DateOnly),
		}).Warn("Nenhum insight do Meta obtido para conta e data")
		return
	}

	// Criar a entrada de insights de anúncios
	adInsightEntry := &domain.AdInsightEntry{
		AccountID:  acc.ID,
		ExternalID: acc.ExternalID,
		Date:       date,
		AdMetrics:  adMetrics,
	}

	// Salvar no banco
	err = s.adInsightRepo.SaveOrUpdate(adInsightEntry)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"account_id":  acc.ID,
			"external_id": acc.ExternalID,
			"date":        date.Format(time.DateOnly),
			"error":       err.Error(),
		}).Error("Erro ao salvar insights do Meta no banco de dados")
		return
	}

	logrus.WithFields(logrus.Fields{
		"account_id":  acc.ID,
		"external_id": acc.ExternalID,
		"date":        date.Format(time.DateOnly),
	}).Info("Insights do Meta salvos com sucesso para conta e data")

	// Aguardar antes da próxima requisição para evitar sobrecarga na API
	time.Sleep(time.Duration(s.config.RequestDelaySeconds) * time.Second)
}

// TriggerManualSync inicia manualmente uma sincronização de insights do Meta
func (s *MetaInsightSyncService) TriggerManualSync() {
	s.syncMutex.Lock()
	if s.syncRunning {
		s.syncMutex.Unlock()
		logrus.Info("Sincronização de insights do Meta já em andamento, ignorando solicitação manual")
		return
	}
	s.syncMutex.Unlock()

	logrus.Info("Iniciando sincronização manual de insights do Meta")
	go s.syncAllMetaInsights()
}

// GetStatus retorna o status atual do agendador
func (s *MetaInsightSyncService) GetStatus() map[string]any {
	return map[string]any{
		"sync_enabled":           s.config.SyncEnabled,
		"sync_cron":              s.config.CronSchedule,
		"sync_lookback_days":     s.config.LookbackDays,
		"sync_max_concurrent":    s.config.MaxConcurrentJobs,
		"sync_request_delay_s":   s.config.RequestDelaySeconds,
		"retention_policy":       "dados mantidos permanentemente",
		"last_sync_started_at":   s.lastSyncStartedAt,
		"last_sync_completed_at": s.lastSyncCompletedAt,
	}
}
