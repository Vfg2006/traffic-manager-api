// Package repository contém as implementações dos repositórios para acesso aos dados
package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/vfg2006/traffic-manager-api/infrastructure/database/postgres"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

const (
	storeRankingTable = "store_ranking sr"
)

type StoreRankingRepository interface {
	GetByAccountID(accountID string, month string) (*domain.StoreRankingItem, error)
	GetStoreRanking() (*domain.StoreRankingResponse, error)
	SaveOrUpdateStoreRanking(rankings []*domain.StoreRankingItem) error
}

type storeRankingRepository struct {
	conn *postgres.Connection
}

func NewStoreRankingRepository(conn *postgres.Connection) StoreRankingRepository {
	return &storeRankingRepository{
		conn: conn,
	}
}

func (r *storeRankingRepository) GetStoreRanking() (*domain.StoreRankingResponse, error) {
	yesterday := time.Now().AddDate(0, 0, -1)
	month := yesterday.Format("01-2006")

	// Construir a query base
	queryBuilder := squirrel.
		Select(
			"sr.id",
			"sr.account_id",
			"sr.month",
			"sr.store_name",
			"sr.social_network_revenue",
			"sr.position",
			"sr.position_change",
			"sr.previous_position",
			"sr.created_at",
			"sr.updated_at",
		).
		From(storeRankingTable).
		Where(squirrel.Eq{"sr.month": month}).
		OrderBy("sr.position ASC").
		PlaceholderFormat(squirrel.Dollar)

	// Converter para SQL
	sqlQuery, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir a query: %w", err)
	}

	// Executar a query
	rows, err := r.conn.Query(sqlQuery, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return &domain.StoreRankingResponse{
				Ranking:    []domain.StoreRankingItem{},
				LastUpdate: time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("erro ao executar a query: %w", err)
	}
	defer rows.Close()

	// Processar os resultados
	rankings := make([]domain.StoreRankingItem, 0)
	var lastUpdate time.Time

	for rows.Next() {
		item, err := r.scanStoreRankingItem(rows)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear item do ranking: %w", err)
		}

		rankings = append(rankings, *item)

		// Manter o último update mais recente
		if item.UpdatedAt.After(lastUpdate) {
			lastUpdate = item.UpdatedAt
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro durante a iteração de linhas: %w", err)
	}

	// Se não há registros, usar tempo atual para lastUpdate
	if lastUpdate.IsZero() {
		lastUpdate = time.Now()
	}

	return &domain.StoreRankingResponse{
		Ranking:    rankings,
		LastUpdate: lastUpdate,
	}, nil
}

func (r *storeRankingRepository) GetByAccountID(accountID string, month string) (*domain.StoreRankingItem, error) {
	query, args, err := squirrel.
		Select("sr.id, sr.account_id, sr.month, sr.store_name, sr.social_network_revenue, sr.position, sr.position_change, sr.previous_position, sr.created_at, sr.updated_at").
		From(storeRankingTable).
		Where(squirrel.Eq{"sr.account_id": accountID, "sr.month": month}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir a query: %w", err)
	}

	row := r.conn.QueryRow(query, args...)
	ranking, err := r.scanStoreRankingItemRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao escanear ranking: %w", err)
	}
	return ranking, nil
}

func (r *storeRankingRepository) SaveOrUpdateStoreRanking(rankings []*domain.StoreRankingItem) error {
	if len(rankings) == 0 {
		return nil
	}

	// Construir query de inserção em lote
	query := squirrel.StatementBuilder.
		Insert("store_ranking").
		Columns(
			"account_id",
			"month",
			"store_name",
			"social_network_revenue",
			"position",
			"position_change",
			"previous_position",
		).
		PlaceholderFormat(squirrel.Dollar)

	// Adicionar os valores de cada ranking
	for _, ranking := range rankings {
		query = query.Values(
			ranking.AccountID,
			ranking.Month,
			ranking.StoreName,
			ranking.SocialNetworkRevenue,
			ranking.Position,
			ranking.PositionChange,
			ranking.PreviousPosition,
		)
	}

	// Configurar comportamento de conflito (upsert)
	query = query.Suffix(`
		ON CONFLICT (account_id, month) DO UPDATE SET
			store_name = EXCLUDED.store_name,
			social_network_revenue = EXCLUDED.social_network_revenue,
			position = EXCLUDED.position,
			position_change = EXCLUDED.position_change,
			previous_position = EXCLUDED.previous_position,
			updated_at = CURRENT_TIMESTAMP
	`)

	// Converter para SQL e executar
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("erro ao construir query de inserção: %w", err)
	}

	_, err = r.conn.Exec(sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("erro ao executar query de inserção: %w", err)
	}

	return nil
}

func (r *storeRankingRepository) scanStoreRankingItem(rows *sql.Rows) (*domain.StoreRankingItem, error) {
	item := &domain.StoreRankingItem{}

	err := rows.Scan(
		&item.ID,
		&item.AccountID,
		&item.Month,
		&item.StoreName,
		&item.SocialNetworkRevenue,
		&item.Position,
		&item.PositionChange,
		&item.PreviousPosition,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return item, nil
}

func (r *storeRankingRepository) scanStoreRankingItemRow(row *sql.Row) (*domain.StoreRankingItem, error) {
	item := &domain.StoreRankingItem{}

	err := row.Scan(
		&item.ID,
		&item.AccountID,
		&item.Month,
		&item.StoreName,
		&item.SocialNetworkRevenue,
		&item.Position,
		&item.PositionChange,
		&item.PreviousPosition,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return item, nil
}
