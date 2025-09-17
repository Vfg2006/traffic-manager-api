package metaclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
)

// TokenResponse representa a resposta da API do Meta ao trocar um token
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// GetLongLivedToken obtém um token de longa duração do Meta
// usando um token de curta duração
func GetLongLivedToken(shortLivedToken, appID, appSecret, baseURL, version string) (*TokenResponse, error) {
	if shortLivedToken == "" {
		return nil, fmt.Errorf("token de acesso não pode ser vazio")
	}

	endpoint := fmt.Sprintf("%s/%s/oauth/access_token", baseURL, version)

	params := url.Values{}
	params.Add("grant_type", "fb_exchange_token")
	params.Add("client_id", appID)
	params.Add("client_secret", appSecret)
	params.Add("fb_exchange_token", shortLivedToken)

	requestURL := endpoint + "?" + params.Encode()

	// Usar um cliente HTTP com timeout adequado
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("erro ao obter token de longa duração: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logrus.Errorf("Erro obtendo token longa duração. Status: %d, Resposta: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("erro ao obter token de longa duração. Status: %d, Resposta: %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("token retornado pela API é vazio")
	}

	expiresInText := FormatDuration(tokenResp.ExpiresIn)
	logrus.Infof("Token de longa duração obtido com sucesso. Expira em %s.", expiresInText)

	return &tokenResp, nil
}

// FormatDuration formata a duração em segundos para um formato legível
func FormatDuration(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	days := duration / (24 * time.Hour)
	hours := (duration % (24 * time.Hour)) / time.Hour
	minutes := (duration % time.Hour) / time.Minute

	return fmt.Sprintf("%d dias, %d horas e %d minutos", days, hours, minutes)
}

// CheckTokenValidity verifica se o token é válido fazendo uma consulta simples à API
func CheckTokenValidity(token, apiURL string) (bool, error) {
	if token == "" {
		return false, fmt.Errorf("token não pode ser vazio")
	}

	// Verificamos a validade do token consultando o /me endpoint
	requestURL := fmt.Sprintf("%s/me?fields=id,name&access_token=%s", apiURL, token)

	// Usar um cliente HTTP com timeout adequado
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(requestURL)
	if err != nil {
		return false, fmt.Errorf("erro ao verificar token: %w", err)
	}
	defer resp.Body.Close()

	// Se o status for diferente de 200, o token pode ter expirado ou ser inválido
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logrus.Warnf("Token inválido ou expirado. Status: %d, Corpo: %s", resp.StatusCode, string(body))
		return false, nil
	}

	return true, nil
}

// GetDebugTokenInfo obtém informações de debug sobre um token do Meta
func GetDebugTokenInfo(token, appID, appSecret, baseURL, version string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/%s/debug_token", baseURL, version)

	params := url.Values{}
	params.Add("input_token", token)
	params.Add("access_token", appID+"|"+appSecret)

	requestURL := endpoint + "?" + params.Encode()

	resp, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("erro ao obter informações de debug do token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro ao obter informações de debug do token. Status: %d, Resposta: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	return response, nil
}

// CalculateTokenExpiration calcula a data de expiração do token com base no tempo de expiração em segundos
func CalculateTokenExpiration(expiresIn int64) time.Time {
	// Subtraímos 1 dia para renovar antes da expiração real
	buffer := int64(24 * 60 * 60) // 1 dia em segundos
	safeExpiresIn := expiresIn - buffer

	if safeExpiresIn < 0 {
		safeExpiresIn = expiresIn / 2 // Se for muito curto, usamos metade do tempo
	}

	return time.Now().Add(time.Duration(safeExpiresIn) * time.Second)
}
