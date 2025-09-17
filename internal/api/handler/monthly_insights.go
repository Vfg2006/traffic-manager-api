package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/vfg2006/traffic-manager-api/internal/usecases/insighting"
	"github.com/vfg2006/traffic-manager-api/pkg/log"
)

// GetMonthlyInsightReport retorna insights mensais de todas as contas para um período específico
func GetMonthlyInsightReport(service insighting.CombinedInsighter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := log.ForContext(r.Context())

		// Obter parâmetros de consulta
		month := r.URL.Query().Get("month")
		year := r.URL.Query().Get("year")

		if month == "" || year == "" {
			http.Error(w, "É necessário informar mês e ano nos parâmetros", http.StatusBadRequest)
			return
		}

		// Validar mês (entre 01 e 12)
		if len(month) != 2 || month < "01" || month > "12" {
			http.Error(w, "Mês inválido. Use formato de dois dígitos (01-12)", http.StatusBadRequest)
			return
		}

		// Validar ano (4 dígitos)
		if len(year) != 4 {
			http.Error(w, "Ano inválido. Use formato de quatro dígitos (ex: 2025)", http.StatusBadRequest)
			return
		}

		// Formar o período no formato esperado mm-yyyy
		period := fmt.Sprintf("%s-%s", month, year)

		logger.WithFields(log.Fields{
			"month":  month,
			"year":   year,
			"period": period,
		}).Info("monthly-insights: buscando relatório de insights mensais")

		// Buscar insights mensais
		insights, err := service.GetMonthlyInsightsByPeriod(period)
		if err != nil {
			logger.WithError(err).WithFields(log.Fields{
				"period": period,
			}).Error("monthly-insights: erro ao buscar insights mensais")

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Registrar resultado
		logger.WithFields(log.Fields{
			"period":            period,
			"accounts_returned": len(insights),
		}).Info("monthly-insights: relatório gerado com sucesso")

		// Retornar resposta
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(insights); err != nil {
			logger.WithError(err).Error("monthly-insights: erro ao codificar resposta")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

// GetAvailableMonthlyPeriods retorna os períodos (meses e anos) disponíveis na API
func GetAvailableMonthlyPeriods(service insighting.CombinedInsighter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := log.ForContext(r.Context())
		logger.Info("insights-periods: buscando períodos disponíveis")

		// Buscar períodos disponíveis
		availablePeriods, err := service.GetAvailableMonthlyPeriods()
		if err != nil {
			logger.WithError(err).Error("insights-periods: erro ao buscar períodos disponíveis")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Registrar resultado
		logger.WithFields(log.Fields{
			"total_periods": len(availablePeriods.Periods),
			"years":         availablePeriods.Years,
			"months":        availablePeriods.Months,
		}).Info("insights-periods: períodos disponíveis recuperados com sucesso")

		// Retornar resposta
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(availablePeriods); err != nil {
			logger.WithError(err).Error("insights-periods: erro ao codificar resposta")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
