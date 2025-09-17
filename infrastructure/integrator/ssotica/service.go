package ssotica

import (
	"time"

	ssoticadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/domain"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/ssoticaclient"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

type SSOticaIntegrator interface {
	GetSalesByAccount(params ssoticadomain.GetSalesParams, filters *domain.InsigthFilters) ([]ssoticadomain.Order, error)
	CheckConnection(params ssoticadomain.CheckConnectionParams) (bool, error)
}

type SSOticaService struct {
	cfg    *config.Config
	Client ssoticaclient.Client
}

func New(cfg *config.Config, client ssoticaclient.Client) SSOticaIntegrator {
	return &SSOticaService{
		cfg:    cfg,
		Client: client,
	}
}

func (s *SSOticaService) GetSalesByAccount(params ssoticadomain.GetSalesParams, filters *domain.InsigthFilters) ([]ssoticadomain.Order, error) {
	ssoticaConfig := s.cfg.SSOticaMultiClient[params.SecretName]

	paramsClient := ssoticaclient.SalesConsultationParams{
		StartDate: filters.StartDate.Format(time.DateOnly),
		EndDate:   filters.EndDate.Format(time.DateOnly),
		CNPJ:      params.CNPJ,
		Token:     ssoticaConfig.AccessToken,
	}

	resp, err := s.Client.GetSales(paramsClient, &ssoticaConfig)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *SSOticaService) CheckConnection(params ssoticadomain.CheckConnectionParams) (bool, error) {
	paramsClient := ssoticaclient.SalesConsultationParams{
		StartDate: params.StartDate.Format(time.DateOnly),
		EndDate:   params.EndDate.Format(time.DateOnly),
		CNPJ:      params.CNPJ,
	}

	s.cfg.SSOtica.AccessToken = params.Token

	_, err := s.Client.GetSales(paramsClient, &s.cfg.SSOtica)
	if err != nil {
		return false, err
	}

	return true, nil
}
