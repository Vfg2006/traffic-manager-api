package handler

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/internal/scheduler"
	"github.com/vfg2006/traffic-manager-api/pkg/apiErrors"
	"github.com/vfg2006/traffic-manager-api/pkg/middleware"
)

// CronJobType define o tipo de cron job que será executada
const (
	CronJobTypeMeta               = "meta"
	CronJobTypeSSOtica            = "ssotica"
	CronJobTypeMonthly            = "monthly"
	CronJobTypeTopRankingAccounts = "top-ranking-accounts"
	CronJobTypeAll                = "all"
)

// CronJobServices contém os serviços de cron necessários para executar manualmente
type CronJobServices struct {
	MetaInsightSyncService        *scheduler.MetaInsightSyncService
	SSOticaInsightSyncService     *scheduler.SSOticaInsightSyncService
	MonthlyInsightsSyncService    *scheduler.MonthlyInsightsSyncService
	TopRankingAccountsSyncService *scheduler.TopRankingAccountsService
}

// RunCronJob executa manualmente uma cron job específica
func RunCronJob(services CronJobServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("INIT - RunCronJob")

		// Verificar permissões - apenas administradores podem executar cron jobs
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok || userClaims.UserRoleID != 1 {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Apenas administradores podem executar cron jobs", nil)
			return
		}

		// Obter o tipo de cron job da URL
		cronType := httprouter.ParamsFromContext(r.Context()).ByName("type")
		if cronType == "" {
			apiErrors.WriteError(w, apiErrors.ErrMissingRequiredData, "Tipo de cron job não especificado", nil)
			return
		}

		// Validar o tipo de cron job
		switch cronType {
		case CronJobTypeMeta:
			// Executar sincronização do Meta
			if services.MetaInsightSyncService == nil {
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Serviço de sincronização do Meta não disponível", nil)
				return
			}
			services.MetaInsightSyncService.TriggerManualSync()

		case CronJobTypeSSOtica:
			// Executar sincronização do SSOtica
			if services.SSOticaInsightSyncService == nil {
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Serviço de sincronização do SSOtica não disponível", nil)
				return
			}
			services.SSOticaInsightSyncService.TriggerManualSync()

		case CronJobTypeMonthly:
			// Executar sincronização mensal
			if services.MonthlyInsightsSyncService == nil {
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Serviço de sincronização mensal não disponível", nil)
				return
			}
			services.MonthlyInsightsSyncService.TriggerManualSync()

		case CronJobTypeTopRankingAccounts:
			// Executar sincronização de top ranking de contas
			if services.TopRankingAccountsSyncService == nil {
				apiErrors.WriteError(w, apiErrors.ErrInternalServer, "Serviço de sincronização de top ranking de contas não disponível", nil)
				return
			}
			services.TopRankingAccountsSyncService.TriggerManualSync()

		case CronJobTypeAll:
			// Executar ambas as sincronizações
			if services.MetaInsightSyncService != nil {
				services.MetaInsightSyncService.TriggerManualSync()
			}
			if services.SSOticaInsightSyncService != nil {
				services.SSOticaInsightSyncService.TriggerManualSync()
			}
			if services.MonthlyInsightsSyncService != nil {
				services.MonthlyInsightsSyncService.TriggerManualSync()
			}
		default:
			apiErrors.WriteError(w, apiErrors.ErrInvalidRequest, "Tipo de cron job inválido. Valores aceitos: meta, ssotica, monthly, all", nil)
			return
		}

		// Responder com sucesso
		response := map[string]any{
			"message": "Cron job iniciada com sucesso",
			"type":    cronType,
		}
		json.NewEncoder(w).Encode(response)
	}
}

// GetCronStatus retorna o status das cron jobs
func GetCronStatus(services CronJobServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("INIT - GetCronStatus")

		// Verificar permissões - apenas administradores podem ver status das crons
		userClaims, ok := r.Context().Value(middleware.ContextKeyUser).(*domain.Claims)
		if !ok || userClaims.UserRoleID != 1 {
			apiErrors.WriteError(w, apiErrors.ErrInsufficientPrivilege, "Apenas administradores podem verificar status de cron jobs", nil)
			return
		}

		status := map[string]any{
			"meta":                 services.MetaInsightSyncService.GetStatus(),
			"ssotica":              services.SSOticaInsightSyncService.GetStatus(),
			"monthly":              services.MonthlyInsightsSyncService.GetStatus(),
			"top-ranking-accounts": services.TopRankingAccountsSyncService.GetStatus(),
		}

		json.NewEncoder(w).Encode(status)
	}
}
