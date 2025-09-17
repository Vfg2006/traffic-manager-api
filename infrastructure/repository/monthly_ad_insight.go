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
	monthlyAdInsightsTable = "monthly_ad_insights mai"
)

type MonthlyAdInsightRepository interface {
	GetByAccountIDAndPeriod(accountID string, date time.Time) (*domain.MonthlyAdInsightEntry, error)
	GetByExternalIDAndPeriod(externalID string, date time.Time) (*domain.MonthlyAdInsightEntry, error)
	SaveOrUpdate(insight *domain.MonthlyAdInsightEntry) error
	DeleteOlderThan(months int) (int64, error)
	GetByPeriodRange(accountID string, startDate, endDate time.Time) ([]*domain.MonthlyAdInsightEntry, error)
	GetAllPeriods() ([]string, error)
}

type monthlyAdInsightRepository struct {
	conn *postgres.Connection
}

func NewMonthlyAdInsightRepository(conn *postgres.Connection) MonthlyAdInsightRepository {
	return &monthlyAdInsightRepository{
		conn: conn,
	}
}

func (r *monthlyAdInsightRepository) GetByAccountIDAndPeriod(accountID string, date time.Time) (*domain.MonthlyAdInsightEntry, error) {
	// Formatar a data no formato mm-yyyy
	period := fmt.Sprintf("%02d-%04d", int(date.Month()), date.Year())

	query, args, err := squirrel.
		Select("mai.id, mai.account_id, mai.external_id, mai.period, mai.ad_metrics, mai.created_at, mai.updated_at").
		From(monthlyAdInsightsTable).
		Where(squirrel.Eq{"mai.account_id": accountID, "mai.period": period}).
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
		return nil, fmt.Errorf("erro ao escanear insight mensal: %w", err)
	}

	return insight, nil
}

func (r *monthlyAdInsightRepository) GetByExternalIDAndPeriod(externalID string, date time.Time) (*domain.MonthlyAdInsightEntry, error) {
	// Formatar a data no formato mm-yyyy
	period := fmt.Sprintf("%02d-%04d", int(date.Month()), date.Year())

	query, args, err := squirrel.
		Select("mai.id, mai.account_id, mai.external_id, mai.period, mai.ad_metrics, mai.created_at, mai.updated_at").
		From(monthlyAdInsightsTable).
		Where(squirrel.Eq{"mai.external_id": externalID, "mai.period": period}).
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
		return nil, fmt.Errorf("erro ao escanear insight mensal: %w", err)
	}

	return insight, nil
}

func (r *monthlyAdInsightRepository) GetByPeriodRange(accountID string, startDate, endDate time.Time) ([]*domain.MonthlyAdInsightEntry, error) {
	// Criar uma lista de períodos entre as datas de início e fim
	periods := []string{}

	// Começar do mês inicial e ir adicionando meses até chegar ao mês final
	current := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, time.UTC)

	for !current.After(end) {
		period := fmt.Sprintf("%02d-%04d", int(current.Month()), current.Year())
		periods = append(periods, period)

		// Avançar para o próximo mês
		current = current.AddDate(0, 1, 0)
	}

	// Construir a consulta para buscar todos os períodos de uma vez
	query := squirrel.
		Select("mai.id, mai.account_id, mai.external_id, mai.period, mai.ad_metrics, mai.created_at, mai.updated_at").
		From(monthlyAdInsightsTable).
		Where(squirrel.Eq{"mai.account_id": accountID}).
		Where(squirrel.Eq{"mai.period": periods}).
		OrderBy("mai.period ASC").
		PlaceholderFormat(squirrel.Dollar)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir a query: %w", err)
	}

	rows, err := r.conn.Query(sqlQuery, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao executar a query: %w", err)
	}
	defer rows.Close()

	insights := make([]*domain.MonthlyAdInsightEntry, 0)
	for rows.Next() {
		insight, err := r.scanInsightRows(rows)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear monthly ad insights: %w", err)
		}
		insights = append(insights, insight)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro durante a iteração de linhas: %w", err)
	}

	return insights, nil
}

func (r *monthlyAdInsightRepository) SaveOrUpdate(insight *domain.MonthlyAdInsightEntry) error {
	var adMetricsJSON []byte
	var err error

	if insight.AdMetrics != nil {
		adMetricsJSON, err = json.Marshal(insight.AdMetrics)
		if err != nil {
			return fmt.Errorf("erro ao serializar AdMetrics para JSON: %w", err)
		}
	}

	query := squirrel.StatementBuilder.
		Insert("monthly_ad_insights").
		Columns("account_id", "external_id", "period", "ad_metrics").
		Values(
			insight.AccountID,
			insight.ExternalID,
			insight.Period,
			adMetricsJSON,
		).
		Suffix(`
			ON CONFLICT (account_id, period) DO UPDATE SET
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

func (r *monthlyAdInsightRepository) DeleteOlderThan(months int) (int64, error) {
	// Calcular a data de corte
	cutoffTime := time.Now().AddDate(0, -months, 0)
	cutoffPeriod := fmt.Sprintf("%02d-%04d", int(cutoffTime.Month()), cutoffTime.Year())

	query := squirrel.Delete("monthly_ad_insights").
		Where(squirrel.Lt{"period": cutoffPeriod}).
		PlaceholderFormat(squirrel.Dollar)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, fmt.Errorf("erro ao construir a query: %w", err)
	}

	result, err := r.conn.Exec(sqlQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("erro ao executar a query: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("erro ao obter número de linhas afetadas: %w", err)
	}

	return rowsAffected, nil
}

func (r *monthlyAdInsightRepository) scanInsight(row *sql.Row) (*domain.MonthlyAdInsightEntry, error) {
	insight := &domain.MonthlyAdInsightEntry{}
	var adMetricsJSON []byte

	err := row.Scan(
		&insight.ID,
		&insight.AccountID,
		&insight.ExternalID,
		&insight.Period,
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
		insight.AdMetrics.AccountID = insight.ExternalID // fallback when account_id is empty
	}

	return insight, nil
}

func (r *monthlyAdInsightRepository) scanInsightRows(rows *sql.Rows) (*domain.MonthlyAdInsightEntry, error) {
	insight := &domain.MonthlyAdInsightEntry{}
	var adMetricsJSON []byte

	err := rows.Scan(
		&insight.ID,
		&insight.AccountID,
		&insight.ExternalID,
		&insight.Period,
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

// GetAllPeriods retorna todos os períodos disponíveis no formato mm-yyyy
func (r *monthlyAdInsightRepository) GetAllPeriods() ([]string, error) {
	query, args, err := squirrel.
		Select("DISTINCT period").
		From("monthly_ad_insights").
		OrderBy("period ASC").
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir a query: %w", err)
	}

	rows, err := r.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao executar a query: %w", err)
	}
	defer rows.Close()

	periods := make([]string, 0)
	for rows.Next() {
		var period string
		if err := rows.Scan(&period); err != nil {
			return nil, fmt.Errorf("erro ao escanear período: %w", err)
		}
		periods = append(periods, period)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro durante a iteração de linhas: %w", err)
	}

	return periods, nil
}
