package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type SecretStorage interface {
	ListSecrets(serviceID string) (map[string]string, error)
	AddOrUpdateSecret(serviceID, secretName, secretContent string) error
}

type AddOrUpdateSecretRequest struct {
	Content string `json:"content"`
}

type RenderClient struct {
	APIKey     string
	HTTPClient *http.Client
}

func NewRenderClient(config *Config) *RenderClient {
	return &RenderClient{
		APIKey:     config.Render.APIKey,
		HTTPClient: &http.Client{},
	}
}

func (c *RenderClient) ListSecrets(serviceID string) (map[string]string, error) {
	url := fmt.Sprintf("https://api.render.com/v1/services/%s/secret-files?limit=100", serviceID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("config: error list secrets: %s", body)
	}

	var response []struct {
		SecretFile struct {
			Content string `json:"content"`
			Name    string `json:"name"`
		} `json:"secretFile"`
		Cursor string `json:"cursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	secretsMap := make(map[string]string)
	for _, sf := range response {
		secretsMap[sf.SecretFile.Name] = sf.SecretFile.Content
	}

	return secretsMap, nil
}

func (c *RenderClient) AddOrUpdateSecret(serviceID, secretName, secretContent string) error {
	url := fmt.Sprintf("https://api.render.com/v1/services/%s/secret-files/%s", serviceID, secretName)

	reqBody := AddOrUpdateSecretRequest{
		Content: secretContent,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("config: Error add or update secret: %s", body)
	}
	return nil
}
