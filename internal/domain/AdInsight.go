package domain

import (
	"time"
)

// AdInsightEntry representa uma entrada de insights de anúncios armazenada no banco
type AdInsightEntry struct {
	ID         int64             `json:"id"`
	AccountID  string            `json:"account_id"`
	ExternalID string            `json:"external_id"`
	Date       time.Time         `json:"date"`
	AdMetrics  *AdAccountMetrics `json:"ad_metrics"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}
