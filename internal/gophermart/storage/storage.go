package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
	"go.uber.org/zap"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

func NewPostgresDB(c *models.Config) (*PostgresDB, error) {
	logger := c.Logger
	poolCfg, err := pgxpool.ParseConfig(c.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the DSN: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)

	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize a connection pool: %w", err)
	}

	tx, err := pool.Begin(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to start a transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			if !errors.Is(err, pgx.ErrTxClosed) {
				logger.Sugar().Errorf("failed to rollback the transaction", zap.Error(err))
			}
		}
	}()

	createSchema := []string{
		`CREATE TABLE IF NOT EXISTS users(
			userid VARCHAR UNIQUE NOT NULL,
			password VARCHAR NOT NULL,
			PRIMARY KEY (userid)
		)`,
		`CREATE TABLE IF NOT EXISTS orders(
			userid VARCHAR NOT NULL,
			number VARCHAR UNIQUE NOT NULL,
			status VARCHAR NOT NULL,
			accrual BIGINT NOT NULL,
			uploaded_at timestamp,
			PRIMARY KEY (number)
		)`,
	}

	for _, table := range createSchema {
		if _, err := tx.Exec(ctx, table); err != nil {
			return nil, fmt.Errorf("failed to execute statement `%s`: %w", table, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit PostgresDB transaction: %w", err)
	}

	return &PostgresDB{
		pool: pool,
	}, nil
}

func (p *PostgresDB) UserAdd(c *models.Config, userid string, passwd string) error {
	db := p.pool
	var PgErr *pgconn.PgError
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "INSERT INTO users (userid, password) VALUES($1, $2)"

	_, err := db.Exec(ctx, querySQL, userid, passwd)
	if err != nil {
		if errors.As(err, &PgErr) && PgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("user already exists: %w", err)
		}
		return fmt.Errorf("failed to insert user %s into Postgres DB: %w", userid, err)
	}
	return nil
}

func (p *PostgresDB) UserGet(c *models.Config, userid string) (models.User, error) {
	db := p.pool
	var user models.User
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	row := db.QueryRow(ctx, "SELECT * FROM users WHERE userid=$1", userid)
	err := row.Scan(&user.UserID, &user.Password)
	if err != nil {
		return models.User{}, fmt.Errorf("failed to query user in DB: %w", err)
	}

	return user, nil
}

func (p *PostgresDB) OrderAdd(c *models.Config, userid string, oid string) error {
	db := p.pool
	var PgErr *pgconn.PgError
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	t := time.Now().Format(time.RFC3339)
	querySQL := "INSERT INTO orders (userid, number, status, accrual, uploaded_at) VALUES($1, $2, $3, $4, $5)"

	_, err := db.Exec(ctx, querySQL, userid, oid, "NEW", 0, t)
	if err != nil {
		if errors.As(err, &PgErr) && PgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("order already exists: %w", err)
		}
		return fmt.Errorf("failed to insert order %s into Postgres DB: %w", userid, err)
	}

	return nil
}

func (p *PostgresDB) OrdersGet(c *models.Config, userid string) (models.Orders, error) {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "SELECT * FROM orders WHERE userid=$1 ORDER BY uploaded_at ASC"

	rows, err := db.Query(ctx, querySQL, userid)
	if err != nil {
		return nil, fmt.Errorf("failed to query user in DB: %w", err)
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Order])
	if err != nil {
		return nil, fmt.Errorf("failed to scan orders: %w", err)
	}
	return orders, nil
}
