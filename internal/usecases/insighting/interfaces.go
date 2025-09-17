package insighting

import (
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

// MetaInsighter define a interface para obter métricas de anúncios do Meta
type MetaInsighter interface {
	// GetAdAccountMetrics obtém as métricas de anúncios para uma conta específica
	GetAdAccountMetrics(accountID string, filters *domain.InsigthFilters) (*domain.AdAccountMetrics, error)
}

// SSOticaInsighter define a interface para obter métricas de vendas do SSOtica
type SSOticaInsighter interface {
	// GetSalesMetrics obtém as métricas de vendas para uma conta específica
	GetSalesMetrics(cnpj string, secretName string, filters *domain.InsigthFilters) (map[string]*domain.SalesMetrics, error)
}

// CombinedInsighter é a interface completa que combina as funcionalidades do Meta e SSOtica
type CombinedInsighter interface {
	MetaInsighter
	SSOticaInsighter

	// GetAdAccountsByID obtém todas as métricas (anúncios e vendas) para uma conta específica
	GetAdAccountsByID(accountID string, filters *domain.InsigthFilters) (*domain.AdAccountInsightsResponse, error)

	// GetAdAccountReachImpressions obtém apenas Reach e Impressions de uma conta específica
	GetAdAccountReachImpressions(accountID string, filters *domain.InsigthFilters) (*domain.ReachImpressionsResponse, error)

	// GetMonthlyInsightsByPeriod obtém os insights mensais para todas as contas em um período específico
	GetMonthlyInsightsByPeriod(period string) ([]*domain.MonthlyInsightReport, error)

	// GetAvailableMonthlyPeriods retorna os períodos (meses e anos) disponíveis nas tabelas de insights mensais
	GetAvailableMonthlyPeriods() (*domain.AvailablePeriods, error)
}
