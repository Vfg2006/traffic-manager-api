package domain

type Campaign struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CampaignInsight struct {
	CampaignID    string  `json:"campaign_id"`
	CampaignName  string  `json:"campaign_name"`
	Clicks        string  `json:"clicks"`
	CostPerResult float64 `json:"cost_per_result"`
	Frequency     string  `json:"frequency"`
	Impressions   string  `json:"impressions"`
	Objective     string  `json:"objective"`
	Reach         string  `json:"reach"`
	Result        int     `json:"result"`
	Spend         float64 `json:"spend"`
}
