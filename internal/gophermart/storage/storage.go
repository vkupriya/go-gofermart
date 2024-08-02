package storage

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vkupriya/go-gophermart/internal/gophermart/models"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

const (
	errRollback string = "failed to rollback transaction: %w"
)

func NewPostgresDB(dsn string) (*PostgresDB, error) {
	if err := runMigrations(dsn); err != nil {
		return nil, fmt.Errorf("failed to run DB migrations: %w", err)
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the DSN: %w", err)
	}

	ctx := context.Background()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize a connection pool: %w", err)
	}

	return &PostgresDB{
		pool: pool,
	}, nil
}

//go:embed migrations/*.sql
var migrationsDir embed.FS

func runMigrations(dsn string) error {
	d, err := iofs.New(migrationsDir, "migrations")
	if err != nil {
		return fmt.Errorf("failed to return an iofs driver: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, dsn)
	if err != nil {
		return fmt.Errorf("failed to get a new migrate instance: %w", err)
	}
	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to apply migrations to the DB: %w", err)
		}
	}
	return nil
}

func (p *PostgresDB) UserAdd(c *models.Config, u models.User) error {
	db := p.pool
	var pgErr *pgconn.PgError
	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
	defer cancel()

	querySQL := "INSERT INTO users (userid, password, accrual) VALUES($1, $2, $3)"

	_, err := db.Exec(ctx, querySQL, u.UserID, u.Password, u.Accrual)
	if err != nil {
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("user already exists: %w", err)
		}
		return fmt.Errorf("failed to insert user %s into Postgres DB: %w", u.UserID, err)
	}
	return nil
}

func (p *PostgresDB) UserGet(c *models.Config, userid string) (models.User, error) {
	db := p.pool
	var user models.User
	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
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
	var pgErr *pgconn.PgError
	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
	defer cancel()

	t := time.Now().Format(time.RFC3339)
	querySQL := "INSERT INTO orders (userid, number, status, accrual, uploaded_at) VALUES($1, $2, $3, $4, $5)"

	_, err := db.Exec(ctx, querySQL, userid, oid, "NEW", 0, t)
	if err != nil {
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("order already exists: %w", err)
		}
		return fmt.Errorf("failed to insert order %s into Postgres DB: %w", userid, err)
	}

	return nil
}

func (p *PostgresDB) OrderGet(c *models.Config, oid string) (models.Order, error) {
	db := p.pool
	var order models.Order
	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
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

	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
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

	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
	defer cancel()

	tx, err := db.Begin(ctx)
	if err != nil {
		return balance, fmt.Errorf("failed to start transaction: %w", err)
	}

	querySQL := "SELECT (accrual) FROM users WHERE userid=$1"

	row := tx.QueryRow(ctx, querySQL, userid)

	err = row.Scan(&accrual)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return balance, fmt.Errorf(errRollback, err)
		}
		return balance, fmt.Errorf("failed to query user table in DB: %w", err)
	}
	balance.Current = accrual

	querySQL = "SELECT COALESCE(SUM(sum), 0) FROM withdrawals WHERE userid=$1"

	row = tx.QueryRow(ctx, querySQL, userid)

	err = row.Scan(&sum)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return balance, fmt.Errorf(errRollback, err)
		}
		return balance, fmt.Errorf("failed to query withdrawals table in DB: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return balance, fmt.Errorf("failed to commit transaction for user %w", err)
	}

	balance.Withdrawn = sum

	return balance, nil
}

func (p *PostgresDB) GetUnprocessedOrders(c *models.Config) (models.Orders, error) {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
	defer cancel()

	querySQL := "UPDATE orders SET status='PROCESSING' WHERE (status='NEW' OR status='PROCESSING') RETURNING *"

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

func (p *PostgresDB) UpdateOrder(c *models.Config, order *models.Order) error {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
	defer cancel()

	querySQL := "UPDATE orders SET status=$1, accrual=$2 WHERE number=$3"

	_, err := db.Exec(ctx, querySQL, order.Status, order.Accrual, order.Number)
	if err != nil {
		return fmt.Errorf("failed to update order %s in Postgres DB: %w", order.Number, err)
	}

	return nil
}

func (p *PostgresDB) UserAddAccrual(c *models.Config, order *models.Order) error {
	db := p.pool

	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
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

	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
	defer cancel()

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	querySQL := "UPDATE users SET accrual = accrual - $1 WHERE userid=$2"

	_, err = tx.Exec(ctx, querySQL, w.Sum, w.UserID)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf(errRollback, err)
		}
		return fmt.Errorf("failed to withdraw accrual for user %s in Postgres DB: %w", w.UserID, err)
	}
	t := time.Now().Format(time.RFC3339)

	querySQL = "INSERT INTO withdrawals (userid, number, sum, processed_at) VALUES($1, $2, $3, $4)"

	_, err = tx.Exec(ctx, querySQL, w.UserID, w.Number, w.Sum, t)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf(errRollback, err)
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
	ctx, cancel := context.WithTimeout(context.Background(), c.ContextTimeout)
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
