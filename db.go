package fxdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	config "github.com/templatedop/fxconfig"
	logger "github.com/templatedop/fxlogger"
	"go.uber.org/fx"
)

type DB struct {
	*pgxpool.Pool
}

// DBConfig holds the configuration for the database connection.
type DBConfig struct {
	DBUsername        string
	DBPassword        string
	DBHost            string
	DBPort            string
	DBDatabase        string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	AppName           string
}

func NewDBConfig(c config.Econfig) *DBConfig {
	fmt.Println("Reached inside NewDBConfig")
	return &DBConfig{
		DBUsername:        c.DBUsername,
		DBPassword:        c.DBPassword,
		DBHost:            c.DBHost,
		DBPort:            c.DBPort,
		DBDatabase:        c.DBDatabase,
		MaxConns:          int32(c.MaxConns),
		MinConns:          int32(c.MinConns),
		MaxConnLifetime:   time.Duration(c.MaxConnLifetime) * time.Minute,
		MaxConnIdleTime:   time.Duration(c.MaxConnIdleTime) * time.Minute,
		HealthCheckPeriod: time.Duration(c.HealthCheckPeriod) * time.Minute,
		AppName:           c.AppName,
	}
}

func Pgxconfig(cfg *DBConfig) (*pgxpool.Config, error) {

	dsn := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s search_path=%s sslmode=disable",
		cfg.DBUsername,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBDatabase,
		"public",
	)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	config.MaxConns = cfg.MaxConns                   // Maximum number of connections in the pool.
	config.MinConns = cfg.MinConns                   // Minimum number of connections to keep in the pool.
	config.MaxConnLifetime = cfg.MaxConnLifetime     // Maximum lifetime of a connection.
	config.MaxConnIdleTime = cfg.MaxConnIdleTime     // Maximum idle time of a connection in the pool.
	config.HealthCheckPeriod = cfg.HealthCheckPeriod // Period between connection health checks.
	config.ConnConfig.ConnectTimeout = 10 * time.Second
	config.ConnConfig.RuntimeParams = map[string]string{
		"application_name": cfg.AppName,
		"search_path":      "public",
	}

	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	config.ConnConfig.StatementCacheCapacity = 100
	config.ConnConfig.DescriptionCacheCapacity = 0
	fmt.Println("Before config")
	fmt.Println("Config is: ", config)
	return config, nil
}

type DBInterface interface {
	Close()
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error, levels ...pgx.TxIsoLevel) error
	ReadTx(ctx context.Context, fn func(tx pgx.Tx) error) error
	// You might have other methods that you want to expose through the interface
}

var _ DBInterface = (*DB)(nil)

// NewDB creates a new PostgreSQL database instance
func NewDB(cfg *DBConfig, log *logger.Logger, pcfg *pgxpool.Config, lc fx.Lifecycle) (*DB, error) {
	log.Debug("NewDB Started")

	ctx := context.Background()
	
	config, err := Pgxconfig(cfg)
	if err != nil {
		return nil, err
	}

	/*
		pgLogTrace := &tracelog.TraceLog{
			Logger:   zerologadapter.NewLogger(log),
			LogLevel: tracelog.LogLevelTrace,
		}
		pgConfig.ConnConfig.Tracer = pgxtrace.CompositeQueryTracer{
			pgLogTrace,
		}
		l.Info().Msg("Connecting to Postgres database")
		pgClient, err := pgxpool.NewWithConfig(ctx, pgConfig)*/

	/*tracer*/
	logger := &testLogger{log: log}
	tracer := &tracelog.TraceLog{

		Logger:   logger,
		LogLevel: tracelog.LogLevelTrace,
	}

	config.ConnConfig.Tracer = tracer
	/*tracer*/

	db, err := pgxpool.NewWithConfig(ctx, config)

	if err != nil {
		return nil, err
	}

	/*IBM prometheus*/
	// collector := pgxpoolprometheus.NewCollector(db, map[string]string{"db_name": "postgres"})
	// prometheus.MustRegister(collector)
	/*IBM prometheus*/
	return &DB{
		db,
	}, nil
}

// Close closes the database connection
func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx) error, levels ...pgx.TxIsoLevel) error {
	var level pgx.TxIsoLevel
	if len(levels) > 0 {
		level = levels[0]
	} else {
		level = pgx.ReadCommitted // Default value
	}
	return db.inTx(ctx, level, "", fn)
}

func (db *DB) ReadTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return db.inTx(ctx, pgx.ReadCommitted, pgx.ReadOnly, fn)

}

func (db *DB) inTx(ctx context.Context, level pgx.TxIsoLevel, access pgx.TxAccessMode,
	fn func(tx pgx.Tx) error) (err error) {

	conn, errAcq := db.Pool.Acquire(ctx)
	if errAcq != nil {
		return fmt.Errorf("acquiring connection: %w", errAcq)
	}
	defer conn.Release()

	opts := pgx.TxOptions{
		IsoLevel:   level,
		AccessMode: access,
	}

	tx, errBegin := conn.BeginTx(ctx, opts)
	if errBegin != nil {
		return fmt.Errorf("begin tx: %w", errBegin)
	}

	defer func() {
		errRollback := tx.Rollback(ctx)
		if !(errRollback == nil || errors.Is(errRollback, pgx.ErrTxClosed)) {
			err = errRollback
		}
	}()

	if err := fn(tx); err != nil {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			return fmt.Errorf("rollback tx: %v (original: %w)", errRollback, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
