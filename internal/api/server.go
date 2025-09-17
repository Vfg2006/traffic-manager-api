package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/justinas/alice"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/internal/api/handler"
	"github.com/vfg2006/traffic-manager-api/internal/api/handler/router"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/scheduler"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/account"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/authenticating"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/insighting"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/ranking"
	"github.com/vfg2006/traffic-manager-api/pkg/middleware"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Server struct {
	httpServer *http.Server
}

func New(
	config *config.Config,
	insightService insighting.CombinedInsighter,
	accountService account.AccountService,
	rankingService ranking.RankingService,
	authenticator authenticating.Authenticator,
	metaSyncService *scheduler.MetaInsightSyncService,
	ssoticaSyncService *scheduler.SSOticaInsightSyncService,
	monthlyInsightsSyncService *scheduler.MonthlyInsightsSyncService,
	topRankingAccountsSyncService *scheduler.TopRankingAccountsService,
) (*Server, error) {
	// Inicializar o struct com os serviços de cron jobs
	cronServices := handler.CronJobServices{
		MetaInsightSyncService:        metaSyncService,
		SSOticaInsightSyncService:     ssoticaSyncService,
		MonthlyInsightsSyncService:    monthlyInsightsSyncService,
		TopRankingAccountsSyncService: topRankingAccountsSyncService,
	}

	rt := router.New(
		router.WithRoutes(handler.Healthcheck()...),
		router.WithRoutes(handler.Authentication(authenticator)...),
		router.WithRoutes(handler.User(authenticator)...),
		router.WithRoutes(handler.Insights(insightService)...),
		router.WithRoutes(handler.AdAccounts(accountService)...),
		router.WithRoutes(handler.UserAccounts(authenticator)...),
		router.WithRoutes(handler.StoreRanking(rankingService)...),
		router.WithRoutes(handler.CronJobs(cronServices)...),
	)

	middlewares := []alice.Constructor{
		middleware.LogPanicMiddleware(),
		middleware.LoggingMiddleware(),
		middleware.Cors(),
		middleware.AuthMiddleware(authenticator),
	}

	handler := alice.New(middlewares...).Then(rt)

	srv := &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf("%s:%s", config.Server.Host, config.Server.Port),
			Handler:           handler,
			ReadHeaderTimeout: 2 * time.Second,
		},
	}

	return srv, nil
}

func (s Server) Run(ctx context.Context) error {
	go func() {
		logrus.WithFields(logrus.Fields{
			"address": s.httpServer.Addr,
		}).Info("Servidor iniciando")

		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("Erro durante a execução do servidor")
		}
	}()

	// Canal para aguardar sinais de término
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	// Aguardar pelo sinal ou pelo cancelamento do contexto
	select {
	case <-done:
		logrus.Info("Sinal de interrupção recebido")
	case <-ctx.Done():
		logrus.Info("Contexto de aplicação cancelado")
	}

	// Define timeout para desligamento
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Log de início do desligamento
	logrus.WithFields(logrus.Fields{
		"timeout": "15s",
	}).Info("Iniciando desligamento gracioso do servidor")

	if err := s.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("Erro durante o desligamento do servidor")
		return err
	}

	logrus.Info("Servidor desligado com sucesso")
	return nil
}

func (s Server) Shutdown(ctx context.Context) error {
	logrus.Info("Executando operações de limpeza antes do desligamento")

	// Aqui você pode adicionar operações de limpeza adicionais
	// como fechar conexões com bancos de dados, limpar recursos, etc.

	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		return err
	}

	logrus.Info("Servidor HTTP desligado com sucesso")
	return nil
}
