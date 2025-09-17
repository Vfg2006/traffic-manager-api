package main

import (
	"context"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/database/postgres"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta/metaclient"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/ssoticaclient"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository"
	"github.com/vfg2006/traffic-manager-api/internal/api"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/scheduler"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/account"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/authenticating"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/insighting"
	"github.com/vfg2006/traffic-manager-api/internal/usecases/ranking"
)

func main() {
	// Inicializa configuração de logs
	configureLogger()

	cfg, err := config.NewConfig()
	if err != nil {
		logrus.Fatal(err)
	}

	// Define o nível de log com base na configuração
	logLevel, err := logrus.ParseLevel(cfg.App.LogLevel)
	if err != nil {
		logrus.Warnf("Nível de log inválido: %s, usando 'info'", cfg.App.LogLevel)
		logLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logLevel)
	logrus.Infof("Nível de log configurado para: %s", logLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgConn := pgconn(ctx, cfg.Database)
	defer pgConn.Close()

	accountRepo := repository.NewAccountRepository(pgConn)
	userRepo := repository.NewUserRepository(pgConn)
	adInsightRepo := repository.NewAdInsightRepository(pgConn)
	salesInsightRepo := repository.NewSalesInsightRepository(pgConn)
	monthlyAdInsightRepo := repository.NewMonthlyAdInsightRepository(pgConn)
	monthlySalesInsightRepo := repository.NewMonthlySalesInsightRepository(pgConn)
	storeRankingRepo := repository.NewStoreRankingRepository(pgConn)

	authenticator := authenticating.NewService(userRepo, accountRepo, cfg)

	renderClient := config.NewRenderClient(cfg)

	tokenManager := metaclient.NewTokenManager(cfg, renderClient)
	go tokenManager.StartAutoRefresh()
	defer tokenManager.StopAutoRefresh()

	metaClient := metaclient.NewClient(cfg, tokenManager)
	metaIntegrator := meta.New(cfg, metaClient)

	ssoticaClient := ssoticaclient.NewClient(cfg)
	ssoticaIntegrator := ssotica.New(cfg, ssoticaClient)

	accountService := account.NewService(accountRepo, metaIntegrator, renderClient, ssoticaIntegrator, cfg)

	// Inicializa o serviço de insights com suporte a cache
	insightService := insighting.NewService(cfg, metaIntegrator, ssoticaIntegrator, accountRepo)
	cachedInsightService := insightService.(*insighting.Service).WithCache(
		adInsightRepo,
		salesInsightRepo,
		monthlyAdInsightRepo,
		monthlySalesInsightRepo,
	)

	rankingService := ranking.NewStoreRankingService(storeRankingRepo)

	// Inicializa os agendadores de sincronização separados
	metaInsightSyncService := scheduler.NewMetaInsightSyncService(
		accountRepo,
		adInsightRepo,
		cachedInsightService, // Implementa MetaInsighter
		cfg,
	)

	ssoticaInsightSyncService := scheduler.NewSSOticaInsightSyncService(
		accountRepo,
		salesInsightRepo,
		cachedInsightService, // Implementa SSOticaInsighter
		cfg,
	)

	// Inicializa o agendador de sincronização mensal
	monthlyInsightsSyncService := scheduler.NewMonthlyInsightsSyncService(
		accountRepo,
		monthlyAdInsightRepo,
		monthlySalesInsightRepo,
		cachedInsightService, // Implementa MetaInsighter
		cachedInsightService, // Implementa SSOticaInsighter
		cfg,
	)

	topRankingAccountsSyncService := scheduler.NewTopRankingAccountsService(
		accountRepo,
		storeRankingRepo,
		salesInsightRepo,
		ssoticaIntegrator,
		cfg,
	)

	// Inicia os agendadores em background
	if err := metaInsightSyncService.Start(ctx); err != nil {
		logrus.WithError(err).Error("Erro ao iniciar o agendador de sincronização de insights do Meta")
	} else {
		logrus.Info("Agendador de sincronização de insights do Meta iniciado com sucesso")
	}

	if err := ssoticaInsightSyncService.Start(ctx); err != nil {
		logrus.WithError(err).Error("Erro ao iniciar o agendador de sincronização de insights do SSOtica")
	} else {
		logrus.Info("Agendador de sincronização de insights do SSOtica iniciado com sucesso")
	}

	if err := monthlyInsightsSyncService.Start(ctx); err != nil {
		logrus.WithError(err).Error("Erro ao iniciar o agendador de sincronização mensal de insights")
	} else {
		logrus.Info("Agendador de sincronização mensal de insights iniciado com sucesso")
	}

	if err := topRankingAccountsSyncService.Start(ctx); err != nil {
		logrus.WithError(err).Error("Erro ao iniciar o agendador de sincronização de top ranking de contas")
	} else {
		logrus.Info("Agendador de sincronização de top ranking de contas iniciado com sucesso")
	}

	server, err := api.New(
		cfg,
		cachedInsightService,
		accountService,
		rankingService,
		authenticator,
		metaInsightSyncService,        // Serviço de sincronização Meta
		ssoticaInsightSyncService,     // Serviço de sincronização SSOtica
		monthlyInsightsSyncService,    // Serviço de sincronização mensal
		topRankingAccountsSyncService, // Serviço de sincronização de top ranking de contas
	)
	if err != nil {
		logrus.Fatal(err)
	}

	if err := server.Run(ctx); err != nil {
		logrus.Error(err)
	}
}

// configureLogger configura o formato e comportamento dos logs
func configureLogger() {
	_, file, _, _ := runtime.Caller(0)
	dir := path.Dir(file)
	os.Chdir(dir)

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
}

// pgconn cria uma conexão com o banco de dados
func pgconn(ctx context.Context, dbConfig config.Database) *postgres.Connection {
	conn, err := postgres.NewConnection(ctx, dbConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Erro ao conectar ao PostgreSQL")
	}

	err = conn.Ping(ctx)
	if err != nil {
		logrus.WithError(err).Fatal("Erro ao testar conexão com PostgreSQL")
	}

	logrus.Info("Conexão com PostgreSQL estabelecida com sucesso")
	return conn
}
