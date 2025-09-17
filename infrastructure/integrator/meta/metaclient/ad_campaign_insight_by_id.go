package metaclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	metadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta/domain"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

type ResponseAdCampaignInsight struct {
	Data   []metadomain.CampaignInsight `json:"data"`
	Paging metadomain.Paging            `json:"paging"`
}

func (c *MetaClient) GetAdCampaignInsightsByID(campaignID string, filters *domain.InsigthFilters) (*metadomain.CampaignInsight, error) {
	// Garantir que o token seja válido antes de fazer a requisição
	if err := c.EnsureValidToken(); err != nil {
		return nil, fmt.Errorf("erro ao verificar validade do token: %w", err)
	}

	baseURL := fmt.Sprintf("%s/%s/insights", c.Cfg.Meta.URL, campaignID)

	timeRange := fmt.Sprintf("{\"since\":\"%s\",\"until\":\"%s\"}", filters.StartDate.Format(time.DateOnly), filters.EndDate.Format(time.DateOnly))

	params := url.Values{}
	params.Add("fields", "account_id,account_name,campaign_name,campaign_id,spend,impressions,frequency,reach,objective,clicks,actions,cost_per_action_type")
	params.Add("filtering", "[{\"field\":\"objective\",\"operator\":\"IN\",\"value\":[\"OUTCOME_ENGAGEMENT\"]}]")
	params.Add("time_range", timeRange)
	params.Add("access_token", c.Cfg.Meta.AccessToken)

	url := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.WithError(err).Error("Erro ao criar a requisição")
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.WithError(err).Error("Erro ao fazer a requisição")
		return nil, err
	}
	defer resp.Body.Close()

	// Usar o novo manipulador de resposta que verifica tokens expirados
	body, err := c.HandleResponse(resp)
	if err != nil {
		// Se o erro indica que o token foi renovado, tentar novamente
		if err.Error() == "token expirado e renovado, por favor tente novamente" {
			return c.GetAdCampaignInsightsByID(campaignID, filters)
		}
		return nil, err
	}

	var response ResponseAdCampaignInsight
	if err := json.Unmarshal(body, &response); err != nil {
		logrus.WithError(err).Error("Erro ao decodificar JSON")
		return nil, err
	}

	if response.Data == nil || len(response.Data) == 0 {
		return nil, errors.New("no data found")
	}

	return &response.Data[0], nil
}
