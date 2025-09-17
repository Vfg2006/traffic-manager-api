package handler

import (
	"net/http"

	"github.com/vfg2006/traffic-manager-api/internal/api/handler/router"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/account"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/authenticating"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/insighting"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/ranking"
	"github.com/vfg2006/traffic-manager-api/pkg/middleware"
)

func Healthcheck() []router.Route {
	return []router.Route{
		{
			Path:    "/healthcheck",
			Method:  http.MethodGet,
			Handler: HealthcheckHandler(),
		},
	}
}

func AdAccounts(service account.AccountService) []router.Route {
	return []router.Route{
		{
			Path:        "/v1/accounts",
			Method:      http.MethodGet,
			Handler:     AdAccountList(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
		},
		{
			Path:        "/v1/accounts/sync",
			Method:      http.MethodGet,
			Handler:     SyncAccounts(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
		},
		{
			Path:        "/v1/accounts/:id",
			Method:      http.MethodPut,
			Handler:     UpdateAdAccount(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AllRoles()},
		},
	}
}

func Insights(service insighting.CombinedInsighter) []router.Route {
	return []router.Route{
		{
			Path:        "/v1/adAccount/:id/insights",
			Method:      http.MethodGet,
			Handler:     GetAdAccountsByID(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AllRoles()},
		},
		{
			Path:        "/v1/adAccount/:id/insights/reach-impressions",
			Method:      http.MethodGet,
			Handler:     GetAdAccountReachImpressions(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AllRoles()},
		},
		{
			Path:        "/v1/insights/report",
			Method:      http.MethodGet,
			Handler:     GetMonthlyInsightReport(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOrSupervisor()},
		},
		{
			Path:        "/v1/insights/periods",
			Method:      http.MethodGet,
			Handler:     GetAvailableMonthlyPeriods(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOrSupervisor()},
		},
	}
}

func Authentication(service authenticating.Authenticator) []router.Route {
	return []router.Route{
		{
			Path:    "/v1/login",
			Method:  http.MethodPost,
			Handler: Login(service),
		},
		{
			Path:    "/v1/register",
			Method:  http.MethodPost,
			Handler: CreateUser(service),
		},
		{
			Path:        "/v1/users/:id/generate-password",
			Method:      http.MethodPost,
			Handler:     GeneratePassword(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
		},
		{
			Path:        "/v1/users/:id/change-password",
			Method:      http.MethodPost,
			Handler:     ChangePassword(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AllRoles()},
		},
		{
			Path:        "/v1/me",
			Method:      http.MethodGet,
			Handler:     GetMe(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AllRoles()},
		},
	}
}

func User(service authenticating.Authenticator) []router.Route {
	return []router.Route{
		{
			Path:        "/v1/users",
			Method:      http.MethodGet,
			Handler:     ListUsers(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
		},
		{
			Path:        "/v1/users",
			Method:      http.MethodPost,
			Handler:     CreateUser(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
		},
		{
			Path:        "/v1/users/:id",
			Method:      http.MethodGet,
			Handler:     GetUser(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AllRoles()},
		},
		{
			Path:        "/v1/users/:id",
			Method:      http.MethodPut,
			Handler:     UpdateUser(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AllRoles()},
		},
	}
}

// UserAccounts retorna as rotas para gerenciamento de contas vinculadas a usuários
func UserAccounts(service authenticating.Authenticator) []router.Route {
	return []router.Route{
		{
			Path:        "/v1/me/accounts",
			Method:      http.MethodGet,
			Handler:     GetUserAccounts(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AllRoles()},
		},
		{
			Path:        "/v1/users/:id/accounts",
			Method:      http.MethodPut,
			Handler:     UpdateUserAccounts(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
		},
		{
			Path:        "/v1/users/:id/accounts/link",
			Method:      http.MethodPost,
			Handler:     LinkUserAccount(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
		},
		{
			Path:        "/v1/users/:id/accounts/:account_id",
			Method:      http.MethodDelete,
			Handler:     UnlinkUserAccount(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOnly()},
		},
	}
}

func StoreRanking(service ranking.RankingService) []router.Route {
	return []router.Route{
		{
			Path:        "/v1/stores/ranking/social-network-revenue",
			Method:      http.MethodGet,
			Handler:     GetStoreRanking(service),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOrSupervisor()},
		},
	}
}

func CronJobs(services CronJobServices) []router.Route {
	return []router.Route{
		{
			Path:        "/v1/cron/:type/run",
			Method:      http.MethodPost,
			Handler:     RunCronJob(services),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOrSupervisor()},
		},
		{
			Path:        "/v1/cron/status",
			Method:      http.MethodGet,
			Handler:     GetCronStatus(services),
			Middlewares: []func(http.Handler) http.Handler{middleware.AdminOrSupervisor()},
		},
	}
}
