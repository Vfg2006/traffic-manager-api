package domain

import (
	"time"
)

type BusinessManager struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ExternalID string `json:"external_id"`
	Origin     string `json:"origin"`
}

type AdAccountStatus string

const (
	AdAccountStatusActive   AdAccountStatus = "ACTIVE"
	AdAccountStatusInactive AdAccountStatus = "INACTIVE"
)

type AdAccount struct {
	BusinessManagerID   string          `json:"business_id"`
	BusinessManagerName string          `json:"business_name"`
	CNPJ                *string         `json:"cnpj"`
	ExternalID          string          `json:"external_id"`
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Nickname            *string         `json:"nickname"`
	Origin              string          `json:"origin"`
	SecretName          *string         `json:"secret_name"`
	Status              AdAccountStatus `json:"status"`
}

type AdAccountResponse struct {
	CNPJ       *string         `json:"cnpj"`
	ExternalID string          `json:"external_id"`
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Nickname   *string         `json:"nickname"`
	HasToken   bool            `json:"hasToken"`
	Status     AdAccountStatus `json:"status"`
}

type AdAccountInsight struct {
	AccountID     string             `json:"account_id"`
	Campaigns     []*CampaignInsight `json:"ad_campaigns"`
	CostPerResult float64            `json:"cost_per_result"`
	Frequency     float64            `json:"frequency"`
	Impressions   int                `json:"impressions"`
	Name          string             `json:"account_name"`
	Objective     string             `json:"objective"`
	Reach         int                `json:"reach"`
	Result        int                `json:"result"`
	Spend         float64            `json:"spend"`
}

type AdAccountMetrics struct {
	AdAccountInsight
	CostPerResultByDate map[string]float64 `json:"cost_per_result_by_date"`
	ResultByDate        map[string]int     `json:"result_by_date"`
}

func (m *AdAccountMetrics) IsEmpty() bool {
	if m == nil {
		return true
	}

	return m.Impressions == 0 && m.Reach == 0 && m.Result == 0 && m.Spend == 0
}

func isSameDate(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

type UpdateAdAccountRequest struct {
	ID         string  `json:"id"`
	Nickname   *string `json:"nickname,omitempty"`
	CNPJ       *string `json:"cnpj,omitempty"`
	SecretName *string `json:"secret_name,omitempty"`
	Token      *string `json:"token,omitempty"`
	Status     *string `json:"status,omitempty"`
}

type UpdateAdAccountResponse struct {
	ID         string  `json:"id"`
	Nickname   *string `json:"nickname,omitempty"`
	CNPJ       *string `json:"cnpj,omitempty"`
	SecretName *string `json:"secret_name,omitempty"`
	Status     *string `json:"status,omitempty"`
}

type SyncAccountsResponse struct {
	Quantity int    `json:"quantity"`
	Message  string `json:"message"`
	Error    bool   `json:"error"`
}
