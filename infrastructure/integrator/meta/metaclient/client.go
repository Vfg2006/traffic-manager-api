package metaclient

import (
	"net/http"
	"net/url"

	metadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta/domain"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

type Client interface {
	GetAdAccountInsightsByID(accountID string, filters *domain.InsigthFilters, params *url.Values) (*metadomain.AdAccountInsight, error)
	GetAdCampaignByAccountID(accountID string) ([]metadomain.Campaign, error)
	GetAdCampaignInsightsByID(campaignID string, filters *domain.InsigthFilters) (*metadomain.CampaignInsight, error)
	GetAdAccountsByBusinessID(businessID string) ([]metadomain.AdAccount, error)
	RefreshToken() error
	EnsureValidToken() error
	HandleResponse(resp *http.Response) ([]byte, error)
}

type MetaClient struct {
	Cfg          *config.Config
	TokenManager *TokenManager
}

func NewClient(cfg *config.Config, tokenManager *TokenManager) Client {
	client := &MetaClient{
		Cfg:          cfg,
		TokenManager: tokenManager,
	}
	return client
}

// RefreshToken obtém um novo token de longa duração
func (c *MetaClient) RefreshToken() error {
	return c.TokenManager.RefreshToken()
}

// EnsureValidToken verifica se o token atual é válido e tenta renová-lo se necessário
func (c *MetaClient) EnsureValidToken() error {
	return c.TokenManager.EnsureValidToken()
}

// HandleResponse manipula a resposta HTTP e verifica erros de token expirado
func (c *MetaClient) HandleResponse(resp *http.Response) ([]byte, error) {
	return c.TokenManager.HandleResponse(resp)
}
