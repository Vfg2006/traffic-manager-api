package domain

import (
	"time"
)

// MonthlyAdInsightEntry representa uma entrada de insights mensais de anúncios armazenada no banco
type MonthlyAdInsightEntry struct {
	ID         int64             `json:"id"`
	AccountID  string            `json:"account_id"`
	ExternalID string            `json:"external_id"`
	Period     string            `json:"period"` // Período no formato mm-yyyy
	AdMetrics  *AdAccountMetrics `json:"ad_metrics"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}
