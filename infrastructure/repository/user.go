package repository

import (
	"database/sql"
	"fmt"

	"github.com/Masterminds/squirrel"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/database/postgres"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

const (
	usersTable        = "users"
	userAccountsTable = "user_accounts"
)

type UserRepository interface {
	CreateUser(user *domain.User) (*domain.User, error)
	UpdateUser(user *domain.User) error
	GetUserByEmail(email string) (*domain.User, error)
	GetUserByID(userID int) (*domain.User, error)
	ListUser() ([]*domain.User, error)
	GetUserLinkedAccounts(userID int) ([]string, error)
	LinkUserAccount(userID int, accountID string) error
	UnlinkUserAccount(userID int, accountID string) error
}

type userRepository struct {
	conn *postgres.Connection
}

func NewUserRepository(conn *postgres.Connection) UserRepository {
	return &userRepository{
		conn: conn,
	}
}

func (r *userRepository) CreateUser(user *domain.User) (*domain.User, error) {
	queryBuilder := squirrel.
		Insert(usersTable).
		Columns("name", "lastname", "email", "password_hash", "active", "role_id").
		Values(user.Name, user.Lastname, user.Email, user.PasswordHash, user.Active, user.RoleID).
		Suffix("RETURNING id").
		PlaceholderFormat(squirrel.Dollar)

	usersSQL, usersArgs, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	err = r.conn.QueryRow(usersSQL, usersArgs...).Scan(&user.ID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepository) UpdateUser(user *domain.User) error {
	queryBuilder := squirrel.
		Update(usersTable).
		Set("active", user.Active).
		Where(squirrel.Eq{"id": user.ID})

	if user.Name != "" {
		queryBuilder = queryBuilder.Set("name", user.Name)
	}

	if user.Lastname != "" {
		queryBuilder = queryBuilder.Set("lastname", user.Lastname)
	}

	if user.Email != "" {
		queryBuilder = queryBuilder.Set("email", user.Email)
	}

	if user.PasswordHash != "" {
		queryBuilder = queryBuilder.Set("password_hash", user.PasswordHash)
	}

	if user.RoleID != 0 {
		queryBuilder = queryBuilder.Set("role_id", user.RoleID)
	}

	if user.AvatarURL != nil && *user.AvatarURL != "" {
		queryBuilder = queryBuilder.Set("avatar_url", user.AvatarURL)
	}

	if user.Deleted {
		queryBuilder = queryBuilder.Set("deleted", true)
		queryBuilder = queryBuilder.Set("deleted_at", user.DeletedAt)
	}

	usersSQL, usersArgs, err := queryBuilder.PlaceholderFormat(squirrel.Dollar).ToSql()
	if err != nil {
		return err
	}

	_, err = r.conn.Exec(usersSQL, usersArgs...)
	if err != nil {
		if err == sql.ErrNoRows {
			return err
		}
		return err
	}

	return nil
}

func (r *userRepository) GetUserByEmail(email string) (*domain.User, error) {
	var user domain.User
	err := r.conn.QueryRow("SELECT id, name, lastname, email, password_hash, active, role_id, avatar_url, created_at, updated_at FROM users WHERE email = $1", email).Scan(
		&user.ID,
		&user.Name,
		&user.Lastname,
		&user.Email,
		&user.PasswordHash,
		&user.Active,
		&user.RoleID,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Buscar contas vinculadas
	linkedAccounts, err := r.GetUserLinkedAccounts(user.ID)
	if err != nil {
		logrus.Warnf("Erro ao buscar contas vinculadas para o usuário %d: %v", user.ID, err)
		// Continua mesmo com erro, apenas com a lista vazia
	} else {
		user.LinkedAccounts = linkedAccounts
	}

	return &user, nil
}

func (r *userRepository) GetUserByID(userID int) (*domain.User, error) {
	var user domain.User
	err := r.conn.QueryRow("SELECT id, name, lastname, email, password_hash, active, role_id, avatar_url, created_at, updated_at FROM users WHERE deleted = false AND id = $1", userID).Scan(
		&user.ID,
		&user.Name,
		&user.Lastname,
		&user.Email,
		&user.PasswordHash,
		&user.Active,
		&user.RoleID,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Buscar contas vinculadas
	linkedAccounts, err := r.GetUserLinkedAccounts(user.ID)
	if err != nil {
		logrus.Warnf("Erro ao buscar contas vinculadas para o usuário %d: %v", user.ID, err)
		// Continua mesmo com erro, apenas com a lista vazia
	} else {
		user.LinkedAccounts = linkedAccounts
	}

	return &user, nil
}

func (r *userRepository) ListUser() ([]*domain.User, error) {
	queryBuilder := squirrel.
		Select("id", "name", "lastname", "email", "active", "role_id", "avatar_url", "created_at", "updated_at").
		From(usersTable).
		Where(squirrel.Eq{"deleted": false}).
		OrderBy("name ASC").
		PlaceholderFormat(squirrel.Dollar)

	usersSQL, usersArgs, err := queryBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.conn.Query(usersSQL, usersArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.Lastname,
			&user.Email,
			&user.Active,
			&user.RoleID,
			&user.AvatarURL,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, err
		}

		// Buscar contas vinculadas
		linkedAccounts, err := r.GetUserLinkedAccounts(user.ID)
		if err != nil {
			logrus.Warnf("Erro ao buscar contas vinculadas para o usuário %d: %v", user.ID, err)
			// Continua mesmo com erro, apenas com a lista vazia
		} else {
			user.LinkedAccounts = linkedAccounts
		}

		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *userRepository) GetUserLinkedAccounts(userID int) ([]string, error) {
	query := squirrel.
		Select("account_id").
		From(userAccountsTable).
		Where(squirrel.Eq{"user_id": userID}).
		PlaceholderFormat(squirrel.Dollar)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("erro ao construir consulta: %w", err)
	}

	rows, err := r.conn.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar contas vinculadas: %w", err)
	}
	defer rows.Close()

	var linkedAccounts []string
	for rows.Next() {
		var accountID string
		if err := rows.Scan(&accountID); err != nil {
			return nil, fmt.Errorf("erro ao processar resultado: %w", err)
		}
		linkedAccounts = append(linkedAccounts, accountID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro durante iteração: %w", err)
	}

	return linkedAccounts, nil
}

func (r *userRepository) LinkUserAccount(userID int, accountID string) error {
	query := squirrel.
		Insert(userAccountsTable).
		Columns("user_id", "account_id").
		Values(userID, accountID).
		Suffix("ON CONFLICT (user_id, account_id) DO NOTHING").
		PlaceholderFormat(squirrel.Dollar)

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("erro ao construir consulta: %w", err)
	}

	_, err = r.conn.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("erro ao vincular conta: %w", err)
	}

	return nil
}

func (r *userRepository) UnlinkUserAccount(userID int, accountID string) error {
	query := squirrel.
		Delete(userAccountsTable).
		Where(squirrel.Eq{"user_id": userID, "account_id": accountID}).
		PlaceholderFormat(squirrel.Dollar)

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("erro ao construir consulta: %w", err)
	}

	_, err = r.conn.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("erro ao desvincular conta: %w", err)
	}

	return nil
}
