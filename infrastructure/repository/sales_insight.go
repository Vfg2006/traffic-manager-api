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
	salesInsightsTable = "sales_insights si"
)

type SalesInsightRepository interface {
	GetByAccountIDAndDate(accountID string, date time.Time) (*domain.SalesInsightEntry, error)
	SaveOrUpdate(insight *domain.SalesInsightEntry) error
	DeleteOlderThan(days int) (int64, error)
	GetByDateRange(accountID string, startDate, endDate time.Time) ([]*domain.SalesInsightEntry, error)
}

type salesInsightRepository struct {
	conn *postgres.Connection
}

func NewSalesInsightRepository(conn *postgres.Connection) SalesInsightRepository {
	return &salesInsightRepository{
		conn: conn,
	}
}

func (r *salesInsightRepository) GetByAccountIDAndDate(accountID string, date time.Time) (*domain.SalesInsightEntry, error) {
	query, args, err := squirrel.
		Select("si.id, si.account_id, si.date, si.sales_metrics, si.created_at, si.updated_at").
		From(salesInsightsTable).
		Where(squirrel.Eq{"si.account_id": accountID, "si.date": date.Format(time.DateOnly)}).
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

func (r *salesInsightRepository) GetByDateRange(accountID string, startDate, endDate time.Time) ([]*domain.SalesInsightEntry, error) {
	query, args, err := squirrel.
		Select("si.id, si.account_id, si.date, si.sales_metrics, si.created_at, si.updated_at").
		From(salesInsightsTable).
		Where(squirrel.Eq{"si.account_id": accountID}).
		Where(squirrel.GtOrEq{"si.date": startDate.Format(time.DateOnly)}).
		Where(squirrel.LtOrEq{"si.date": endDate.Format(time.DateOnly)}).
		OrderBy("si.date ASC").
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

	insights := make([]*domain.SalesInsightEntry, 0)
	for rows.Next() {
		insight, err := r.scanInsightRows(rows)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear sales insights: %w", err)
		}
		insights = append(insights, insight)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro durante a iteração de linhas: %w", err)
	}

	return insights, nil
}

func (r *salesInsightRepository) SaveOrUpdate(insight *domain.SalesInsightEntry) error {
	var salesMetricsJSON []byte
	var err error

	if insight.SalesMetrics != nil {
		salesMetricsJSON, err = json.Marshal(insight.SalesMetrics)
		if err != nil {
			return fmt.Errorf("erro ao serializar SalesMetrics para JSON: %w", err)
		}
	}

	query := squirrel.StatementBuilder.
		Insert("sales_insights").
		Columns("account_id", "date", "sales_metrics").
		Values(
			insight.AccountID,
			insight.Date.Format(time.DateOnly),
			salesMetricsJSON,
		).
		Suffix(`
			ON CONFLICT (account_id, date) DO UPDATE SET
				sales_metrics = EXCLUDED.sales_metrics,
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

func (r *salesInsightRepository) DeleteOlderThan(days int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -days).Format(time.DateOnly)

	query, args, err := squirrel.
		Delete("sales_insights").
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

func (r *salesInsightRepository) scanInsight(row *sql.Row) (*domain.SalesInsightEntry, error) {
	insight := &domain.SalesInsightEntry{}
	var salesMetricsJSON []byte
	var dateStr string

	err := row.Scan(
		&insight.ID,
		&insight.AccountID,
		&dateStr,
		&salesMetricsJSON,
		&insight.CreatedAt,
		&insight.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	date, err := time.Parse(time.DateOnly, dateStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao converter data: %w", err)
	}
	insight.Date = date

	if salesMetricsJSON != nil {
		salesMetrics := make(map[string]*domain.SalesMetrics)
		if err := json.Unmarshal(salesMetricsJSON, &salesMetrics); err != nil {
			return nil, fmt.Errorf("erro ao deserializar JSON de sales_metrics: %w", err)
		}
		insight.SalesMetrics = salesMetrics
	}

	return insight, nil
}

func (r *salesInsightRepository) scanInsightRows(rows *sql.Rows) (*domain.SalesInsightEntry, error) {
	insight := &domain.SalesInsightEntry{}
	var salesMetricsJSON []byte
	// var dateStr string

	err := rows.Scan(
		&insight.ID,
		&insight.AccountID,
		&insight.Date,
		&salesMetricsJSON,
		&insight.CreatedAt,
		&insight.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// date, err := time.Parse(time.DateOnly, dateStr)
	// if err != nil {
	// 	return nil, fmt.Errorf("erro ao converter data: %w", err)
	// }
	// insight.Date = dateStr

	if salesMetricsJSON != nil {
		salesMetrics := make(map[string]*domain.SalesMetrics)
		if err := json.Unmarshal(salesMetricsJSON, &salesMetrics); err != nil {
			return nil, fmt.Errorf("erro ao deserializar JSON de sales_metrics: %w", err)
		}
		insight.SalesMetrics = salesMetrics
	}

	return insight, nil
}
