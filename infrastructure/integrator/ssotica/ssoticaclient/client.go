package ssoticaclient

import (
	"net/http"
	"time"

	"github.com/vfg2006/traffic-manager-api/internal/config"
)

type Client interface {
	GetSales(params SalesConsultationParams, ssoticaConfig *config.SSOtica) (SalesConsultationResponse, error)
}

type SSOticaClient struct {
	httpClient *http.Client
	config     *config.Config
}

// NovoClienteAPI cria uma nova instância de clienteAPI.
func NewClient(cfg *config.Config) Client {
	return &SSOticaClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: cfg,
	}
}
