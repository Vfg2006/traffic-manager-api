package insighting

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica"
	ssoticadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/domain"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/pkg/utils"
)

// Estrutura para acompanhar as origens e fazer a agregação correta
type originAggregator struct {
	totalRevenue  float64
	salesQuantity int
	sales         []*domain.Sale
}

// Service implementa tanto a interface Insighter quanto MetaInsighter e SSOticaInsighter
type Service struct {
	cfg                           *config.Config
	metaService                   *meta.MetaIntegrator
	ssoticaService                ssotica.SSOticaIntegrator
	accountRepository             repository.AccountRepository
	adInsightRepository           repository.AdInsightRepository
	salesInsightRepository        repository.SalesInsightRepository
	monthlyAdInsightRepository    repository.MonthlyAdInsightRepository
	monthlySalesInsightRepository repository.MonthlySalesInsightRepository
	useCache                      bool
}

// NewService cria uma nova instância do serviço de insights
func NewService(
	cfg *config.Config,
	metaService *meta.MetaIntegrator,
	ssoticaService ssotica.SSOticaIntegrator,
	accountRepo repository.AccountRepository,
) CombinedInsighter {
	return &Service{
		cfg:                    cfg,
		metaService:            metaService,
		ssoticaService:         ssoticaService,
		accountRepository:      accountRepo,
		adInsightRepository:    nil,   // Inicialmente null
		salesInsightRepository: nil,   // Inicialmente null
		useCache:               false, // Inicialmente não usa cache
	}
}

// WithCache habilita o uso de cache de insights
func (s *Service) WithCache(
	adInsightRepo repository.AdInsightRepository,
	salesInsightRepo repository.SalesInsightRepository,
	monthlyAdInsightRepo repository.MonthlyAdInsightRepository,
	monthlySalesInsightRepo repository.MonthlySalesInsightRepository,
) *Service {
	s.adInsightRepository = adInsightRepo
	s.salesInsightRepository = salesInsightRepo
	s.monthlyAdInsightRepository = monthlyAdInsightRepo
	s.monthlySalesInsightRepository = monthlySalesInsightRepo
	s.useCache = (s.adInsightRepository != nil && s.salesInsightRepository != nil)
	return s
}

// GetAdAccountsByID obtém todas as métricas (anúncios e vendas) para uma conta específica
func (s *Service) GetAdAccountsByID(accountID string, filters *domain.InsigthFilters) (*domain.AdAccountInsightsResponse, error) {
	// Verificar se os filtros têm datas válidas
	if filters == nil || filters.StartDate == nil || filters.EndDate == nil {
		return nil, fmt.Errorf("é necessário informar as datas de início e fim")
	}

	// Validar se as datas estão em ordem
	if filters.StartDate.After(*filters.EndDate) {
		return nil, fmt.Errorf("a data de início não pode ser posterior à data de fim")
	}

	// Buscar a conta do repositório para obter o ID interno, CNPJ e SecretName
	account, err := s.accountRepository.GetAccountByExternalID(accountID)
	if err != nil {
		logrus.Error("Erro ao buscar conta pelo ID no repositório", map[string]any{
			"accountID": accountID,
			"error":     err,
		})
		return nil, err
	}

	if account == nil {
		return nil, fmt.Errorf("conta não encontrada: %s", accountID)
	}

	// Criar a resposta final
	insights := &domain.AdAccountInsightsResponse{
		Filters: filters,
	}

	// Se o cache estiver habilitado, tentar buscar as métricas do banco primeiro
	if s.useCache {
		return s.GetAdAccountsByIDWithCache(insights, account, accountID, filters)
	}

	return s.GetAdAccountsByIDWithoutCache(insights, account, accountID, filters)
}

// GetAdAccountsByID obtém todas as métricas (anúncios e vendas) para uma conta específica
func (s *Service) GetAdAccountsByIDWithCache(insights *domain.AdAccountInsightsResponse,
	account *domain.AdAccount,
	accountExternalID string,
	filters *domain.InsigthFilters,
) (*domain.AdAccountInsightsResponse, error) {
	// Gerar lista de todas as datas do período solicitado para controle
	allDates := generateDateRange(filters.StartDate, filters.EndDate)
	if len(allDates) == 0 {
		return nil, fmt.Errorf("período de datas inválido")
	}

	// Variáveis para armazenar os resultados
	var (
		adInsights     []*domain.AdInsightEntry
		salesInsights  []*domain.SalesInsightEntry
		adInsightError error
		salesError     error
	)

	// Usar WaitGroup para esperar as goroutines terminarem
	wg := sync.WaitGroup{}
	wg.Add(2)

	// Goroutine para buscar e processar métricas de anúncios
	go func() {
		defer wg.Done()
		adInsights, adInsightError = s.getAdMetricsWithCache(account, accountExternalID, filters, allDates)
	}()

	// Goroutine para buscar e processar métricas de vendas (apenas se a conta tiver os dados necessários)
	go func() {
		defer wg.Done()
		if account.CNPJ != nil && *account.CNPJ != "" && account.SecretName != nil && *account.SecretName != "" {
			salesInsights, salesError = s.getSalesMetricsWithCache(account, filters, allDates)
		}
	}()

	// Aguardar as goroutines terminarem
	wg.Wait()

	// Verificar se houve erro nas goroutines
	if adInsightError != nil {
		logrus.WithError(adInsightError).Error("Erro ao buscar métricas de anúncios com cache")
	}

	if salesError != nil {
		logrus.WithError(salesError).Error("Erro ao buscar métricas de vendas com cache")
	}

	// Combinar todos os insights de anúncios
	if len(adInsights) > 0 {
		// Agregar todas as métricas de anúncios
		combinedAdMetrics := combineAdMetrics(adInsights)
		insights.AdAccountMetrics = combinedAdMetrics
	}

	// Combinar todos os insights de vendas
	if len(salesInsights) > 0 {
		// Agregar todas as métricas de vendas
		combinedSalesMetrics := combineSalesMetrics(salesInsights)
		insights.SalesMetrics = combinedSalesMetrics
	}

	// Se conseguimos dados tanto de anúncios quanto de vendas, calcular métricas de resultado
	if insights.AdAccountMetrics != nil && insights.SalesMetrics != nil && insights.SalesMetrics[domain.SocialNetwork] != nil {
		insights.ResultMetrics = domain.CalculateResultMetrics(
			insights.AdAccountMetrics,
			insights.SalesMetrics,
		)
	}

	// Se encontramos dados suficientes, retornar
	if insights.AdAccountMetrics != nil || insights.SalesMetrics != nil {
		return insights, nil
	}

	return insights, nil
}

// getAdMetricsWithCache busca métricas de anúncios do cache e preenche dados faltantes via API
func (s *Service) getAdMetricsWithCache(
	account *domain.AdAccount,
	accountExternalID string,
	filters *domain.InsigthFilters,
	allDates []time.Time,
) ([]*domain.AdInsightEntry, error) {
	// Mapa para armazenar as datas que já temos no banco
	existingAdDates := make(map[string]bool)

	// Armazenar os insights encontrados
	adInsights := make([]*domain.AdInsightEntry, 0)

	// 1. Buscar todos os insights de anúncios para o período completo
	periodAdInsights, err := s.adInsightRepository.GetByDateRange(
		account.ID,
		*filters.StartDate,
		*filters.EndDate,
	)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"account_id": account.ID,
			"start_date": filters.StartDate.Format(time.DateOnly),
			"end_date":   filters.EndDate.Format(time.DateOnly),
		}).Warn("Erro ao buscar insights de anúncios do banco de dados para o período")
	} else {
		// Adicionar aos insights encontrados e marcar as datas que já temos
		for _, insight := range periodAdInsights {
			adInsights = append(adInsights, insight)
			dateStr := insight.Date.Format(time.DateOnly)
			existingAdDates[dateStr] = true
		}
	}

	// 2. Determinar quais datas estão faltando para buscar das APIs
	var missingAdDates []time.Time

	for _, date := range allDates {
		dateStr := date.Format(time.DateOnly)

		// Verificar se temos dados de anúncios para esta data
		if !existingAdDates[dateStr] {
			missingAdDates = append(missingAdDates, date)
		}
	}

	// 3. Se temos datas faltantes de anúncios, buscá-las da API do Meta
	if len(missingAdDates) > 0 {
		logrus.WithFields(logrus.Fields{
			"account_id":    account.ID,
			"external_id":   accountExternalID,
			"missing_dates": len(missingAdDates),
			"total_dates":   len(allDates),
			"first_missing": missingAdDates[0].Format(time.DateOnly),
			"last_missing":  missingAdDates[len(missingAdDates)-1].Format(time.DateOnly),
		}).Info("Buscando insights de anúncios da API para datas faltantes")

		// Definir o número máximo de goroutines simultâneas
		const maxConcurrent = 5
		semaphore := make(chan struct{}, maxConcurrent)

		// Usar WaitGroup para esperar todas as chamadas à API terminarem
		var fetchWg sync.WaitGroup

		// Mutex para proteger o slice de salesInsights durante atualizações concorrentes
		var mutex sync.Mutex

		for _, date := range missingAdDates {
			fetchWg.Add(1)

			// Função para buscar dados para uma data específica
			go func(date time.Time) {
				defer fetchWg.Done()

				// Adquirir uma vaga no semáforo
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				dailyFilter := &domain.InsigthFilters{
					StartDate: &date,
					EndDate:   &date,
				}

				logrus.WithFields(logrus.Fields{
					"account_id":  account.ID,
					"external_id": accountExternalID,
					"start_date":  dailyFilter.StartDate.Format(time.DateOnly),
					"end_date":    dailyFilter.EndDate.Format(time.DateOnly),
				}).Info("Buscando insights de anúncios da API para datas faltantes")

				// Buscar da API do Meta
				adMetrics, err := s.metaService.GetAdAccountsInsights(accountExternalID, dailyFilter)
				if err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{
						"account_id":  account.ID,
						"external_id": accountExternalID,
						"start_date":  dailyFilter.StartDate.Format(time.DateOnly),
						"end_date":    dailyFilter.EndDate.Format(time.DateOnly),
					}).Warn("Erro ao obter insights de anúncios do Meta")
					return
				}

				logrus.WithFields(logrus.Fields{
					"ad_metrics": adMetrics,
				}).Info("Insights de anúncios obtidos da API do Meta")

				// Criar entrada para o cache
				adInsight := &domain.AdInsightEntry{
					AccountID:  account.ID,
					ExternalID: accountExternalID,
					Date:       *dailyFilter.StartDate,
					AdMetrics:  adMetrics,
				}

				if date.Format(time.DateOnly) != time.Now().Format(time.DateOnly) {
					err = s.adInsightRepository.SaveOrUpdate(adInsight)
					if err != nil {
						logrus.WithError(err).WithFields(logrus.Fields{
							"account_id": account.ID,
						}).Warn("Erro ao salvar insights de anúncios no banco de dados")
					}
				}

				// Adicionar aos insights encontrados - protegido por mutex
				mutex.Lock()
				adInsights = append(adInsights, adInsight)
				mutex.Unlock()
			}(date)
		}

		// Aguardar todas as goroutines terminarem
		fetchWg.Wait()
	}

	return adInsights, nil
}

// getSalesMetricsWithCache busca métricas de vendas do cache e preenche dados faltantes via API
func (s *Service) getSalesMetricsWithCache(
	account *domain.AdAccount,
	filters *domain.InsigthFilters,
	allDates []time.Time,
) ([]*domain.SalesInsightEntry, error) {
	// Mapa para armazenar as datas que já temos no banco
	existingSalesDates := make(map[string]bool)

	// Armazenar os insights encontrados
	salesInsights := make([]*domain.SalesInsightEntry, 0)

	// 1. Buscar todos os insights de vendas para o período completo
	periodSalesInsights, err := s.salesInsightRepository.GetByDateRange(
		account.ID,
		*filters.StartDate,
		*filters.EndDate,
	)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"account_id": account.ID,
			"start_date": filters.StartDate.Format(time.DateOnly),
			"end_date":   filters.EndDate.Format(time.DateOnly),
		}).Warn("Erro ao buscar insights de vendas do banco de dados para o período")
	} else {
		// Adicionar aos insights encontrados e marcar as datas que já temos
		for _, insight := range periodSalesInsights {
			salesInsights = append(salesInsights, insight)
			dateStr := insight.Date.Format(time.DateOnly)
			existingSalesDates[dateStr] = true
		}
	}

	// 2. Determinar quais datas estão faltando para buscar das APIs
	var missingSalesDates []time.Time

	for _, date := range allDates {
		dateStr := date.Format(time.DateOnly)

		// Verificar se temos dados de vendas para esta data
		if !existingSalesDates[dateStr] {
			missingSalesDates = append(missingSalesDates, date)
		}
	}

	// 3. Se temos datas faltantes de vendas, buscá-las da API do SSOtica
	if len(missingSalesDates) > 0 && account.CNPJ != nil && *account.CNPJ != "" && account.SecretName != nil && *account.SecretName != "" {
		logrus.WithFields(logrus.Fields{
			"account_id":    account.ID,
			"missing_dates": len(missingSalesDates),
			"total_dates":   len(allDates),
			"first_missing": missingSalesDates[0].Format(time.DateOnly),
			"last_missing":  missingSalesDates[len(missingSalesDates)-1].Format(time.DateOnly),
		}).Info("Buscando insights de vendas da API para datas faltantes")

		// Definir o número máximo de goroutines simultâneas
		const maxConcurrent = 5
		semaphore := make(chan struct{}, maxConcurrent)

		// Usar WaitGroup para esperar todas as chamadas à API terminarem
		var fetchWg sync.WaitGroup

		// Mutex para proteger o slice de salesInsights durante atualizações concorrentes
		var mutex sync.Mutex

		// Configurar os parâmetros base para a chamada ao SSOtica
		params := &ssoticadomain.GetSalesParams{
			CNPJ:       *account.CNPJ,
			SecretName: *account.SecretName,
		}

		// Buscar cada data faltante da API em paralelo
		for _, date := range missingSalesDates {
			fetchWg.Add(1)

			// Função para buscar dados para uma data específica
			go func(date time.Time, baseParams ssoticadomain.GetSalesParams) {
				defer fetchWg.Done()

				// Adquirir uma vaga no semáforo
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				dailyFilter := &domain.InsigthFilters{
					StartDate: &date,
					EndDate:   &date,
				}

				// Buscar da API do SSOtica
				salesMetrics, err := s.GetSalesMetrics(*account.CNPJ, *account.SecretName, dailyFilter)
				if err != nil {
					logrus.Warn("Erro ao obter dados de vendas do SSOtica", map[string]any{
						"accountID": account.ID,
						"error":     err,
					})
					return
				}

				if salesMetrics == nil || len(salesMetrics) == 0 {
					logrus.Warn("Erro ao obter dados de vendas do SSOtica", map[string]any{
						"accountID": account.ID,
						"error":     err,
					})
					return
				}

				// Criar entrada para o cache
				salesInsight := &domain.SalesInsightEntry{
					AccountID:    account.ID,
					Date:         date,
					SalesMetrics: salesMetrics,
				}

				// Salvar no cache
				if date.Format(time.DateOnly) != time.Now().Format(time.DateOnly) {
					err = s.salesInsightRepository.SaveOrUpdate(salesInsight)
					if err != nil {
						logrus.WithError(err).WithFields(logrus.Fields{
							"account_id": account.ID,
							"date":       date.Format(time.DateOnly),
						}).Warn("Erro ao salvar insights de vendas no banco de dados")
					}
				}

				// Adicionar aos insights encontrados - protegido por mutex
				mutex.Lock()
				salesInsights = append(salesInsights, salesInsight)
				mutex.Unlock()
			}(date, *params)
		}

		// Aguardar todas as goroutines terminarem
		fetchWg.Wait()
	}

	return salesInsights, nil
}

func (s *Service) GetAdAccountsByIDWithoutCache(
	insights *domain.AdAccountInsightsResponse,
	account *domain.AdAccount,
	accountExternalID string,
	filters *domain.InsigthFilters,
) (*domain.AdAccountInsightsResponse, error) {
	// Se não conseguimos dados do cache ou se o cache não está habilitado, buscar diretamente das APIs
	// Este é o comportamento original da função
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		adAccountMetrics, err := s.metaService.GetAdAccountsInsights(accountExternalID, filters)
		if err != nil {
			logrus.Warn("Erro ao obter insights de anúncios do Meta", map[string]any{
				"accountID": accountExternalID,
				"error":     err,
			})
			return
		}

		insights.AdAccountMetrics = adAccountMetrics
	}()

	params := &ssoticadomain.GetSalesParams{}
	if account.CNPJ != nil && account.SecretName != nil {
		params.CNPJ = *account.CNPJ
		params.SecretName = *account.SecretName

		go func(params ssoticadomain.GetSalesParams) {
			defer wg.Done()

			salesMetrics, err := s.GetSalesMetrics(*account.CNPJ, *account.SecretName, filters)
			if err != nil {
				logrus.Warn("Erro ao obter dados de vendas do SSOtica", map[string]any{
					"accountID": accountExternalID,
					"error":     err,
				})
				return
			}

			if salesMetrics == nil || len(salesMetrics) == 0 {
				logrus.Warn("Erro ao obter dados de vendas do SSOtica", map[string]any{
					"accountID": accountExternalID,
					"error":     err,
				})
				return
			}

			insights.SalesMetrics = salesMetrics
		}(*params)
	} else {
		wg.Done()
	}

	wg.Wait()

	// Calcular métricas de resultado se temos dados suficientes
	if insights.SalesMetrics != nil && insights.SalesMetrics[domain.SocialNetwork] != nil &&
		insights.AdAccountMetrics != nil {
		insights.ResultMetrics = domain.CalculateResultMetrics(
			insights.AdAccountMetrics,
			insights.SalesMetrics,
		)
	}

	return insights, nil
}

// generateDateRange gera um slice de datas entre startDate e endDate (inclusive)
func generateDateRange(startDate, endDate *time.Time) []time.Time {
	if startDate == nil || endDate == nil || startDate.After(*endDate) {
		return []time.Time{}
	}

	var dates []time.Time
	currentDate := *startDate

	// Normalizando as datas para meia-noite
	currentDate = time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 0, 0, 0, 0, currentDate.Location())
	endDateTime := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, endDate.Location())

	for !currentDate.After(endDateTime) {
		dates = append(dates, currentDate)
		currentDate = currentDate.AddDate(0, 0, 1) // Adiciona um dia
	}

	return dates
}

// sumStringInts converte duas strings de valores inteiros para números, soma-os e retorna o resultado como string
func sumStringInts(a, b string) string {
	if a == "" {
		a = "0"
	}

	aVal, err := strconv.Atoi(a)
	if err != nil {
		logrus.Errorf("Error on sumStringInts a: %s and b: %s. error: %s", a, b, err)
		return "0"
	}

	if b == "" {
		b = "0"
	}

	bVal, err := strconv.Atoi(b)
	if err != nil {
		logrus.Errorf("Error on sumStringInts a: %s and b: %s. error: %s", a, b, err)
		return "0"
	}

	return strconv.Itoa(aVal + bVal)
}

// updateCampaignMetrics atualiza as métricas de uma campanha somando os valores
func updateCampaignMetrics(existing, adding *domain.CampaignInsight) {
	// Atualizar campos que são strings de números
	existing.Impressions = sumStringInts(existing.Impressions, adding.Impressions)
	existing.Reach = sumStringInts(existing.Reach, adding.Reach)
	existing.Clicks = sumStringInts(existing.Clicks, adding.Clicks)

	// Atualizar campos que são numéricos
	existing.Result += adding.Result
	existing.Spend += adding.Spend
}

// calculateDerivedMetrics calcula métricas derivadas como CostPerResult e Frequency
func calculateDerivedMetrics(metrics *domain.AdAccountMetrics, totalImpression, totalReach, totalResults int, totalSpend float64) {
	// Definir os totais calculados nos métodos
	metrics.Impressions = totalImpression
	metrics.Reach = totalReach
	metrics.Result = totalResults
	metrics.Spend = utils.RoundWithTwoDecimalPlace(totalSpend)

	// Calcular CostPerResult
	if totalResults > 0 {
		metrics.CostPerResult = utils.RoundWithTwoDecimalPlace(totalSpend / float64(totalResults))
	}

	// Calcular Frequency
	if totalReach > 0 {
		metrics.Frequency = utils.RoundWithTwoDecimalPlace(float64(totalImpression) / float64(totalReach))
	}
}

// combineAdMetrics combina múltiplos insights de anúncios em um único
func combineAdMetrics(adInsights []*domain.AdInsightEntry) *domain.AdAccountMetrics {
	if len(adInsights) == 0 {
		return nil
	}

	costPerResultByDate := make(map[string]float64)
	resultByDate := make(map[string]int)
	costPerResultByDate[adInsights[0].Date.Format(time.DateOnly)] = adInsights[0].AdMetrics.CostPerResult
	resultByDate[adInsights[0].Date.Format(time.DateOnly)] = adInsights[0].AdMetrics.Result

	// Criar um novo objeto de métricas para acumular os valores
	combined := &domain.AdAccountMetrics{
		AdAccountInsight: domain.AdAccountInsight{
			AccountID: adInsights[0].ExternalID,
			Name:      adInsights[0].AdMetrics.Name,
			Objective: adInsights[0].AdMetrics.Objective,
			Campaigns: make([]*domain.CampaignInsight, 0),
		},
		CostPerResultByDate: costPerResultByDate,
		ResultByDate:        resultByDate,
	}

	totalImpression := 0
	totalReach := 0
	totalResults := 0
	totalSpend := 0.0

	// Mapear campanhas pelo ID para combinar adequadamente
	campaignMap := make(map[string]*domain.CampaignInsight)

	// Somar todos os valores
	for _, insight := range adInsights {
		if insight.AdMetrics == nil {
			continue
		}

		// Somar métricas de nível de conta
		totalImpression += insight.AdMetrics.Impressions
		totalReach += insight.AdMetrics.Reach
		totalResults += insight.AdMetrics.Result
		totalSpend += insight.AdMetrics.Spend

		date := insight.Date.Format(time.DateOnly)
		combined.CostPerResultByDate[date] += insight.AdMetrics.CostPerResult
		combined.ResultByDate[date] += insight.AdMetrics.Result

		// Combinar métricas das campanhas
		for _, campaign := range insight.AdMetrics.Campaigns {
			existingCampaign, exists := campaignMap[campaign.CampaignID]
			if !exists {
				// Criar uma cópia do objeto para não modificar o original
				campaignCopy := *campaign
				campaignMap[campaign.CampaignID] = &campaignCopy
			} else {
				// Usar a função auxiliar para atualizar as métricas
				updateCampaignMetrics(existingCampaign, campaign)
			}
		}
	}

	// Converter o mapa de campanhas de volta para um slice
	for _, campaign := range campaignMap {
		// Recalcular campos derivados
		impressionsVal, err := strconv.Atoi(campaign.Impressions)
		if err != nil {
			logrus.Errorf("Error on updateCampaignMetrics a: %s and b: %s. error: %s", campaign.Impressions, campaign.Impressions, err)
			continue
		}

		reachVal, err := strconv.Atoi(campaign.Reach)
		if err != nil {
			logrus.Errorf("Error on updateCampaignMetrics a: %s and b: %s. error: %s", campaign.Reach, campaign.Reach, err)
			continue
		}

		// Recalcular o CostPerResult
		if campaign.Result > 0 {
			campaign.CostPerResult = utils.RoundWithTwoDecimalPlace(campaign.Spend / float64(campaign.Result))
		}

		// Recalcular a Frequency
		if reachVal > 0 {
			frequency := float64(impressionsVal) / float64(reachVal)
			campaign.Frequency = strconv.FormatFloat(utils.RoundWithTwoDecimalPlace(frequency), 'f', -1, 64)
		}

		campaign.Spend = utils.RoundWithTwoDecimalPlace(campaign.Spend)

		combined.Campaigns = append(combined.Campaigns, campaign)
	}

	// Calcular métricas derivadas
	calculateDerivedMetrics(combined, totalImpression, totalReach, totalResults, totalSpend)

	return combined
}

// processOriginSalesMetrics processa e combina métricas de vendas para determinada origem
func processOriginSalesMetrics(originMetrics *originAggregator) *domain.SalesMetrics {
	averageTicket := 0.0
	if originMetrics.salesQuantity > 0 {
		averageTicket = originMetrics.totalRevenue / float64(originMetrics.salesQuantity)
	}

	return &domain.SalesMetrics{
		TotalRevenue:  utils.RoundWithTwoDecimalPlace(originMetrics.totalRevenue),
		SalesQuantity: originMetrics.salesQuantity,
		AverageTicket: utils.RoundWithTwoDecimalPlace(averageTicket),
		Sales:         originMetrics.sales,
	}
}

// combineSalesMetrics combina múltiplas entradas de insights de vendas
func combineSalesMetrics(salesInsights []*domain.SalesInsightEntry) map[string]*domain.SalesMetrics {
	if len(salesInsights) == 0 {
		return nil
	}

	// Mapa para armazenar métricas combinadas por origem
	combinedMetrics := make(map[string]*domain.SalesMetrics)

	// Mapa para acumular os valores por origem
	originAccumulators := make(map[string]*originAggregator)

	// Para cada insight de vendas
	for _, insight := range salesInsights {
		if insight.SalesMetrics == nil {
			continue
		}

		// Para cada origem (como SocialNetwork, Store, etc.)
		for origin, metrics := range insight.SalesMetrics {
			if metrics == nil {
				continue
			}

			// Obter ou criar o acumulador para esta origem
			accumulator, exists := originAccumulators[origin]
			if !exists {
				accumulator = &originAggregator{
					totalRevenue:  0,
					salesQuantity: 0,
					sales:         make([]*domain.Sale, 0),
				}
				originAccumulators[origin] = accumulator
			}

			// Acumular os valores
			accumulator.totalRevenue += metrics.TotalRevenue
			accumulator.salesQuantity += metrics.SalesQuantity

			// Adicionar as vendas individuais, se disponíveis
			if metrics.Sales != nil {
				accumulator.sales = append(accumulator.sales, metrics.Sales...)
			}
		}
	}

	// Converter os acumuladores para objetos SalesMetrics
	for origin, accumulator := range originAccumulators {
		combinedMetrics[origin] = processOriginSalesMetrics(accumulator)
	}

	return combinedMetrics
}

func getSalesMetricsByOrigin(origin ssoticadomain.Origin, sales []ssoticadomain.Order) (*domain.SalesMetrics, error) {
	var totalRevenue float64

	domainSales := make([]*domain.Sale, 0)

	for _, sale := range sales {
		// Verifica se estamos buscando por SocialNetwork ou por Others
		isSocialNetworkSearch := origin == ssoticadomain.SocialNetworkOrigin
		isOthersSearch := origin == ssoticadomain.OthersOrigin

		// Verifica se a venda deve ser contabilizada
		shouldCount := false

		if isSocialNetworkSearch {
			// Para SocialNetwork: venda deve ter origem e essa origem deve estar na lista de SocialNetworkOrigins
			shouldCount = len(sale.CustomerOrigins) > 0 && slices.Contains(ssoticadomain.SocialNetworkOrigins, sale.CustomerOrigins[0])
		} else if isOthersSearch {
			// Para Others: venda não tem origem OU sua origem não está na lista de SocialNetworkOrigins
			shouldCount = len(sale.CustomerOrigins) == 0 || !slices.Contains(ssoticadomain.SocialNetworkOrigins, sale.CustomerOrigins[0])
		}

		if shouldCount {
			date, err := time.Parse(time.DateOnly, sale.Date)
			if err != nil {
				logrus.Error("Error on parse sale date:", err)
				return nil, err
			}

			totalRevenue += sale.NetAmount
			domainSales = append(domainSales, &domain.Sale{
				Date:      &date,
				NetAmount: sale.NetAmount,
			})
		}
	}

	salesQuantity := len(domainSales)

	averageTicket := 0.0
	if salesQuantity > 0 {
		averageTicket = utils.RoundWithTwoDecimalPlace(totalRevenue / float64(salesQuantity))
	}

	return &domain.SalesMetrics{
		TotalRevenue:  utils.RoundWithTwoDecimalPlace(totalRevenue),
		SalesQuantity: salesQuantity,
		AverageTicket: averageTicket,
		Sales:         domainSales,
	}, nil
}

// Métodos para a interface MetaInsighter

// GetAdAccountMetrics obtém métricas de anúncios do Meta
func (s *Service) GetAdAccountMetrics(accountID string, filters *domain.InsigthFilters) (*domain.AdAccountMetrics, error) {
	logrus.WithFields(logrus.Fields{
		"account_id": accountID,
		"start_date": filters.StartDate.Format(time.DateOnly),
		"end_date":   filters.EndDate.Format(time.DateOnly),
	}).Info("Obtendo métricas de anúncios do Meta")

	adAccountMetrics, err := s.metaService.GetAdAccountsInsights(accountID, filters)
	if err != nil {
		logrus.WithError(err).Warn("Erro ao obter métricas de anúncios do Meta")
		return nil, err
	}

	return adAccountMetrics, nil
}

// Métodos para a interface SSOticaInsighter

// GetSalesMetrics obtém métricas de vendas do SSOtica
func (s *Service) GetSalesMetrics(cnpj string, secretName string, filters *domain.InsigthFilters) (map[string]*domain.SalesMetrics, error) {
	logrus.WithFields(logrus.Fields{
		"cnpj":        cnpj,
		"secret_name": secretName,
		"start_date":  filters.StartDate.Format(time.DateOnly),
		"end_date":    filters.EndDate.Format(time.DateOnly),
	}).Info("Obtendo métricas de vendas do SSOtica")

	// Configurar os parâmetros para a chamada ao SSOtica
	params := &ssoticadomain.GetSalesParams{
		CNPJ:       cnpj,
		SecretName: secretName,
	}

	// Obter as vendas do SSOtica
	sales, err := s.ssoticaService.GetSalesByAccount(*params, filters)
	if err != nil {
		logrus.WithError(err).Warn("Erro ao obter vendas do SSOtica")
		return nil, err
	}

	if sales == nil {
		sales = make([]ssoticadomain.Order, 0)
	}

	// Processar as métricas de vendas por origem
	salesMetricsSocialNetwork, err := getSalesMetricsByOrigin(ssoticadomain.SocialNetworkOrigin, sales)
	if err != nil {
		logrus.WithError(err).Warn("Erro ao processar métricas de vendas para redes sociais")
		return nil, err
	}

	salesMetricsOthers, err := getSalesMetricsByOrigin(ssoticadomain.OthersOrigin, sales)
	if err != nil {
		logrus.WithError(err).Warn("Erro ao processar métricas de vendas para outras origens")
		return nil, err
	}

	// Montar o mapa de métricas por origem
	salesMetricsByOrigin := make(map[string]*domain.SalesMetrics)
	salesMetricsByOrigin[domain.SocialNetwork] = salesMetricsSocialNetwork
	salesMetricsByOrigin[domain.Store] = salesMetricsOthers

	return salesMetricsByOrigin, nil
}

// GetMonthlyInsightsByPeriod obtém os insights mensais para todas as contas em um período específico
func (s *Service) GetMonthlyInsightsByPeriod(period string) ([]*domain.MonthlyInsightReport, error) {
	// Buscar todas as contas ativas
	activeAccounts, err := s.accountRepository.ListAccounts([]domain.AdAccountStatus{domain.AdAccountStatusActive})
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar contas: %w", err)
	}

	// Buscar relatórios mensais de anúncios para o período
	reports := make([]*domain.MonthlyInsightReport, 0, len(activeAccounts))

	// Para cada conta, buscar os insights do mês especificado
	for _, acc := range activeAccounts {
		// Conversão de período para time.Time para uso nos repositórios
		t := parseMonthYearToPeriod(period)

		// Buscar insights de anúncios
		adInsight, err := s.monthlyAdInsightRepository.GetByAccountIDAndPeriod(acc.ID, t)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"account_id": acc.ID,
				"period":     period,
			}).Error("erro ao buscar insights mensais de anúncios")
			continue
		}

		// Buscar insights de vendas
		salesInsight, err := s.monthlySalesInsightRepository.GetByAccountIDAndPeriod(acc.ID, t)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"account_id": acc.ID,
				"period":     period,
			}).Error("erro ao buscar insights mensais de vendas")
		}

		// Se não tiver nem insights de anúncios nem de vendas, pular esta conta
		if adInsight == nil && salesInsight == nil {
			continue
		}

		// Criar o relatório para esta conta
		report := &domain.MonthlyInsightReport{
			AccountID:   acc.ID,
			AccountName: *acc.Nickname,
			Period:      period,
		}

		// Adicionar métricas de anúncios se disponíveis
		if adInsight != nil {
			report.AdMetrics = adInsight.AdMetrics
		}

		// Adicionar métricas de vendas se disponíveis
		if salesInsight != nil {
			report.SalesMetrics = salesInsight.SalesMetrics
		}

		// Calcular métricas de resultado se tiver ambos os dados
		if report.AdMetrics != nil && report.SalesMetrics != nil {
			report.ResultMetrics = domain.CalculateResultMetrics(report.AdMetrics, report.SalesMetrics)
		}

		reports = append(reports, report)
	}

	return reports, nil
}

// parseMonthYearToPeriod converte um período no formato "mm-yyyy" para time.Time
func parseMonthYearToPeriod(period string) time.Time {
	// Aqui assumimos que o período já está no formato mm-yyyy
	// Criamos uma data para o primeiro dia do mês
	timeFormat := "01-2006"
	t, err := time.Parse(timeFormat, period)
	if err != nil {
		// Em caso de erro, retorna a data atual
		logrus.WithError(err).WithField("period", period).Error("erro ao converter período para data")
		return time.Now()
	}
	return t
}

// GetAvailableMonthlyPeriods retorna os períodos (meses e anos) disponíveis nas tabelas de insights mensais
func (s *Service) GetAvailableMonthlyPeriods() (*domain.AvailablePeriods, error) {
	// Verificar se os repositórios mensais estão disponíveis
	if s.monthlyAdInsightRepository == nil || s.monthlySalesInsightRepository == nil {
		return nil, fmt.Errorf("repositórios de insights mensais não estão disponíveis")
	}

	// Obter todos os períodos disponíveis
	adPeriods, err := s.monthlyAdInsightRepository.GetAllPeriods()
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar períodos de insights de anúncios: %w", err)
	}

	salesPeriods, err := s.monthlySalesInsightRepository.GetAllPeriods()
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar períodos de insights de vendas: %w", err)
	}

	// Combinar e remover duplicados
	periodMap := make(map[string]bool)
	yearMap := make(map[string]bool)
	monthMap := make(map[string]bool)

	// Adicionar períodos de anúncios
	for _, period := range adPeriods {
		periodMap[period] = true

		// Extrair ano e mês do período (formato mm-yyyy)
		if len(period) == 7 {
			month := period[:2]
			year := period[3:]

			monthMap[month] = true
			yearMap[year] = true
		}
	}

	// Adicionar períodos de vendas
	for _, period := range salesPeriods {
		periodMap[period] = true

		// Extrair ano e mês do período (formato mm-yyyy)
		if len(period) == 7 {
			month := period[:2]
			year := period[3:]

			monthMap[month] = true
			yearMap[year] = true
		}
	}

	// Converter mapas para slices
	periods := make([]string, 0, len(periodMap))
	for period := range periodMap {
		periods = append(periods, period)
	}

	years := make([]string, 0, len(yearMap))
	for year := range yearMap {
		years = append(years, year)
	}

	months := make([]string, 0, len(monthMap))
	for month := range monthMap {
		months = append(months, month)
	}

	// Ordenar os slices
	sort.Strings(periods)
	sort.Strings(years)
	sort.Strings(months)

	return &domain.AvailablePeriods{
		Periods: periods,
		Years:   years,
		Months:  months,
	}, nil
}

// GetAdAccountReachImpressions obtém apenas Reach e Impressions de uma conta específica
func (s *Service) GetAdAccountReachImpressions(accountID string, filters *domain.InsigthFilters) (*domain.ReachImpressionsResponse, error) {
	// Verificar se os filtros têm datas válidas
	if filters == nil || filters.StartDate == nil || filters.EndDate == nil {
		return nil, fmt.Errorf("é necessário informar as datas de início e fim")
	}

	// Validar se as datas estão em ordem
	if filters.StartDate.After(*filters.EndDate) {
		return nil, fmt.Errorf("a data de início não pode ser posterior à data de fim")
	}

	logrus.WithFields(logrus.Fields{
		"account_id": accountID,
		"start_date": filters.StartDate.Format(time.DateOnly),
		"end_date":   filters.EndDate.Format(time.DateOnly),
	}).Info("Obtendo Reach e Impressions da conta do Meta")

	// Buscar diretamente da API do Meta
	metrics, err := s.metaService.GetAdAccountReachImpressions(accountID, filters)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"account_id": accountID,
			"start_date": filters.StartDate.Format(time.DateOnly),
			"end_date":   filters.EndDate.Format(time.DateOnly),
		}).Error("Erro ao obter Reach e Impressions do Meta")
		return nil, err
	}

	return metrics, nil
}
