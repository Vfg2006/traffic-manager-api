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

// SSOticaInsightSyncConfig representa a configuração do agendador de insights do SSOtica
type SSOticaInsightSyncConfig struct {
	CronSchedule        string
	LookbackDays        int
	RequestDelaySeconds int
	MaxConcurrentJobs   int
	SyncEnabled         bool
}

// SSOticaInsightSyncService gerencia o agendamento e execução da sincronização de insights do SSOtica
type SSOticaInsightSyncService struct {
	scheduler           *gocron.Scheduler
	config              SSOticaInsightSyncConfig
	appConfig           *config.Config
	accountRepo         repository.AccountRepository
	salesInsightRepo    repository.SalesInsightRepository
	ssoticaService      insighting.SSOticaInsighter
	syncRunning         bool
	syncMutex           sync.Mutex
	lastSyncStartedAt   time.Time
	lastSyncCompletedAt time.Time
}

// NewSSOticaInsightSyncService cria uma nova instância do serviço de sincronização de insights do SSOtica
func NewSSOticaInsightSyncService(
	accountRepo repository.AccountRepository,
	salesInsightRepo repository.SalesInsightRepository,
	ssoticaService insighting.SSOticaInsighter,
	appConfig *config.Config,
) *SSOticaInsightSyncService {
	// Criar a configuração com base na config global
	insightConfig := SSOticaInsightSyncConfig{
		CronSchedule:        appConfig.SSOticaInsightSync.CronSchedule,
		LookbackDays:        appConfig.SSOticaInsightSync.LookbackDays,
		RequestDelaySeconds: appConfig.SSOticaInsightSync.RequestDelaySeconds,
		MaxConcurrentJobs:   appConfig.SSOticaInsightSync.MaxConcurrentJobs,
		SyncEnabled:         appConfig.SSOticaInsightSync.Enabled,
	}

	// Criar o agendador
	scheduler := gocron.NewScheduler(time.Local)

	logrus.WithFields(logrus.Fields{
		"cron_schedule":         insightConfig.CronSchedule,
		"lookback_days":         insightConfig.LookbackDays,
		"request_delay_seconds": insightConfig.RequestDelaySeconds,
		"max_concurrent_jobs":   insightConfig.MaxConcurrentJobs,
		"sync_enabled":          insightConfig.SyncEnabled,
	}).Info("Configuração do agendador de insights do SSOtica carregada")

	return &SSOticaInsightSyncService{
		scheduler:        scheduler,
		config:           insightConfig,
		appConfig:        appConfig,
		accountRepo:      accountRepo,
		salesInsightRepo: salesInsightRepo,
		ssoticaService:   ssoticaService,
		syncRunning:      false,
	}
}

// Start inicia o agendador
func (s *SSOticaInsightSyncService) Start(ctx context.Context) error {
	if !s.config.SyncEnabled {
		logrus.Info("Sincronização de insights do SSOtica desabilitada por configuração")
		return nil
	}

	logrus.WithField("cron", s.config.CronSchedule).Info("Iniciando agendador de sincronização de insights do SSOtica")

	// Agendar a sincronização de insights
	_, err := s.scheduler.Cron(s.config.CronSchedule).Do(func() {
		s.syncAllSSOticaInsights()
	})
	if err != nil {
		return fmt.Errorf("erro ao agendar sincronização de insights do SSOtica: %w", err)
	}

	// Executar o agendador em uma goroutine separada
	s.scheduler.StartAsync()

	// Configurar o cancelamento do agendador quando o contexto for cancelado
	go func() {
		<-ctx.Done()
		logrus.Info("Parando agendador de sincronização de insights do SSOtica")
		s.scheduler.Stop()
	}()

	return nil
}

// syncAllSSOticaInsights sincroniza os insights do SSOtica de todas as contas ativas
func (s *SSOticaInsightSyncService) syncAllSSOticaInsights() {
	s.syncMutex.Lock()
	if s.syncRunning {
		s.syncMutex.Unlock()
		logrus.Info("Sincronização de insights do SSOtica já em andamento, ignorando")
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

	logrus.Info("Iniciando sincronização de insights do SSOtica para todas as contas ativas")

	// Buscar todas as contas ativas
	activeAccounts, err := s.getActiveAccounts()
	if err != nil {
		logrus.WithError(err).Error("Erro ao buscar lista de contas para sincronização de insights do SSOtica")
		return
	}

	if len(activeAccounts) == 0 {
		logrus.Info("Nenhuma conta ativa encontrada para sincronização de insights do SSOtica")
		return
	}

	// Criar datas para processamento
	dates := s.getDatesToProcess()
	logrus.WithFields(logrus.Fields{
		"days":       s.config.LookbackDays,
		"start_date": dates[len(dates)-1].Format(time.DateOnly),
		"end_date":   dates[0].Format(time.DateOnly),
	}).Info("Período para sincronização de insights do SSOtica")

	// Processar insights
	s.processSSOticaInsightsForDates(activeAccounts, dates)

	duration := time.Since(startTime)
	logrus.WithFields(logrus.Fields{
		"duration": duration.String(),
		"accounts": len(activeAccounts),
		"days":     s.config.LookbackDays,
	}).Info("Sincronização de insights do SSOtica concluída")

	s.lastSyncCompletedAt = time.Now()
}

// getActiveAccounts busca e filtra contas ativas
func (s *SSOticaInsightSyncService) getActiveAccounts() ([]*domain.AdAccount, error) {
	accounts, err := s.accountRepo.ListAccounts([]domain.AdAccountStatus{domain.AdAccountStatusActive})
	if err != nil {
		return nil, err
	}

	if accounts == nil || len(accounts) == 0 {
		logrus.Info("Nenhuma conta encontrada para sincronização de insights do SSOtica")
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
	}).Info("Contas encontradas para sincronização de insights do SSOtica")

	return activeAccounts, nil
}

// getDatesToProcess cria um conjunto de datas para processar
func (s *SSOticaInsightSyncService) getDatesToProcess() []time.Time {
	dates := make([]time.Time, s.config.LookbackDays)
	for i := 0; i < s.config.LookbackDays; i++ {
		dates[i] = time.Now().AddDate(0, 0, -i-1) // Começar de ontem e ir para trás
	}
	return dates
}

// processSSOticaInsightsForDates processa insights do SSOtica para cada conta e todas as suas datas
func (s *SSOticaInsightSyncService) processSSOticaInsightsForDates(accounts []*domain.AdAccount, dates []time.Time) {
	// Criar um canal para controlar o número de workers concorrentes
	semaphore := make(chan struct{}, s.config.MaxConcurrentJobs)
	var wg sync.WaitGroup

	// Para cada conta, processar todas as datas em sequência
	for _, account := range accounts {
		// Verificação adicional de CNPJ e SecretName
		if account.CNPJ == nil || *account.CNPJ == "" || account.SecretName == nil || *account.SecretName == "" {
			logrus.WithField("account_id", account.ID).Warn("Conta sem CNPJ ou Token. Pulando.")
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
				"account_name": acc.Name,
				"cnpj":         *acc.CNPJ,
				"secret_name":  *acc.SecretName,
				"total_dates":  len(dates),
			}).Info("Processando insights do SSOtica para conta")

			// Processar todas as datas para esta conta
			s.processAccountForAllDates(acc, dates)
		}(account)
	}

	// Aguardar todos os workers terminarem
	wg.Wait()
}

// processAccountForAllDates processa os insights do SSOtica para uma conta em todas as datas
func (s *SSOticaInsightSyncService) processAccountForAllDates(acc *domain.AdAccount, dates []time.Time) {
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})

	// Processa uma data por vez, para APIs que não suportam ranges
	for _, date := range dates {
		s.processAccountSSOticaInsights(acc, date)

		// Aguardar antes da próxima requisição para evitar sobrecarga na API
		time.Sleep(time.Duration(s.config.RequestDelaySeconds) * time.Second)
	}
}

// processAccountSSOticaInsights processa os insights do SSOtica para uma conta e data específicas
func (s *SSOticaInsightSyncService) processAccountSSOticaInsights(acc *domain.AdAccount, date time.Time) {
	// Criar filtros para a data específica
	filters := &domain.InsigthFilters{
		StartDate: &date,
		EndDate:   &date,
	}

	logrus.WithFields(logrus.Fields{
		"account_id":   acc.ID,
		"account_name": acc.Name,
		"date":         date.Format(time.DateOnly),
		"cnpj":         *acc.CNPJ,
		"secret_name":  *acc.SecretName,
	}).Info("Obtendo insights do SSOtica para conta e data")

	// Obter insights do SSOtica para a conta e data
	salesMetrics, err := s.ssoticaService.GetSalesMetrics(*acc.CNPJ, *acc.SecretName, filters)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"account_id": acc.ID,
			"date":       date.Format(time.DateOnly),
			"error":      err.Error(),
		}).Error("Erro ao obter insights do SSOtica para conta e data")
		return
	}

	if salesMetrics == nil || len(salesMetrics) == 0 {
		logrus.WithFields(logrus.Fields{
			"account_id": acc.ID,
			"date":       date.Format(time.DateOnly),
		}).Warn("Nenhum insight do SSOtica obtido para conta e data")
		return
	}

	// Criar a entrada de insights de vendas
	salesInsightEntry := &domain.SalesInsightEntry{
		AccountID:    acc.ID,
		Date:         date,
		SalesMetrics: salesMetrics,
	}

	// Salvar no banco
	err = s.salesInsightRepo.SaveOrUpdate(salesInsightEntry)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"account_id": acc.ID,
			"date":       date.Format(time.DateOnly),
			"error":      err.Error(),
		}).Error("Erro ao salvar insights do SSOtica no banco de dados")
		return
	}

	logrus.WithFields(logrus.Fields{
		"account_id": acc.ID,
		"date":       date.Format(time.DateOnly),
	}).Info("Insights do SSOtica salvos com sucesso para conta e data")

	// Aguardar antes da próxima requisição para evitar sobrecarga na API
	time.Sleep(time.Duration(s.config.RequestDelaySeconds) * time.Second)
}

// TriggerManualSync inicia manualmente uma sincronização de insights do SSOtica
func (s *SSOticaInsightSyncService) TriggerManualSync() {
	s.syncMutex.Lock()
	if s.syncRunning {
		s.syncMutex.Unlock()
		logrus.Info("Sincronização de insights do SSOtica já em andamento, ignorando solicitação manual")
		return
	}
	s.syncMutex.Unlock()

	logrus.Info("Iniciando sincronização manual de insights do SSOtica")
	go s.syncAllSSOticaInsights()
}

// GetStatus retorna o status atual do agendador
func (s *SSOticaInsightSyncService) GetStatus() map[string]any {
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
