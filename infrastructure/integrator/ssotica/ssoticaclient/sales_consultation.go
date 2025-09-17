package ssoticaclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	ssoticadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/ssotica/domain"
	"github.com/vfg2006/traffic-manager-api/internal/config"
)

type SalesConsultationParams struct {
	StartDate string
	EndDate   string
	CNPJ      string
	Token     string
}

type SalesConsultationResponse []ssoticadomain.Order

func (c *SSOticaClient) GetSales(params SalesConsultationParams, ssoticaConfig *config.SSOtica) (SalesConsultationResponse, error) {
	var response SalesConsultationResponse

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Construir a URL da requisição.
	endpoint, err := url.Parse(ssoticaConfig.URL)
	if err != nil {
		return response, fmt.Errorf("erro ao analisar a URL base: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/integracoes/vendas/periodo")

	// Adicionar parâmetros de consulta.
	query := endpoint.Query()
	query.Set("inicio_periodo", params.StartDate)
	query.Set("fim_periodo", params.EndDate)
	query.Set("cnpj", params.CNPJ)
	endpoint.RawQuery = query.Encode()

	// Criar a requisição HTTP.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return response, fmt.Errorf("erro ao criar a requisição: %w", err)
	}

	// Adicionar cabeçalhos necessários.
	req.Header.Set("Authorization", "Bearer "+ssoticaConfig.AccessToken)
	req.Header.Set("Accept", "application/json")

	// Executar a requisição.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return response, fmt.Errorf("erro ao executar a requisição: %w", err)
	}
	defer resp.Body.Close()

	// Verificar o código de status da resposta.
	if resp.StatusCode != http.StatusOK {
		return response, fmt.Errorf("requisição falhou com status: %s", resp.Status)
	}

	// Decodificar a resposta JSON.
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return response, fmt.Errorf("erro ao decodificar a resposta: %w", err)
	}

	return response, nil
}
