package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	App                 App                 `mapstructure:",squash"`
	Server              Server              `mapstructure:",squash"`
	Database            Database            `mapstructure:",squash"`
	Meta                Meta                `mapstructure:",squash"`
	Render              Render              `mapstructure:",squash"`
	SSOtica             SSOtica             `mapstructure:",squash"`
	Auth                Auth                `mapstructure:",squash"`
	MetaInsightSync     MetaInsightSync     `mapstructure:",squash"`
	SSOticaInsightSync  SSOticaInsightSync  `mapstructure:",squash"`
	MonthlyInsightsSync MonthlyInsightsSync `mapstructure:",squash"`
	TopRankingAccounts  TopRankingAccounts  `mapstructure:",squash"`
	SecretKey           string              `mapstructure:"secret_key"`
	SSOticaMultiClient  map[string]SSOtica  `mapstructure:"-"`
}

type Server struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

type Database struct {
	DSN      string `mapstructure:"-"`
	Driver   string `mapstructure:"database_driver"`
	Password string `mapstructure:"database_password"`
	URL      string `mapstructure:"database_url"`
	User     string `mapstructure:"database_user"`
}

type Meta struct {
	BaseURL        string    `mapstructure:"meta_base_url"`
	URL            string    `mapstructure:"meta_url"`
	Version        string    `mapstructure:"meta_version"`
	AccessToken    string    `mapstructure:"meta_access_token"`
	AppID          string    `mapstructure:"meta_app_id"`
	AppSecret      string    `mapstructure:"meta_app_secret"`
	LongLivedToken string    `mapstructure:"meta_long_lived_token"`
	TokenExpiresAt time.Time `mapstructure:"-"`
}

type SSOtica struct {
	URL         string `mapstructure:"ssotica_url"`
	AccessToken string `mapstructure:"ssotica_access_token"`
}

type Render struct {
	APIKey    string `mapstructure:"render_api_key"`
	ServiceID string `mapstructure:"render_service_id"`
}

type App struct {
	LogLevel string `mapstructure:"log_level"`
}

type Auth struct {
	Secret string `mapstructure:"auth_secret"`
}

type MetaInsightSync struct {
	CronSchedule        string `mapstructure:"meta_insight_sync_cron"`
	LookbackDays        int    `mapstructure:"meta_insight_sync_lookback_days"`
	RequestDelaySeconds int    `mapstructure:"meta_insight_sync_request_delay_seconds"`
	MaxConcurrentJobs   int    `mapstructure:"meta_insight_sync_max_concurrent_jobs"`
	Enabled             bool   `mapstructure:"meta_insight_sync_enabled"`
}

type SSOticaInsightSync struct {
	CronSchedule        string `mapstructure:"ssotica_insight_sync_cron"`
	LookbackDays        int    `mapstructure:"ssotica_insight_sync_lookback_days"`
	RequestDelaySeconds int    `mapstructure:"ssotica_insight_sync_request_delay_seconds"`
	MaxConcurrentJobs   int    `mapstructure:"ssotica_insight_sync_max_concurrent_jobs"`
	Enabled             bool   `mapstructure:"ssotica_insight_sync_enabled"`
}

type MonthlyInsightsSync struct {
	CronSchedule        string `mapstructure:"monthly_insights_sync_cron"`
	RequestDelaySeconds int    `mapstructure:"monthly_insights_sync_request_delay_seconds"`
	MaxConcurrentJobs   int    `mapstructure:"monthly_insights_sync_max_concurrent_jobs"`
	Enabled             bool   `mapstructure:"monthly_insights_sync_enabled"`
	MonthLookBack       int    `mapstructure:"monthly_insights_sync_month_lookback"`
}

type TopRankingAccounts struct {
	CronSchedule string `mapstructure:"top_ranking_accounts_cron"`
	SyncEnabled  bool   `mapstructure:"top_ranking_accounts_sync_enabled"`
}

func SetDefaults() {
	viper.SetDefault("HOST", "localhost")
	viper.SetDefault("PORT", 8000)

	viper.SetDefault("DATABASE_DRIVER", "postgres")
	viper.SetDefault("DATABASE_URL", "localhost:5432/traffic")
	viper.SetDefault("DATABASE_USER", "postgres")
	viper.SetDefault("DATABASE_PASSWORD", "root")

	viper.SetDefault("META_BASE_URL", "https://graph.facebook.com")
	viper.SetDefault("META_URL", "https://graph.facebook.com/v22.0")
	viper.SetDefault("META_VERSION", "v22.0")
	viper.SetDefault("META_APP_ID", "your_app_id")
	viper.SetDefault("META_APP_SECRET", "your_app_secret")
	viper.SetDefault("META_ACCESS_TOKEN", "your_access_token") // ONLY LOCAL

	viper.SetDefault("SECRET_KEY", "your_secret_key")

	viper.SetDefault("RENDER_API_KEY", "")
	viper.SetDefault("RENDER_SERVICE_ID", "")

	viper.SetDefault("SSOTICA_URL", "https://app.ssotica.com.br/api/v1")
	viper.SetDefault("SSOTICA_ACCESS_TOKEN", "your_access_token")

	// Defaults para sincronização de insights
	viper.SetDefault("META_INSIGHT_SYNC_CRON", "0 3 * * *")        // Todos os dias às 3h da manhã
	viper.SetDefault("META_INSIGHT_SYNC_LOOKBACK_DAYS", 7)         // 7 dias para buscar dados
	viper.SetDefault("META_INSIGHT_SYNC_REQUEST_DELAY_SECONDS", 2) // 2 segundos entre requisições
	viper.SetDefault("META_INSIGHT_SYNC_MAX_CONCURRENT_JOBS", 3)   // 3 jobs concorrentes
	viper.SetDefault("META_INSIGHT_SYNC_ENABLED", false)           // Habilitar sincronização de anúncios

	viper.SetDefault("SSOTICA_INSIGHT_SYNC_CRON", "0 4 * * *")        // Todos os dias às 4h da manhã
	viper.SetDefault("SSOTICA_INSIGHT_SYNC_LOOKBACK_DAYS", 7)         // 7 dias para buscar dados
	viper.SetDefault("SSOTICA_INSIGHT_SYNC_REQUEST_DELAY_SECONDS", 2) // 2 segundos entre requisições
	viper.SetDefault("SSOTICA_INSIGHT_SYNC_MAX_CONCURRENT_JOBS", 3)   // 3 jobs concorrentes
	viper.SetDefault("SSOTICA_INSIGHT_SYNC_ENABLED", false)           // Habilitar sincronização de vendas

	// Defaults para sincronização mensal de insights
	viper.SetDefault("MONTHLY_INSIGHTS_SYNC_CRON", "0 5 1 * *")        // No primeiro dia de cada mês às 5h da manhã
	viper.SetDefault("MONTHLY_INSIGHTS_SYNC_REQUEST_DELAY_SECONDS", 2) // 2 segundos entre requisições
	viper.SetDefault("MONTHLY_INSIGHTS_SYNC_MAX_CONCURRENT_JOBS", 3)   // 3 jobs concorrentes
	viper.SetDefault("MONTHLY_INSIGHTS_SYNC_ENABLED", false)           // Habilitar sincronização mensal
	viper.SetDefault("MONTHLY_INSIGHTS_SYNC_MONTH_LOOKBACK", 1)        // 1 mês para buscar dados

	viper.SetDefault("TOP_RANKING_ACCOUNTS_CRON", "0 6 * * *")   // Todos os dias às 6h da manhã
	viper.SetDefault("TOP_RANKING_ACCOUNTS_SYNC_ENABLED", false) // Habilitar sincronização de top ranking de contas

	viper.SetDefault("LOG_LEVEL", "debug")
}

func NewConfig() (*Config, error) {
	// Primeiro carregar o arquivo .env usando godotenv
	loadEnvFile() // ONLY LOCAL

	config := &Config{}

	// Configurar valores padrão
	SetDefaults()

	// Configurar o Viper
	viper.SetConfigType("env")
	viper.SetConfigFile(".env")
	viper.AutomaticEnv() // Isso permite que o Viper leia variáveis de ambiente

	// Tentar ler o arquivo .env com o Viper (opcional, já que usamos godotenv)
	if err := viper.ReadInConfig(); err != nil {
		logrus.Info("Usando variáveis carregadas pelo godotenv (viper não conseguiu ler .env):", err)
	} else {
		logrus.Info("Arquivo .env lido pelo Viper com sucesso")
	}

	err := viper.Unmarshal(&config, viper.DecodeHook(
		mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	))
	if err != nil {
		return nil, err
	}

	// Resto do código de configuração
	// renderClient := NewRenderClient(config)
	// secretsByCode := make(map[string]string)
	// if config.Render.ServiceID != "" {
	// 	secretsByCode, err = renderClient.ListSecrets(config.Render.ServiceID)
	// 	if err != nil {
	// 		logrus.Error("Erro ao obter secrets do Render:", err)
	// 		return nil, err
	// 	}
	// }

	secretsByCode := map[string]string{
		"token1":  "vstWEUyFwEXYqe7zezFvP4uuV9MwUS7T96WeSbfPrucJhu7UKTiFAmyrsHpg", // IVS FLORIPA 01
		"token2":  "wdiKmxz5ZgncbAh4PBm9a4AtFEkVA0yundQxdcQkbYLuLqWj4MV9pA7UvwVV", // IVS ERECHIM
		"token3":  "gpbWF2zoSzQr08bKIuNAsWntidCw54LGdqpk9mOBhHTTYcfjWkDhMTVHlZ9x", // IVS CÁCERES
		"token4":  "cmNSHh8qUGb1yBuHuZ6gtvruVZmcsonpUPOStw2qp6uhtFA65XQVo07Nl3Tr", // IVS FORMOSA
		"token5":  "0990e7ppemnDpUnHB6PUm61M0FMjamAzuPoxK2Q5bLNO9D9CuFOxKYW3xnZE", // IVS CORUMBÁ
		"token6":  "0990e7ppemnDpUnHB6PUm61M0FMjamAzuPoxK2Q5bLNO9D9CuFOxKYW3xnZE", // IVS CRICIÚMA
		"token7":  "0990e7ppemnDpUnHB6PUm61M0FMjamAzuPoxK2Q5bLNO9D9CuFOxKYW3xnZE", // IVS DOURADOS
		"token8":  "7FfQv29YEl215Pju8mW1u6oqThDqGwNp4PladjFmUrYjpYcvuMUfjXaIC6Tq", // IVS INDAIATUBA
		"token9":  "X9jNW4RQKQKCtOHQw6naGSnIk6njmYPeejmooMhjO39uLgBLrZADYxMcsNRm", // IVS ITAJAI
		"token10": "2yN0PtPZvpJgczHXdg2cOIi7SCqMhZAjJsUhAymHm8DcKy3RYFPkBNPAeHsA", // IVS JARU
		"token11": "g1jjsEmrfunbljlWFRclTnM5lB9fDFEbBrNz6bnktF3Plo8JpC5ybwI0GZ6Q", // IVS JOINVILLE
		"token12": "q1me0kWUCfki07e0SX5Tkkq11lOSlTgcRdPpAqUL4vcfYMcnIxk3AfAltmOt", // IVS MACEIÓ
		"token13": "T5bIztgSE4l3yQvX9FSIgO0lSwycwkePvG4vJ5x6yjEfMJZzDn6vh2DiuqHH", // IVS PATO BRANCO
	}

	// Configurar token Meta e outras configurações
	metaAccessToken, secretHasMetaAccessToken := secretsByCode["meta_access_token"]
	if config.Meta.AccessToken == "" && secretHasMetaAccessToken {
		config.Meta.AccessToken = metaAccessToken
	}

	config.Meta.URL = fmt.Sprintf("%s/%s", config.Meta.BaseURL, config.Meta.Version)
	config.SSOticaMultiClient = make(map[string]SSOtica)
	for key, token := range secretsByCode {
		config.SSOticaMultiClient[key] = SSOtica{
			URL:         config.SSOtica.URL,
			AccessToken: token,
		}
	}

	config.Database.DSN = fmt.Sprintf(
		"%s://%s:%s@%s",
		config.Database.Driver,
		config.Database.User,
		config.Database.Password,
		config.Database.URL,
	)

	return config, nil
}

// Função auxiliar para carregar o arquivo .env usando godotenv
func loadEnvFile() {
	// Obter diretório atual
	cwd, err := os.Getwd()
	if err != nil {
		logrus.Warn("Não foi possível obter o diretório atual:", err)
		return
	}

	// Tentar várias localizações possíveis para o arquivo .env
	locations := []string{
		filepath.Join(cwd, ".env"),               // Diretório atual
		filepath.Join(filepath.Dir(cwd), ".env"), // Diretório pai
		filepath.Join(cwd, "../.env"),            // Diretório acima
		filepath.Join(cwd, "../../.env"),         // Dois diretórios acima
	}

	for _, location := range locations {
		logrus.Info("Tentando carregar .env de:", location)
		err := godotenv.Load(location)
		if err == nil {
			logrus.Info("Arquivo .env carregado com sucesso de:", location)
			return
		}
	}

	logrus.Warn("Não foi possível carregar o arquivo .env de nenhuma localização conhecida")
}
