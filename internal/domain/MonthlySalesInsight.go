package domain

import (
	"time"
)

// MonthlySalesInsightEntry representa uma entrada de insights mensais de vendas armazenada no banco
type MonthlySalesInsightEntry struct {
	ID           int64                    `json:"id"`
	AccountID    string                   `json:"account_id"`
	Period       string                   `json:"period"` // Período no formato mm-yyyy
	SalesMetrics map[string]*SalesMetrics `json:"sales_metrics"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}
