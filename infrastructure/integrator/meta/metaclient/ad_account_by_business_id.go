package metaclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/sirupsen/logrus"
	metadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta/domain"
)

type ResponseAdAccount struct {
	Data []metadomain.AdAccount `json:"data"`
}

type ResponseBatchAdAccount []struct {
	Code    int `json:"code"`
	Headers []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	Body ResponseAdAccount `json:"body"`
}

type BatchField struct {
	Method      string `json:"method"`
	RelativeURL string `json:"relative_url"`
}

// TODO fazer iteração para pegar todos os dados
func (c *MetaClient) GetAdAccountsByBusinessID(businessID string) ([]metadomain.AdAccount, error) {
	// Garantir que o token seja válido antes de fazer a requisição
	if err := c.EnsureValidToken(); err != nil {
		return nil, fmt.Errorf("erro ao verificar validade do token: %w", err)
	}

	baseURL := fmt.Sprintf("%s/%s/owned_ad_accounts", c.Cfg.Meta.URL, businessID)

	params := url.Values{}
	params.Add("fields", "id,name")
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
			return c.GetAdAccountsByBusinessID(businessID)
		}
		return nil, err
	}

	var response ResponseAdAccount
	if err := json.Unmarshal(body, &response); err != nil {
		logrus.WithError(err).Error("Erro ao decodificar JSON")
		return nil, err
	}

	if response.Data == nil {
		return nil, fmt.Errorf("No data found")
	}

	return response.Data, nil
}

func (c *MetaClient) BatchGetAdAccountsByBusinessID(batchFields []BatchField) ([]metadomain.AdAccount, error) {
	// Garantir que o token seja válido antes de fazer a requisição
	if err := c.EnsureValidToken(); err != nil {
		return nil, fmt.Errorf("erro ao verificar validade do token: %w", err)
	}

	batch, err := json.Marshal(batchFields)

	params := url.Values{}
	params.Add("Batch", string(batch))
	params.Add("access_token", c.Cfg.Meta.AccessToken)

	url := c.Cfg.Meta.URL + "?" + params.Encode()

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
			return c.BatchGetAdAccountsByBusinessID(batchFields)
		}
		return nil, err
	}

	var response ResponseBatchAdAccount
	if err := json.Unmarshal(body, &response); err != nil {
		logrus.WithError(err).Error("Erro ao decodificar JSON")
		return nil, err
	}

	if response == nil {
		return nil, fmt.Errorf("No data found")
	}

	results := make([]metadomain.AdAccount, 0)
	for _, r := range response {
		if r.Body.Data != nil {
			results = append(results, r.Body.Data...)
		}
	}

	return results, nil
}
