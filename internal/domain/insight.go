package domain

import (
	"fmt"
	"time"

	"github.com/vfg2006/traffic-manager-api/pkg/utils"
)

type InsigthFilters struct {
	StartDate *time.Time
	EndDate   *time.Time
}

type ResultMetrics struct {
	Conversion float64
	ROI        string
}

type AdAccountInsightsResponse struct {
	AdAccountMetrics *AdAccountMetrics
	SalesMetrics     map[string]*SalesMetrics
	ResultMetrics    *ResultMetrics
	Filters          *InsigthFilters
}

// CalculateResultMetrics calcula métricas de resultado combinando dados de anúncios e vendas
func CalculateResultMetrics(adMetrics *AdAccountMetrics, salesMetrics map[string]*SalesMetrics) *ResultMetrics {
	if adMetrics == nil || salesMetrics == nil || salesMetrics[SocialNetwork] == nil {
		return nil
	}

	// Calcular conversão (porcentagem de resultados que geraram vendas)
	conversion := 0.0
	if adMetrics.Result > 0 {
		conversion = (float64(salesMetrics[SocialNetwork].SalesQuantity) / float64(adMetrics.Result)) * 100
	}

	// Calcular ROI (retorno sobre investimento)
	roi := 0.0
	if adMetrics.Spend > 0 {
		roi = salesMetrics[SocialNetwork].TotalRevenue / adMetrics.Spend
	}

	return &ResultMetrics{
		Conversion: utils.RoundWithTwoDecimalPlace(conversion),
		ROI:        fmt.Sprintf("%dx", int(roi)),
	}
}

// CombineInsights combina insights de anúncios e vendas em uma resposta completa
func CombineInsights(adInsight *AdInsightEntry, salesInsight *SalesInsightEntry, filters *InsigthFilters) *AdAccountInsightsResponse {
	if adInsight == nil && salesInsight == nil {
		return nil
	}

	response := &AdAccountInsightsResponse{
		Filters: filters,
	}

	// Adicionar métricas de anúncios, se disponíveis
	if adInsight != nil {
		response.AdAccountMetrics = adInsight.AdMetrics
	}

	// Adicionar métricas de vendas, se disponíveis
	if salesInsight != nil {
		response.SalesMetrics = salesInsight.SalesMetrics
	}

	// Calcular métricas de resultado se ambos os dados estiverem disponíveis
	if adInsight != nil && adInsight.AdMetrics != nil && salesInsight != nil && salesInsight.SalesMetrics != nil {
		response.ResultMetrics = CalculateResultMetrics(adInsight.AdMetrics, salesInsight.SalesMetrics)
	}

	return response
}
