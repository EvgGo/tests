package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"testing/internal/config"
	"time"
)

type Client interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

func NewPool(conf *config.DatabaseConfig) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dsn, err := DSN(conf)
	if err != nil {
		return nil, err
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}

	if conf.ConnectConf.MaxConns > 0 {
		poolCfg.MaxConns = conf.ConnectConf.MaxConns
	}

	if conf.ConnectConf.MinConns > 0 {
		poolCfg.MinConns = conf.ConnectConf.MinConns
	}

	if conf.ConnectConf.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = conf.ConnectConf.MaxConnLifetime
	}

	if conf.ConnectConf.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = conf.ConnectConf.MaxConnIdleTime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}

	// Проверяем соединение
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	return pool, nil
}

// DSN возвращает строку вида:
// postgres://user:pass@host:5432/dbname?sslmode=disable
func DSN(c *config.DatabaseConfig) (string, error) {
	if c.Host == "" {
		return "", fmt.Errorf("postgres host is empty")
	}
	if c.Port <= 0 {
		return "", fmt.Errorf("postgres port is invalid")
	}
	if c.User == "" {
		return "", fmt.Errorf("postgres user is empty")
	}
	if c.DBName == "" {
		return "", fmt.Errorf("postgres DBName is empty")
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   "/" + c.DBName,
	}

	q := url.Values{}
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()

	return u.String(), nil
}
