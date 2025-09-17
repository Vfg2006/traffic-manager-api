package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"github.com/vfg2006/traffic-manager-api/infrastructure/database/postgres"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

const (
	adInsightsTable = "ad_insights ai"
)

type AdInsightRepository interface {
	GetByAccountIDAndDate(accountID string, date time.Time) (*domain.AdInsightEntry, error)
	GetByExternalIDAndDate(externalID string, date time.Time) (*domain.AdInsightEntry, error)
	SaveOrUpdate(insight *domain.AdInsightEntry) error
	DeleteOlderThan(days int) (int64, error)
	GetByDateRange(accountID string, startDate, endDate time.Time) ([]*domain.AdInsightEntry, error)
}

type adInsightRepository struct {
	conn *postgres.Connection
}

func NewAdInsightRepository(conn *postgres.Connection) AdInsightRepository {
	return &adInsightRepository{
		conn: conn,
	}
}

func (r *adInsightRepository) GetByAccountIDAndDate(accountID string, date time.Time) (*domain.AdInsightEntry, error) {
	query, args, err := squirrel.
		Select("ai.id, ai.account_id, ai.external_id, ai.date, ai.ad_metrics, ai.created_at, ai.updated_at").
		From(adInsightsTable).
		Where(squirrel.Eq{"ai.account_id": accountID, "ai.date": date.Format("2006-01-02")}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir a query: %w", err)
	}

	row := r.conn.QueryRow(query, args...)
	insight, err := r.scanInsight(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao escanear insight: %w", err)
	}

	return insight, nil
}

func (r *adInsightRepository) GetByExternalIDAndDate(externalID string, date time.Time) (*domain.AdInsightEntry, error) {
	query, args, err := squirrel.
		Select("ai.id, ai.account_id, ai.external_id, ai.date, ai.ad_metrics, ai.created_at, ai.updated_at").
		From(adInsightsTable).
		Where(squirrel.Eq{"ai.external_id": externalID, "ai.date": date.Format("2006-01-02")}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir a query: %w", err)
	}

	row := r.conn.QueryRow(query, args...)
	insight, err := r.scanInsight(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao escanear insight: %w", err)
	}

	return insight, nil
}

func (r *adInsightRepository) GetByDateRange(accountID string, startDate, endDate time.Time) ([]*domain.AdInsightEntry, error) {
	query, args, err := squirrel.
		Select("ai.id, ai.account_id, ai.external_id, ai.date, ai.ad_metrics, ai.created_at, ai.updated_at").
		From(adInsightsTable).
		Where(squirrel.Eq{"ai.account_id": accountID}).
		Where(squirrel.GtOrEq{"ai.date": startDate.Format("2006-01-02")}).
		Where(squirrel.LtOrEq{"ai.date": endDate.Format("2006-01-02")}).
		OrderBy("ai.date ASC").
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir a query: %w", err)
	}

	rows, err := r.conn.Query(query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao executar a query: %w", err)
	}
	defer rows.Close()

	insights := make([]*domain.AdInsightEntry, 0)
	for rows.Next() {
		insight, err := r.scanInsightRows(rows)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear ad insights: %w", err)
		}
		insights = append(insights, insight)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro durante a iteração de linhas: %w", err)
	}

	return insights, nil
}

func (r *adInsightRepository) SaveOrUpdate(insight *domain.AdInsightEntry) error {
	var adMetricsJSON []byte
	var err error

	if insight.AdMetrics != nil {
		adMetricsJSON, err = json.Marshal(insight.AdMetrics)
		if err != nil {
			return fmt.Errorf("erro ao serializar AdMetrics para JSON: %w", err)
		}
	}

	query := squirrel.StatementBuilder.
		Insert("ad_insights").
		Columns("account_id", "external_id", "date", "ad_metrics").
		Values(
			insight.AccountID,
			insight.ExternalID,
			insight.Date.Format("2006-01-02"),
			adMetricsJSON,
		).
		Suffix(`
			ON CONFLICT (account_id, date) DO UPDATE SET
				external_id = EXCLUDED.external_id,
				ad_metrics = EXCLUDED.ad_metrics,
				updated_at = NOW()
		`).
		PlaceholderFormat(squirrel.Dollar)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("erro ao construir a query: %w", err)
	}

	_, err = r.conn.Exec(sqlQuery, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			return fmt.Errorf("erro no banco de dados: %w (código: %s)", pqErr, pqErr.Code)
		}
		return fmt.Errorf("erro ao executar a query: %w", err)
	}

	return nil
}

func (r *adInsightRepository) DeleteOlderThan(days int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	query, args, err := squirrel.
		Delete("ad_insights").
		Where(squirrel.Lt{"date": cutoffDate}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("erro ao construir a query: %w", err)
	}

	result, err := r.conn.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("erro ao executar a query: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("erro ao obter número de linhas afetadas: %w", err)
	}

	return rowsAffected, nil
}

func (r *adInsightRepository) scanInsight(row *sql.Row) (*domain.AdInsightEntry, error) {
	insight := &domain.AdInsightEntry{}
	var adMetricsJSON []byte
	var dateStr string

	err := row.Scan(
		&insight.ID,
		&insight.AccountID,
		&insight.ExternalID,
		&dateStr,
		&adMetricsJSON,
		&insight.CreatedAt,
		&insight.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao converter data: %w", err)
	}
	insight.Date = date

	if adMetricsJSON != nil {
		adMetrics := &domain.AdAccountMetrics{}
		if err := json.Unmarshal(adMetricsJSON, adMetrics); err != nil {
			return nil, fmt.Errorf("erro ao deserializar JSON de ad_metrics: %w", err)
		}
		insight.AdMetrics = adMetrics
	}

	return insight, nil
}

func (r *adInsightRepository) scanInsightRows(rows *sql.Rows) (*domain.AdInsightEntry, error) {
	insight := &domain.AdInsightEntry{}
	var adMetricsJSON []byte

	err := rows.Scan(
		&insight.ID,
		&insight.AccountID,
		&insight.ExternalID,
		&insight.Date,
		&adMetricsJSON,
		&insight.CreatedAt,
		&insight.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if adMetricsJSON != nil {
		adMetrics := &domain.AdAccountMetrics{}
		if err := json.Unmarshal(adMetricsJSON, adMetrics); err != nil {
			return nil, fmt.Errorf("erro ao deserializar JSON de ad_metrics: %w", err)
		}
		insight.AdMetrics = adMetrics
	}

	return insight, nil
}
