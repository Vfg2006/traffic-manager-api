package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/database/postgres"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

const (
	accountsTable        = "accounts a"
	businessManagerTable = "business_manager bm"
)

type AccountRepository interface {
	GetAccountByID(accountID string) (*domain.AdAccount, error)
	GetAccountByExternalID(accountExternalID string) (*domain.AdAccount, error)
	ListAccounts(availableStatus []domain.AdAccountStatus) ([]*domain.AdAccount, error)
	ListAccountsMap() (map[string]struct{}, error)
	SaveOrUpdate(account []*domain.AdAccount, businessManagerIDs map[string]string) error
	SaveOrUpdateBusinessManager(bms []*domain.BusinessManager) (map[string]string, error)
	UpdateAccount(account *domain.UpdateAdAccountRequest) error
}

type accountRepository struct {
	conn *postgres.Connection
}

func NewAccountRepository(conn *postgres.Connection) AccountRepository {
	return &accountRepository{
		conn: conn,
	}
}

func (a *accountRepository) GetAccountByExternalID(accountExternalID string) (*domain.AdAccount, error) {
	return a.GetAccount(squirrel.Eq{"a.external_id": accountExternalID})
}

func (a *accountRepository) GetAccountByID(accountID string) (*domain.AdAccount, error) {
	return a.GetAccount(squirrel.Eq{"a.id": accountID})
}

func (a *accountRepository) GetAccount(whereClause map[string]interface{}) (*domain.AdAccount, error) {
	accountsSQL, accountsArgs, err := squirrel.
		Select("a.id, a.external_id, a.name, a.nickname, a.cnpj, a.secret_name, a.status, a.origin, a.business_id").
		From(accountsTable).
		Where(whereClause).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, err
	}

	row := a.conn.QueryRow(accountsSQL, accountsArgs...)

	acc, err := a.deserializeAccount(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return acc, err
}

func (a *accountRepository) deserializeAccount(row *sql.Row) (*domain.AdAccount, error) {
	acc := &domain.AdAccount{}

	if err := row.Scan(
		&acc.ID,
		&acc.ExternalID,
		&acc.Name,
		&acc.Nickname,
		&acc.CNPJ,
		&acc.SecretName,
		&acc.Status,
		&acc.Origin,
		&acc.BusinessManagerID,
	); err != nil {
		return nil, err
	}

	return acc, nil
}

func (a *accountRepository) ListAccounts(availableStatus []domain.AdAccountStatus) ([]*domain.AdAccount, error) {
	queryBuilder := squirrel.
		Select("a.id, a.external_id, a.name, a.nickname, a.cnpj, a.secret_name, a.status, bm.id, bm.name").
		From(accountsTable).
		Join("business_manager bm ON a.business_id = bm.id").
		OrderBy("a.nickname ASC").
		PlaceholderFormat(squirrel.Dollar)

	if len(availableStatus) > 0 {
		queryBuilder = queryBuilder.Where(squirrel.Eq{"a.status": availableStatus})
	}

	accountsSQL, accountsArgs, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := a.conn.Query(accountsSQL, accountsArgs...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}
	defer rows.Close()

	accounts := make([]*domain.AdAccount, 0)

	for rows.Next() {
		acc, err := a.deserializeAccountWithBM(rows)
		if err != nil {
			return nil, err
		}

		if acc == nil {
			continue
		}

		accounts = append(accounts, acc)
	}

	if len(accounts) == 0 {
		return nil, nil
	}

	return accounts, err
}

func (r *accountRepository) SaveOrUpdate(accounts []*domain.AdAccount, businessManagerIDs map[string]string) error {
	if len(accounts) == 0 {
		return nil
	}

	// Cria a query de inserção ou atualização
	query := squirrel.StatementBuilder.
		Insert("accounts").
		Columns("id", "external_id", "cnpj", "secret_name", "name", "nickname", "origin", "business_id", "status").
		PlaceholderFormat(squirrel.Dollar)

	// Adiciona os valores de cada account ao batch
	for _, account := range accounts {
		// Cria a chave composta para buscar o business manager correto
		bmKey := fmt.Sprintf("%s:%s", account.Origin, account.BusinessManagerID)

		// Obtém o ID do business manager usando a chave composta
		businessID, exists := businessManagerIDs[bmKey]
		if !exists {
			logrus.Warnf("Business manager não encontrado para a chave: %s", bmKey)
			continue
		}

		query = query.Values(
			account.ID,
			account.ExternalID,
			account.CNPJ,
			account.SecretName,
			account.Name,
			account.Nickname,
			account.Origin,
			businessID,
			account.Status,
		)
	}

	// Define o comportamento em caso de conflito (atualiza os campos)
	query = query.Suffix(`
			ON CONFLICT (external_id, origin) DO UPDATE SET
				cnpj = EXCLUDED.cnpj,
				secret_name = EXCLUDED.secret_name,
				name = EXCLUDED.name,
				status = EXCLUDED.status,
				nickname = COALESCE(accounts.nickname, EXCLUDED.nickname)
		`)

	// Converte a query para SQL
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	// Executa a query
	_, err = r.conn.Exec(sqlQuery, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			return fmt.Errorf("database error: %w (code: %s)", pqErr, pqErr.Code)
		}
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

func (r *accountRepository) SaveOrUpdateBusinessManager(bms []*domain.BusinessManager) (map[string]string, error) {
	// Inicializa o mapa para armazenar os IDs dos business managers
	businessManagerIDS := make(map[string]string, 0)

	// Primeiro, recupera os business managers existentes
	err := r.getExistingBusinessManagers(businessManagerIDS)
	if err != nil {
		return nil, fmt.Errorf("erro ao recuperar business managers existentes: %w", err)
	}

	// Adiciona os valores de cada business manager ao batch
	for _, bm := range bms {
		// Cria a chave composta para verificar se já existe
		compositeKey := fmt.Sprintf("%s:%s", bm.Origin, bm.ExternalID)

		// Verifica se o business manager já existe no mapa
		if _, exists := businessManagerIDS[compositeKey]; exists {
			logrus.Infof("Business manager já existe: %s", compositeKey)
			continue
		}

		// Cria a query de inserção ou atualização
		query := squirrel.StatementBuilder.
			Insert("business_manager").
			Columns("id", "external_id", "name", "origin").
			PlaceholderFormat(squirrel.Dollar)

		query = query.Values(
			bm.ID,
			bm.ExternalID,
			bm.Name,
			bm.Origin,
		)

		// Define o comportamento em caso de conflito (atualiza os campos)
		query = query.Suffix(`
			ON CONFLICT (external_id, origin) DO UPDATE SET
				name = EXCLUDED.name RETURNING id
		`)

		// Converte a query para SQL
		sqlQuery, args, err := query.ToSql()
		if err != nil {
			return businessManagerIDS, fmt.Errorf("failed to build query: %w", err)
		}

		// Executa a query
		var ID string
		err = r.conn.QueryRow(sqlQuery, args...).Scan(&ID)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok {
				return businessManagerIDS, fmt.Errorf("database error: %w (code: %s)", pqErr, pqErr.Code)
			}
			return businessManagerIDS, fmt.Errorf("failed to execute query: %w", err)
		}

		businessManagerIDS[compositeKey] = ID
	}

	return businessManagerIDS, nil
}

func (a *accountRepository) deserializeAccountWithBM(row *sql.Rows) (*domain.AdAccount, error) {
	acc := domain.AdAccount{}

	if err := row.Scan(
		&acc.ID,
		&acc.ExternalID,
		&acc.Name,
		&acc.Nickname,
		&acc.CNPJ,
		&acc.SecretName,
		&acc.Status,
		&acc.BusinessManagerID,
		&acc.BusinessManagerName,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	return &acc, nil
}

func (a *accountRepository) UpdateAccount(account *domain.UpdateAdAccountRequest) error {
	if account.ID == "" {
		return errors.New("ID is required")
	}

	// Constrói a query de atualização
	queryBuilder := squirrel.
		Update("accounts").
		Where(squirrel.Eq{"id": account.ID}).
		PlaceholderFormat(squirrel.Dollar)

	// Adiciona os campos que foram fornecidos para atualização
	if account.Nickname != nil {
		queryBuilder = queryBuilder.Set("nickname", *account.Nickname)
	}

	if account.CNPJ != nil {
		queryBuilder = queryBuilder.Set("cnpj", *account.CNPJ)
	}

	if account.SecretName != nil {
		queryBuilder = queryBuilder.Set("secret_name", *account.SecretName)
	}

	if account.Status != nil {
		queryBuilder = queryBuilder.Set("status", *account.Status)
	}

	// Converte a query para SQL
	sqlQuery, args, err := queryBuilder.ToSql()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	// Executa a query
	result, err := a.conn.Exec(sqlQuery, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			return fmt.Errorf("database error: %w (code: %s)", pqErr, pqErr.Code)
		}
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// Verifica se algum registro foi afetado
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("account not found")
	}

	return nil
}

func (a *accountRepository) ListAccountsMap() (map[string]struct{}, error) {
	// Query simplificada para buscar apenas os campos essenciais
	accountsSQL, accountsArgs, err := squirrel.
		Select("a.id, a.external_id, a.origin").
		From(accountsTable).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir a query: %w", err)
	}

	rows, err := a.conn.Query(accountsSQL, accountsArgs...)
	if err != nil {
		if err == sql.ErrNoRows {
			return make(map[string]struct{}, 0), nil
		}
		return nil, fmt.Errorf("erro ao executar a query: %w", err)
	}
	defer rows.Close()

	// Inicializa o mapa para armazenar as contas
	accountsMap := make(map[string]struct{})

	// Itera sobre os resultados
	for rows.Next() {
		account := &domain.AdAccount{}
		err := rows.Scan(
			&account.ID,
			&account.ExternalID,
			&account.Origin,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao deserializar a conta: %w", err)
		}

		// Cria uma chave composta com origin e external_id
		compositeKey := fmt.Sprintf("%s:%s", account.Origin, account.ExternalID)

		// Adiciona a conta ao mapa usando a chave composta
		accountsMap[compositeKey] = struct{}{}
	}

	// Verifica se houve erros durante a iteração
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar sobre os resultados: %w", err)
	}

	return accountsMap, nil
}

// GetExistingBusinessManagers recupera os business managers existentes no banco de dados
// e adiciona os IDs no mapa passado como parâmetro (externalID -> id)
func (r *accountRepository) getExistingBusinessManagers(bmIDs map[string]string) error {
	if bmIDs == nil {
		return errors.New("o mapa de business managers não pode ser nulo")
	}

	// Constrói a consulta SQL para buscar todos os business managers
	query, args, err := squirrel.
		Select("id, external_id, origin").
		From("business_manager").
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("erro ao construir a query: %w", err)
	}

	// Executa a consulta
	rows, err := r.conn.Query(query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // Não há business managers, retorna sem erro
		}
		return fmt.Errorf("erro ao consultar business managers: %w", err)
	}
	defer rows.Close()

	// Processa cada linha do resultado
	for rows.Next() {
		var id, externalID, origin string
		if err := rows.Scan(&id, &externalID, &origin); err != nil {
			return fmt.Errorf("erro ao ler business manager: %w", err)
		}

		// Adiciona ao mapa usando a combinação de externalID e origin
		compositeKey := fmt.Sprintf("%s:%s", origin, externalID)
		bmIDs[compositeKey] = id
	}

	// Verifica erros de iteração
	if err = rows.Err(); err != nil {
		return fmt.Errorf("erro durante iteração dos resultados: %w", err)
	}

	return nil
}
