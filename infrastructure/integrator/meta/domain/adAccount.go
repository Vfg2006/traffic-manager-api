package metadomain

type AdAccount struct {
	BusinessManagerID   string `json:"business_id"`
	BusinessManagerName string `json:"business_name"`
	ID                  string `json:"id"`
	Name                string `json:"name"`
}

type AdAccountInsight struct {
	AccountID      string   `json:"account_id"`
	Actions        []Action `json:"actions"`
	Clicks         int      `json:"clicks"`
	Conversions    int      `json:"conversions"`
	CostPerActions []Action `json:"cost_per_action_type"`
	CTR            float64  `json:"ctr"`
	DateStart      string   `json:"date_start"`
	DateStop       string   `json:"date_stop"`
	Frequency      string   `json:"frequency"`
	Impressions    string   `json:"impressions"`
	Name           string   `json:"account_name"`
	Objective      string   `json:"objective"`
	Reach          string   `json:"reach"`
	Spend          string   `json:"spend"`
}
