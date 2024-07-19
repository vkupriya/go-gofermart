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
			accrual BIGINT NOT NULL,
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
		`CREATE TABLE IF NOT EXISTS withdrawals(
			userid VARCHAR NOT NULL,
			number VARCHAR UNIQUE NOT NULL,
			sum BIGINT NOT NULL,
			processed_at timestamp
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

func (p *PostgresDB) UserAdd(c *models.Config, u models.User) error {
	db := p.pool
	var PgErr *pgconn.PgError
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "INSERT INTO users (userid, password, accrual) VALUES($1, $2, $3)"

	_, err := db.Exec(ctx, querySQL, u.UserID, u.Password, u.Accrual)
	if err != nil {
		if errors.As(err, &PgErr) && PgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("user already exists: %w", err)
		}
		return fmt.Errorf("failed to insert user %s into Postgres DB: %w", u.UserID, err)
	}
	return nil
}

func (p *PostgresDB) UserGet(c *models.Config, userid string) (models.User, error) {
	db := p.pool
	var user models.User
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	row := db.QueryRow(ctx, "SELECT * FROM users WHERE userid=$1", userid)
	err := row.Scan(&user.UserID, &user.Password, &user.Accrual)
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

func (p *PostgresDB) OrderGet(c *models.Config, oid string) (models.Order, error) {
	db := p.pool
	var order models.Order
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "SELECT * FROM orders WHERE number=$1"

	row := db.QueryRow(ctx, querySQL, oid)

	err := row.Scan(&order.UserID, &order.Number, &order.Status, &order.Accrual, &order.Uploaded)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return order, nil
		}
		return order, fmt.Errorf("failed to query order in DB: %w", err)
	}
	return order, nil
}

func (p *PostgresDB) OrdersGet(c *models.Config, userid string) (models.Orders, error) {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "SELECT * FROM orders WHERE userid=$1 ORDER BY uploaded_at ASC"

	rows, err := db.Query(ctx, querySQL, userid)
	if err != nil {
		return models.Orders{}, fmt.Errorf("failed to query DB: %w", err)
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Order])
	if err != nil {
		return nil, fmt.Errorf("failed to scan orders: %w", err)
	}
	return orders, nil
}

func (p *PostgresDB) BalanceGet(c *models.Config, userid string) (models.Balance, error) {
	db := p.pool

	balance := models.Balance{}
	var accrual float32
	var sum float32

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "SELECT (accrual) FROM users WHERE userid=$1"

	row := db.QueryRow(ctx, querySQL, userid)

	err := row.Scan(&accrual)
	if err != nil {
		return balance, fmt.Errorf("failed to query user table in DB: %w", err)
	}
	balance.Current = accrual

	querySQL = "SELECT SUM(sum) FROM withdrawals WHERE userid=$1"

	row = db.QueryRow(ctx, querySQL, userid)

	err = row.Scan(&sum)
	if err != nil {
		return balance, fmt.Errorf("failed to query withdrawals table in DB: %w", err)
	}

	balance.Withdrawn = sum

	return balance, nil
}

func (p *PostgresDB) GetAllNewOrders(c *models.Config) (models.Orders, error) {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "UPDATE orders SET status='PROCESSING' WHERE status='NEW' RETURNING *"

	rows, err := db.Query(ctx, querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query DB: %w", err)
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Order])
	if err != nil {
		return nil, fmt.Errorf("failed to scan orders: %w", err)
	}
	return orders, nil
}

func (p *PostgresDB) UpdateOrder(c *models.Config, order models.Order) error {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "UPDATE orders SET status=$1, accrual=$2 WHERE number=$3"

	_, err := db.Exec(ctx, querySQL, order.Status, order.Accrual, order.Number)
	if err != nil {
		return fmt.Errorf("failed to update order %s in Postgres DB: %w", order.Number, err)
	}

	return nil
}

func (p *PostgresDB) UserAddAccrual(c *models.Config, order models.Order) error {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	querySQL := "UPDATE users SET accrual = accrual + $1 WHERE userid=$2"

	_, err := db.Exec(ctx, querySQL, order.Accrual, order.UserID)
	if err != nil {
		return fmt.Errorf("failed to add accrual for user %s in Postgres DB: %w", order.UserID, err)
	}
	return nil
}

func (p *PostgresDB) AccrualWithdraw(c *models.Config, w models.Withdrawal) error {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	querySQL := "UPDATE users SET accrual = accrual - $1 WHERE userid=$2"

	_, err = tx.Exec(ctx, querySQL, w.Sum, w.UserID)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf("failed to rollback transaction: %w", err)
		}
		return fmt.Errorf("failed to withdraw accrual for user %s in Postgres DB: %w", w.UserID, err)
	}
	t := time.Now().Format(time.RFC3339)

	querySQL = "INSERT INTO withdrawals (userid, number, sum, processed_at) VALUES($1, $2, $3, $4)"

	_, err = tx.Exec(ctx, querySQL, w.UserID, w.Number, w.Sum, t)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf("failed to rollback transaction: %w", err)
		}
		return fmt.Errorf("failed to withdraw accrual for user %s in Postgres DB: %w", w.UserID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit accrual withdrawal transaction for user %s", w.UserID)
	}
	return nil
}

func (p *PostgresDB) WithdrawalsGet(c *models.Config, uid string) (models.Withdrawals, error) {
	db := p.pool
	var w models.Withdrawals
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ContextTimeout)*time.Second)
	defer cancel()

	query := "SELECT * FROM withdrawals WHERE userid=$1 ORDER BY processed_at ASC"

	rows, err := db.Query(ctx, query, uid)
	if err != nil {
		return w, fmt.Errorf("failed to query DB: %w", err)
	}
	defer rows.Close()

	w, err = pgx.CollectRows(rows, pgx.RowToStructByName[models.Withdrawal])
	if err != nil {
		return w, fmt.Errorf("failed to scan withdrawals: %w", err)
	}
	return w, nil
}

func (p *PostgresDB) Close() {
	p.pool.Close()
}
