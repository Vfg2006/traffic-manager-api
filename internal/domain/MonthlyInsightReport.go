package domain

// MonthlyInsightReport representa um relatório combinado de insights mensais para uma conta
type MonthlyInsightReport struct {
	AccountID     string                   `json:"account_id"`
	AccountName   string                   `json:"account_name,omitempty"`
	ExternalID    string                   `json:"external_id,omitempty"`
	Period        string                   `json:"period"` // Período no formato mm-yyyy
	AdMetrics     *AdAccountMetrics        `json:"ad_metrics,omitempty"`
	SalesMetrics  map[string]*SalesMetrics `json:"sales_metrics,omitempty"`
	ResultMetrics *ResultMetrics           `json:"result_metrics,omitempty"`
}
