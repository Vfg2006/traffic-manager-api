package postgres

import (
	"context"
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/vfg2006/traffic-manager-api/internal/config"
)

type Conn interface {
	Queryer
	Begin(context.Context) (*sql.Tx, error)
	Close() error
	Ping(context.Context) error
	RunInTransaction(context.Context, func(*sql.Tx) error) error
}

type Connection struct {
	*sql.DB
}

func NewConnection(
	ctx context.Context,
	cfg config.Database,
) (*Connection, error) {
	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return &Connection{DB: db}, nil
}

func (c *Connection) Ping(ctx context.Context) error {
	return c.DB.PingContext(ctx)
}

// RunInTransaction run a query in the transaction
func (c *Connection) RunInTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err := recover(); err != nil {
			_ = tx.Rollback()
			panic(err)
		}
	}()

	if err := fn(tx); err != nil {
		if err := tx.Rollback(); err != nil {
			return err
		}
		return err
	}

	return tx.Commit()
}
