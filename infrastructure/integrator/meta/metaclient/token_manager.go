package metaclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	metadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta/domain"
	"github.com/vfg2006/traffic-manager-api/internal/config"

	"github.com/sirupsen/logrus"
)

// TokenManager gerencia tokens de acesso da API do Meta
type TokenManager struct {
	cfg               *config.Config
	TokenRefreshMutex sync.Mutex `mapstructure:"-"`
	stopRefresh       chan struct{}
	RenderClient      *config.RenderClient
}

// NewTokenManager cria uma nova instância do gerenciador de tokens
func NewTokenManager(cfg *config.Config, renderClient *config.RenderClient) *TokenManager {
	return &TokenManager{
		cfg:               cfg,
		TokenRefreshMutex: sync.Mutex{},
		stopRefresh:       make(chan struct{}),
		RenderClient:      renderClient,
	}
}

func (tm *TokenManager) InitToken() {
	// Inicializa o token de longa duração se necessário
	if tm.cfg.Meta.LongLivedToken == "" {
		logrus.Info("Token de longa duração não encontrado. Iniciando processo de obtenção...")
		if err := tm.InitiateToken(); err != nil {
			logrus.Errorf("Falha ao inicializar token de longa duração: %v", err)
			logrus.Warn("A API Meta pode ter funcionalidade limitada até que o token seja configurado corretamente")
		}

		logrus.Info("Token de longa duração inicializado com sucesso")

	} else if tm.cfg.Meta.TokenExpiresAt.IsZero() {
		// Se já existe um token de longa duração, mas não sabemos quando expira
		// vamos validar e obter mais informações sobre ele
		logrus.Info("Validando token de longa duração existente...")
		if err := tm.ValidateExistingToken(); err != nil {
			logrus.Errorf("Falha ao validar token existente: %v", err)
			logrus.Warn("Tentando renovar o token...")
			if err := tm.RefreshToken(); err != nil {
				logrus.Errorf("Falha ao renovar token: %v", err)
				logrus.Warn("A API Meta pode ter funcionalidade limitada até que o token seja renovado")
			}
		} else {
			logrus.Info("Token de longa duração validado com sucesso")
		}
	} else {
		// Garantir que o token seja válido, mesmo que a data de expiração esteja definida
		if err := tm.EnsureValidToken(); err != nil {
			logrus.Errorf("Erro ao verificar validade do token: %v", err)
		}
	}
}

// startAutoRefresh inicia uma goroutine que atualiza o token periodicamente
func (tm *TokenManager) StartAutoRefresh() {
	if err := tm.InitiateToken(); err != nil {
		logrus.Errorf("Erro ao iniciar o token: %v", err)
	}

	// Definir tempo para renovação diária (aproximadamente 23 horas para garantir que seja feito antes de 24h)
	refreshInterval := 23 * time.Hour
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Tenta renovar o token periodicamente
			logrus.Info("Iniciando renovação periódica do token da Meta")
			if err := tm.RefreshToken(); err != nil {
				logrus.Errorf("Erro na renovação periódica do token: %v", err)

				// Se falhar, tente novamente em um intervalo mais curto
				ticker.Reset(1 * time.Hour)
			} else {
				logrus.Info("Renovação periódica do token concluída com sucesso")

				tm.RenderClient.AddOrUpdateSecret(tm.cfg.Render.ServiceID, "meta_access_token", tm.cfg.Meta.AccessToken)

				// Restaurar para o intervalo normal
				ticker.Reset(refreshInterval)
			}
		case <-tm.stopRefresh:
			logrus.Info("Encerrando goroutine de renovação periódica do token")
			return
		}
	}
}

// StopAutoRefresh para a goroutine de renovação automática
func (tm *TokenManager) StopAutoRefresh() {
	close(tm.stopRefresh)
}

// InitiateToken obtém um token de longa duração a partir do token de curta duração
func (tm *TokenManager) InitiateToken() error {
	tm.TokenRefreshMutex.Lock()
	defer tm.TokenRefreshMutex.Unlock()

	// Verificar novamente se o token já foi inicializado por outra goroutine
	if tm.cfg.Meta.LongLivedToken != "" {
		return nil
	}

	// Obter token de longa duração
	tokenResponse, err := GetLongLivedToken(
		tm.cfg.Meta.AccessToken,
		tm.cfg.Meta.AppID,
		tm.cfg.Meta.AppSecret,
		tm.cfg.Meta.BaseURL,
		tm.cfg.Meta.Version,
	)
	if err != nil {
		return fmt.Errorf("erro ao obter token de longa duração: %w", err)
	}

	// Atualizar a configuração
	tm.cfg.Meta.LongLivedToken = tokenResponse.AccessToken
	tm.cfg.Meta.TokenExpiresAt = CalculateTokenExpiration(tokenResponse.ExpiresIn)

	// Atualizar o token de acesso para usar o token de longa duração
	tm.cfg.Meta.AccessToken = tm.cfg.Meta.LongLivedToken

	tm.RenderClient.AddOrUpdateSecret(tm.cfg.Render.ServiceID, "meta_access_token", tm.cfg.Meta.AccessToken)

	logrus.Infof("Token de longa duração inicializado com sucesso. Expira em: %s",
		tm.cfg.Meta.TokenExpiresAt.Format(time.RFC3339))

	return nil
}

// ValidateExistingToken valida um token existente e atualiza as informações de expiração
func (tm *TokenManager) ValidateExistingToken() error {
	tm.TokenRefreshMutex.Lock()
	defer tm.TokenRefreshMutex.Unlock()

	// Verificar se o token de longa duração é válido
	isValid, err := CheckTokenValidity(tm.cfg.Meta.LongLivedToken, tm.cfg.Meta.URL)
	if err != nil {
		return fmt.Errorf("erro ao verificar validade do token de longa duração: %w", err)
	}

	if !isValid {
		// Se não é válido, tenta obter um novo token
		return tm.refreshTokenInternal()
	}

	// Obter informações sobre o token de longa duração
	debugInfo, err := GetDebugTokenInfo(
		tm.cfg.Meta.LongLivedToken,
		tm.cfg.Meta.AppID,
		tm.cfg.Meta.AppSecret,
		tm.cfg.Meta.BaseURL,
		tm.cfg.Meta.Version,
	)
	if err != nil {
		return fmt.Errorf("erro ao obter informações do token: %w", err)
	}

	// Extrair a data de expiração
	if data, ok := debugInfo["data"].(map[string]interface{}); ok {
		if expiresAt, ok := data["expires_at"].(float64); ok {
			tm.cfg.Meta.TokenExpiresAt = time.Unix(int64(expiresAt), 0)
			// Subtrair alguns dias para garantir que renovemos antes da expiração
			tm.cfg.Meta.TokenExpiresAt = tm.cfg.Meta.TokenExpiresAt.Add(-24 * time.Hour)

			logrus.Infof("Token de longa duração é válido. Expira em: %s",
				tm.cfg.Meta.TokenExpiresAt.Format(time.RFC3339))

			// Atualizar o token de acesso para usar o token de longa duração
			tm.cfg.Meta.AccessToken = tm.cfg.Meta.LongLivedToken

			return nil
		}
	}

	return fmt.Errorf("não foi possível determinar quando o token expira")
}

// RefreshToken obtém um novo token de longa duração
func (tm *TokenManager) RefreshToken() error {
	return tm.refreshTokenInternal()
}

// refreshTokenInternal é a implementação interna do refresh de token
func (tm *TokenManager) refreshTokenInternal() error {
	tm.TokenRefreshMutex.Lock()
	defer tm.TokenRefreshMutex.Unlock()

	// Verifica se já não estamos muito próximos da expiração
	if !tm.cfg.Meta.TokenExpiresAt.IsZero() && time.Until(tm.cfg.Meta.TokenExpiresAt) < 1*time.Hour {
		logrus.Warn("Token está muito próximo da expiração ou já expirou - pode ser necessária reautorização manual")
	}

	// Tenta obter um novo token de longa duração a partir do token existente
	logrus.Info("Iniciando renovação do token...")
	tokenResponse, err := GetLongLivedToken(
		tm.cfg.Meta.AccessToken,
		tm.cfg.Meta.AppID,
		tm.cfg.Meta.AppSecret,
		tm.cfg.Meta.BaseURL,
		tm.cfg.Meta.Version,
	)
	if err != nil {
		errMsg := err.Error()

		// Verifica se a mensagem de erro contém informações sobre expiração
		if strings.Contains(errMsg, "Error validating access token") ||
			strings.Contains(errMsg, "Session has expired") ||
			strings.Contains(errMsg, "The session has been invalidated") {

			logrus.Error("O token de acesso expirou e não pode ser renovado automaticamente. É necessário reautorizar")

			// Notificar por log a necessidade de reautorização
			return fmt.Errorf("o token de acesso expirou e não pode ser renovado automaticamente. "+
				"É necessário reautorizar o aplicativo através do processo de autenticação OAuth: %w", err)
		}

		// Outros erros
		logrus.Errorf("Erro ao renovar token: %v", err)
		return fmt.Errorf("erro ao obter novo token de longa duração: %w", err)
	}

	// Atualizar a configuração
	oldToken := tm.cfg.Meta.LongLivedToken
	tm.cfg.Meta.LongLivedToken = tokenResponse.AccessToken
	tm.cfg.Meta.TokenExpiresAt = CalculateTokenExpiration(tokenResponse.ExpiresIn)
	tm.cfg.Meta.AccessToken = tm.cfg.Meta.LongLivedToken

	// Logar informações sobre o novo token (com cuidado para não expor o token completo)
	if oldToken != tm.cfg.Meta.LongLivedToken {
		logrus.Infof("Token de longa duração atualizado com sucesso. Expira em: %s",
			tm.cfg.Meta.TokenExpiresAt.Format(time.RFC3339))
	} else {
		logrus.Info("Token renovado, mas não mudou. Isso pode indicar um problema na API da Meta")
	}

	return nil
}

// EnsureValidToken verifica se o token atual é válido e tenta renová-lo se necessário
func (tm *TokenManager) EnsureValidToken() error {
	// Se o token está nulo ou vazio, precisamos inicializá-lo
	if tm.cfg.Meta.AccessToken == "" {
		logrus.Info("Token não inicializado. Inicializando...")
		return tm.InitiateToken()
	}

	// Verificar se o token está prestes a expirar (menos de 24 horas)
	// Nota: estamos sendo conservadores aqui para garantir que sempre tenhamos um token válido
	if time.Until(tm.cfg.Meta.TokenExpiresAt) < 24*time.Hour {
		logrus.Info("Token expira em menos de 24 horas. Renovando proativamente...")
		return tm.RefreshToken()
	}

	return nil
}

// ParseErrorResponse tenta parsear um erro da API do Meta
func ParseErrorResponse(body []byte) (*metadomain.ErrorResponse, error) {
	var errorResp metadomain.ErrorResponse
	err := json.Unmarshal(body, &errorResp)
	if err != nil {
		return nil, err
	}
	return &errorResp, nil
}

// HandleResponse manipula a resposta HTTP e verifica erros de token expirado
func (tm *TokenManager) HandleResponse(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	// Se a resposta for bem-sucedida, retorna o corpo
	if resp.StatusCode == http.StatusOK {
		return body, nil
	}

	// Processa erro na resposta da API
	return tm.handleErrorResponse(body)
}

// handleErrorResponse processa erros nas respostas da API
func (tm *TokenManager) handleErrorResponse(body []byte) ([]byte, error) {
	// Primeiro tentar parsear como JSON
	errorResp, parseErr := ParseErrorResponse(body)

	// Verificar se é erro de token expirado pela estrutura JSON
	if parseErr == nil && errorResp.IsTokenExpired() {
		return tm.handleExpiredToken(errorResp)
	}

	// Verificar pela mensagem de erro em texto
	bodyStr := string(body)
	if containsTokenExpirationMessage(bodyStr) {
		return tm.handleExpiredTokenByMessage(bodyStr)
	}

	return nil, fmt.Errorf("erro na resposta da API. Status: %d, Corpo: %s", http.StatusBadRequest, string(body))
}

// handleExpiredToken trata um token expirado detectado via estrutura de erro
func (tm *TokenManager) handleExpiredToken(errorResp *metadomain.ErrorResponse) ([]byte, error) {
	logrus.Warnf("Token expirado detectado pela API Meta. Código: %d, Subcódigo: %d",
		errorResp.Error.Code, errorResp.Error.ErrorSubcode)

	// Tenta renovar o token
	if refreshErr := tm.RefreshToken(); refreshErr != nil {
		if strings.Contains(refreshErr.Error(), "necessário reautorizar") {
			return nil, fmt.Errorf("token expirou permanentemente e requer reautorização manual: %w", refreshErr)
		}
		return nil, fmt.Errorf("erro ao renovar token expirado: %w", refreshErr)
	}

	return nil, fmt.Errorf("token expirado e renovado, por favor tente novamente")
}

// handleExpiredTokenByMessage trata um token expirado detectado via mensagem de texto
func (tm *TokenManager) handleExpiredTokenByMessage(bodyStr string) ([]byte, error) {
	logrus.Warnf("Token expirado detectado pela mensagem de erro: %s", bodyStr)

	// Tenta renovar o token
	if refreshErr := tm.RefreshToken(); refreshErr != nil {
		if strings.Contains(refreshErr.Error(), "necessário reautorizar") {
			return nil, fmt.Errorf("token expirou permanentemente e requer reautorização manual: %w", refreshErr)
		}
		return nil, fmt.Errorf("erro ao renovar token expirado: %w", refreshErr)
	}

	return nil, fmt.Errorf("token expirado e renovado, por favor tente novamente")
}

// containsTokenExpirationMessage verifica se a mensagem contém indicação de token expirado
func containsTokenExpirationMessage(message string) bool {
	return strings.Contains(message, "Error validating access token") ||
		strings.Contains(message, "Session has expired") ||
		strings.Contains(message, "The session has been invalidated")
}
