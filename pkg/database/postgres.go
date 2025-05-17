package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConfig represents configuration for a PostgreSQL database connection
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	MaxConns int
	Timeout  time.Duration
}

// NewPostgresConfig creates a new PostgreSQL database configuration
func NewPostgresConfig(host string, port int, user, password, database, sslMode string) *PostgresConfig {
	return &PostgresConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
		SSLMode:  sslMode,
		MaxConns: 10,
		Timeout:  10 * time.Second,
	}
}

// ConnectionString returns the database connection string
func (c *PostgresConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode)
}

// PostgresDB handles interactions with a PostgreSQL database
type PostgresDB struct {
	pool *pgxpool.Pool
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(config *PostgresConfig) (*PostgresDB, error) {
	connString := config.ConnectionString()
	
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %v", err)
	}
	
	// Set max connection pool size
	poolConfig.MaxConns = int32(config.MaxConns)
	
	// Create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %v", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}
	
	return &PostgresDB{pool: pool}, nil
}

// Close closes the database connection pool
func (db *PostgresDB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Pool returns the connection pool
func (db *PostgresDB) Pool() *pgxpool.Pool {
	return db.pool
}

// Ping tests the database connection
func (db *PostgresDB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// ExecContext executes an SQL query with no rows returned
func (db *PostgresDB) ExecContext(ctx context.Context, sql string, args ...interface{}) (pgx.CommandTag, error) {
	return db.pool.Exec(ctx, sql, args...)
}

// QueryContext executes an SQL query and returns the rows
func (db *PostgresDB) QueryContext(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

// QueryRowContext executes an SQL query and returns a single row
func (db *PostgresDB) QueryRowContext(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

// BeginTx starts a transaction
func (db *PostgresDB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return db.pool.Begin(ctx)
} 