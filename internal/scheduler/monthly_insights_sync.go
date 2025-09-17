package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/insighting"
)

// MonthlyInsightsSyncConfig representa a configuração do agendador de insights mensais
type MonthlyInsightsSyncConfig struct {
	CronSchedule        string
	RequestDelaySeconds int
	MaxConcurrentJobs   int
	SyncEnabled         bool
	MonthLookBack       int
}

// MonthlyInsightsSyncService gerencia o agendamento e execução da sincronização mensal de insights
type MonthlyInsightsSyncService struct {
	scheduler               *gocron.Scheduler
	config                  MonthlyInsightsSyncConfig
	appConfig               *config.Config
	accountRepo             repository.AccountRepository
	monthlyAdInsightRepo    repository.MonthlyAdInsightRepository
	monthlySalesInsightRepo repository.MonthlySalesInsightRepository
	metaService             insighting.MetaInsighter
	ssoticaService          insighting.SSOticaInsighter
	syncRunning             bool
	syncMutex               sync.Mutex
	lastSyncStartedAt       time.Time
	lastSyncCompletedAt     time.Time
}

// NewMonthlyInsightsSyncService cria uma nova instância do serviço de sincronização mensal de insights
func NewMonthlyInsightsSyncService(
	accountRepo repository.AccountRepository,
	monthlyAdInsightRepo repository.MonthlyAdInsightRepository,
	monthlySalesInsightRepo repository.MonthlySalesInsightRepository,
	metaService insighting.MetaInsighter,
	ssoticaService insighting.SSOticaInsighter,
	appConfig *config.Config,
) *MonthlyInsightsSyncService {
	// Criar a configuração com base na config global
	insightConfig := MonthlyInsightsSyncConfig{
		CronSchedule:        appConfig.MonthlyInsightsSync.CronSchedule,
		RequestDelaySeconds: appConfig.MonthlyInsightsSync.RequestDelaySeconds,
		MaxConcurrentJobs:   appConfig.MonthlyInsightsSync.MaxConcurrentJobs,
		SyncEnabled:         appConfig.MonthlyInsightsSync.Enabled,
		MonthLookBack:       appConfig.MonthlyInsightsSync.MonthLookBack,
	}

	// Criar o agendador
	scheduler := gocron.NewScheduler(time.Local)

	logrus.WithFields(logrus.Fields{
		"cron_schedule":         insightConfig.CronSchedule,
		"request_delay_seconds": insightConfig.RequestDelaySeconds,
		"max_concurrent_jobs":   insightConfig.MaxConcurrentJobs,
		"sync_enabled":          insightConfig.SyncEnabled,
	}).Info("Configuração do agendador de insights mensais carregada")

	return &MonthlyInsightsSyncService{
		scheduler:               scheduler,
		config:                  insightConfig,
		appConfig:               appConfig,
		accountRepo:             accountRepo,
		monthlyAdInsightRepo:    monthlyAdInsightRepo,
		monthlySalesInsightRepo: monthlySalesInsightRepo,
		metaService:             metaService,
		ssoticaService:          ssoticaService,
		syncRunning:             false,
	}
}

// Start inicia o agendador
func (s *MonthlyInsightsSyncService) Start(ctx context.Context) error {
	if !s.config.SyncEnabled {
		logrus.Info("Sincronização mensal de insights desabilitada por configuração")
		return nil
	}

	logrus.WithField("cron", s.config.CronSchedule).Info("Iniciando agendador de sincronização mensal de insights")

	// Agendar a sincronização de insights
	_, err := s.scheduler.Cron(s.config.CronSchedule).Do(func() {
		s.syncMonthlyInsights()
	})
	if err != nil {
		return fmt.Errorf("erro ao agendar sincronização mensal de insights: %w", err)
	}

	// Executar o agendador em uma goroutine separada
	s.scheduler.StartAsync()

	// Configurar o cancelamento do agendador quando o contexto for cancelado
	go func() {
		<-ctx.Done()
		logrus.Info("Parando agendador de sincronização mensal de insights")
		s.scheduler.Stop()
	}()

	return nil
}

// syncMonthlyInsights sincroniza os insights mensais de todas as contas ativas
func (s *MonthlyInsightsSyncService) syncMonthlyInsights() {
	s.syncMutex.Lock()
	if s.syncRunning {
		s.syncMutex.Unlock()
		logrus.Info("Sincronização mensal de insights já em andamento, ignorando")
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

	logrus.Info("Iniciando sincronização mensal de insights para todas as contas ativas")

	// Buscar todas as contas ativas
	activeAccounts, err := s.getActiveAccounts()
	if err != nil {
		logrus.WithError(err).Error("Erro ao buscar lista de contas para sincronização mensal de insights")
		return
	}

	if len(activeAccounts) == 0 {
		logrus.Info("Nenhuma conta ativa encontrada para sincronização mensal de insights")
		return
	}

	for i := 1; i <= s.config.MonthLookBack; i++ {
		now := time.Now()
		month := now.AddDate(0, -i, 0)
		firstDayOfMonth := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
		lastDayOfMonth := time.Date(month.Year(), month.Month()+1, 1, 0, 0, 0, 0, month.Location()).AddDate(0, 0, -1)

		logrus.WithFields(logrus.Fields{
			"start_date": firstDayOfMonth.Format(time.DateOnly),
			"end_date":   lastDayOfMonth.Format(time.DateOnly),
		}).Info("Período para sincronização mensal de insights")

		s.processMonthlyInsights(activeAccounts, firstDayOfMonth, lastDayOfMonth)
	}

	duration := time.Since(startTime)
	logrus.WithFields(logrus.Fields{
		"duration": duration.String(),
		"accounts": len(activeAccounts),
	}).Info("Sincronização mensal de insights concluída")

	s.lastSyncCompletedAt = time.Now()
}

// getActiveAccounts busca e filtra contas ativas
func (s *MonthlyInsightsSyncService) getActiveAccounts() ([]*domain.AdAccount, error) {
	activeAccounts, err := s.accountRepo.ListAccounts([]domain.AdAccountStatus{domain.AdAccountStatusActive})
	if err != nil {
		return nil, err
	}

	if activeAccounts == nil || len(activeAccounts) == 0 {
		logrus.Info("Nenhuma conta encontrada para sincronização mensal de insights")
		return []*domain.AdAccount{}, nil
	}

	logrus.WithFields(logrus.Fields{
		"active_accounts": len(activeAccounts),
	}).Info("Contas encontradas para sincronização mensal de insights")

	return activeAccounts, nil
}

// processMonthlyInsights processa os insights mensais para todas as contas
func (s *MonthlyInsightsSyncService) processMonthlyInsights(accounts []*domain.AdAccount, startDate, endDate time.Time) {
	// Criar um canal para controlar o número de workers concorrentes
	semaphore := make(chan struct{}, s.config.MaxConcurrentJobs)
	var wg sync.WaitGroup

	// Para cada conta, processar as métricas mensais
	for _, account := range accounts {

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
				"start_date":   startDate.Format(time.DateOnly),
				"end_date":     endDate.Format(time.DateOnly),
			}).Info("Processando insights mensais para conta")

			// Criar filtros com as datas do mês anterior
			filters := &domain.InsigthFilters{
				StartDate: &startDate,
				EndDate:   &endDate,
			}

			// Processar métricas de anúncios do mês anterior
			err := s.processMonthlyAdMetrics(acc, filters)
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"account_id":  acc.ID,
					"external_id": acc.ExternalID,
					"start_date":  startDate.Format(time.DateOnly),
					"end_date":    endDate.Format(time.DateOnly),
				}).Error("Erro ao processar métricas mensais de anúncios")
			}

			// Processar métricas de vendas do mês anterior se a conta tiver os dados necessários
			if acc.CNPJ != nil && *acc.CNPJ != "" && acc.SecretName != nil && *acc.SecretName != "" {
				err = s.processMonthlySalesMetrics(acc, filters)
				if err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{
						"account_id":  acc.ID,
						"cnpj":        *acc.CNPJ,
						"secret_name": *acc.SecretName,
						"start_date":  startDate.Format(time.DateOnly),
						"end_date":    endDate.Format(time.DateOnly),
					}).Error("Erro ao processar métricas mensais de vendas")
				}
			}

			// Aguardar antes da próxima conta para evitar excesso de requisições
			time.Sleep(time.Duration(s.config.RequestDelaySeconds) * time.Second)
		}(account)
	}

	// Aguardar todos os workers terminarem
	wg.Wait()
}

// processMonthlyAdMetrics processa as métricas mensais de anúncios para uma conta
func (s *MonthlyInsightsSyncService) processMonthlyAdMetrics(acc *domain.AdAccount, filters *domain.InsigthFilters) error {
	if acc.ExternalID == "" {
		return fmt.Errorf("conta sem ID externo")
	}

	// Buscar métricas de anúncios diretamente via API
	adMetrics, err := s.metaService.GetAdAccountMetrics(acc.ExternalID, filters)
	if err != nil {
		return fmt.Errorf("erro ao obter métricas de anúncios: %w", err)
	}

	if adMetrics == nil {
		return fmt.Errorf("nenhuma métrica de anúncios encontrada")
	}

	// Extrair mês e ano da data e formatar o período
	startDate := *filters.StartDate
	period := fmt.Sprintf("%02d-%04d", int(startDate.Month()), startDate.Year())

	// Criar entrada mensal
	monthlyInsight := &domain.MonthlyAdInsightEntry{
		AccountID:  acc.ID,
		ExternalID: acc.ExternalID,
		Period:     period,
		AdMetrics:  adMetrics,
	}

	// Salvar no banco de dados
	err = s.monthlyAdInsightRepo.SaveOrUpdate(monthlyInsight)
	if err != nil {
		return fmt.Errorf("erro ao salvar métricas mensais de anúncios: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"account_id":  acc.ID,
		"external_id": acc.ExternalID,
		"period":      period,
	}).Info("Métricas mensais de anúncios salvas com sucesso")

	return nil
}

// processMonthlySalesMetrics processa as métricas mensais de vendas para uma conta
func (s *MonthlyInsightsSyncService) processMonthlySalesMetrics(acc *domain.AdAccount, filters *domain.InsigthFilters) error {
	if acc.CNPJ == nil || *acc.CNPJ == "" || acc.SecretName == nil || *acc.SecretName == "" {
		return fmt.Errorf("conta sem CNPJ ou SecretName")
	}

	// Buscar métricas de vendas diretamente via API
	salesMetrics, err := s.ssoticaService.GetSalesMetrics(*acc.CNPJ, *acc.SecretName, filters)
	if err != nil {
		return fmt.Errorf("erro ao obter métricas de vendas: %w", err)
	}

	if salesMetrics == nil {
		return fmt.Errorf("nenhuma métrica de vendas encontrada")
	}

	// Extrair mês e ano da data e formatar o período
	startDate := *filters.StartDate
	period := fmt.Sprintf("%02d-%04d", int(startDate.Month()), startDate.Year())

	// Criar entrada mensal
	monthlyInsight := &domain.MonthlySalesInsightEntry{
		AccountID:    acc.ID,
		Period:       period,
		SalesMetrics: salesMetrics,
	}

	// Salvar no banco de dados
	err = s.monthlySalesInsightRepo.SaveOrUpdate(monthlyInsight)
	if err != nil {
		return fmt.Errorf("erro ao salvar métricas mensais de vendas: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"account_id":  acc.ID,
		"cnpj":        *acc.CNPJ,
		"secret_name": *acc.SecretName,
		"period":      period,
	}).Info("Métricas mensais de vendas salvas com sucesso")

	return nil
}

// TriggerManualSync inicia manualmente uma sincronização de insights mensais
func (s *MonthlyInsightsSyncService) TriggerManualSync() {
	s.syncMutex.Lock()
	if s.syncRunning {
		s.syncMutex.Unlock()
		logrus.Info("Sincronização mensal de insights já em andamento, ignorando solicitação manual")
		return
	}
	s.syncMutex.Unlock()

	logrus.Info("Iniciando sincronização manual de insights mensais")
	go s.syncMonthlyInsights()
}

// GetStatus retorna o status atual da sincronização
func (s *MonthlyInsightsSyncService) GetStatus() map[string]any {
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()

	return map[string]any{
		"sync_running":           s.syncRunning,
		"sync_cron":              s.config.CronSchedule,
		"sync_enabled":           s.config.SyncEnabled,
		"last_sync_started_at":   s.lastSyncStartedAt,
		"last_sync_completed_at": s.lastSyncCompletedAt,
	}
}
