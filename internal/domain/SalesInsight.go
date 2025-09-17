package domain

import (
	"time"
)

// SalesInsightEntry representa uma entrada de insights de vendas armazenada no banco
type SalesInsightEntry struct {
	ID           int64                    `json:"id"`
	AccountID    string                   `json:"account_id"`
	Date         time.Time                `json:"date"`
	SalesMetrics map[string]*SalesMetrics `json:"sales_metrics"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}
