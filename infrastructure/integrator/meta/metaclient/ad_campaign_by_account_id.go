package metaclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/sirupsen/logrus"
	metadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta/domain"
)

type ResponseAdCampaign struct {
	Data   []metadomain.Campaign `json:"data"`
	Paging metadomain.Paging     `json:"paging"`
}

// TODO adicionar loop para pegar todas as páginas
func (c *MetaClient) GetAdCampaignByAccountID(accountID string) ([]metadomain.Campaign, error) {
	// Garantir que o token seja válido antes de fazer a requisição
	if err := c.EnsureValidToken(); err != nil {
		return nil, fmt.Errorf("erro ao verificar validade do token: %w", err)
	}

	baseURL := fmt.Sprintf("%s/act_%s/campaigns", c.Cfg.Meta.URL, accountID)

	params := url.Values{}
	params.Add("fields", "id,name,status")
	params.Add("effective_status", "['ACTIVE']")
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
			return c.GetAdCampaignByAccountID(accountID)
		}
		return nil, err
	}

	var response ResponseAdCampaign
	if err := json.Unmarshal(body, &response); err != nil {
		logrus.WithError(err).Error("Erro ao decodificar JSON")
		return nil, err
	}

	if response.Data == nil {
		return nil, errors.New("no data found")
	}

	return response.Data, nil
}
