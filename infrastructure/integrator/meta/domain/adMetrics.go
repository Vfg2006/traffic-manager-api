package metadomain

type AdAccountMetrics struct {
	AccountID   string             `json:"account_id"`
	Actions     map[string]float64 `json:"actions"`
	Clicks      int                `json:"clicks"`
	Conversions int                `json:"conversions"`
	CTR         float64            `json:"ctr"`
	Frequency   float64            `json:"frequency"`
	Impressions int                `json:"impressions"`
	Name        string             `json:"account_name"`
	Objective   string             `json:"objective"`
	Reach       int                `json:"reach"`
	Spend       float64            `json:"spend"`
}

// Mapeamento de "objective" -> "cost_per_action_type"
var MetaObjectiveToActionType = map[string]string{
	"LINK_CLICKS":           "link_click",
	"POST_ENGAGEMENT":       "post_engagement",
	"PAGE_LIKES":            "like",
	"VIDEO_VIEWS":           "video_view",
	"LEAD_GENERATION":       "lead",
	"CONVERSIONS":           "offsite_conversion",
	"APP_INSTALLS":          "app_install",
	"PRODUCT_CATALOG_SALES": "offsite_conversion.fb_pixel_purchase",
	"MESSAGES":              "onsite_conversion.messaging_first_reply",
	"BRAND_AWARENESS":       "brand_awareness",
	"REACH":                 "reach",
	"STORE_TRAFFIC":         "store_visit",
	"EVENT_RESPONSES":       "rsvp",
	"ADD_TO_CART":           "offsite_conversion.fb_pixel_add_to_cart",
	"PURCHASE":              "offsite_conversion.fb_pixel_purchase",
	"OUTCOME_ENGAGEMENT":    "onsite_conversion.messaging_conversation_started_7d",
}
