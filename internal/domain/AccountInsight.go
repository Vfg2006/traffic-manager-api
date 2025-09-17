package domain

import (
	"time"
)

// AccountInsightEntry representa uma entrada de insights armazenada no banco
type AccountInsightEntry struct {
	ID            int64                    `json:"id"`
	AccountID     string                   `json:"account_id"`
	ExternalID    string                   `json:"external_id"`
	Date          time.Time                `json:"date"`
	AdMetrics     *AdAccountMetrics        `json:"ad_metrics"`
	SalesMetrics  map[string]*SalesMetrics `json:"sales_metrics"`
	ResultMetrics *ResultMetrics           `json:"result_metrics"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

// ReachImpressionsResponse representa a resposta do endpoint de Reach e Impressions
type ReachImpressionsResponse struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	Reach       int    `json:"reach"`
	Impressions int    `json:"impressions"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
}
