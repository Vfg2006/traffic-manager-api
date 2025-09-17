// Package domain contém as estruturas de dados do domínio da aplicação
package domain

import "time"

type StoreRankingResponse struct {
	Ranking    []StoreRankingItem `json:"ranking"`
	LastUpdate time.Time          `json:"last_update"`
}

type StoreRankingItem struct {
	ID                   int       `json:"id"`
	AccountID            string    `json:"account_id"`
	Month                string    `json:"month"` // Formato mm-yyyy (ex: 01-2024)
	StoreName            string    `json:"store_name"`
	SocialNetworkRevenue float64   `json:"social_network_revenue"`
	Position             int       `json:"position"`
	PositionChange       int       `json:"position_change"` // Valor positivo = subiu, negativo = desceu, 0 = manteve
	PreviousPosition     int       `json:"previous_position"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
