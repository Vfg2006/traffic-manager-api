package domain

import "time"

const (
	SocialNetwork = "SocialNetwork"
	Store         = "Store"
)

type Sale struct {
	Date      *time.Time
	NetAmount float64
}

type SalesMetrics struct {
	TotalRevenue  float64
	SalesQuantity int
	AverageTicket float64
	Sales         []*Sale
}
