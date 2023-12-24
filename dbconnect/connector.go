package dbconnect

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/myLogic207/gotils/config"
	log "github.com/myLogic207/gotils/logger"
)

type DBContextKey string

const (
	PostgresDBType = "postgres"
	MysqlDBType    = "mysql"
	MssqlDBType    = "mssql"
)

var (
	ErrCouldNotConnect = errors.New("could not connect to database")
	ErrUnknownDBType   = errors.New("unknown database type")
	ErrUnknownTable    = errors.New("unknown table")
)

var defaultDBConfig = map[string]interface{}{
	"TYPE":     "postgres",
	"HOST":     "localhost",
	"PORT":     "5432",
	"USERNAME": "postgres",
	"PASSWORD": "postgres",
	"NAME":     "postgres",
	// "SSLMODE":  "disable",
	"TIMEZONE": "Europe/Berlin",
	"POOL": map[string]interface{}{
		"CONNS_OPEN":    10,
		"CONNS_IDLE":    5,
		"MAX_LIFETIME":  -1,
		"MAX_IDLE_TIME": -1,
	},
	"LOGGER": map[string]interface{}{
		"PREFIX": "DATABASE",
	},
}

type urlGenerator func(context.Context, *config.Config) (string, error)

var dbTypeLookup = map[string]urlGenerator{
	PostgresDBType: newPostgresConnector,
	MysqlDBType:    newMysqlConnector,
	MssqlDBType:    newMssqlConnector,
}

type DB struct {
	*sql.DB
	conf   *config.Config
	logger log.Logger
}

func NewWithConnector(ctx context.Context, connector *sql.DB, logger log.Logger) (*DB, error) {
	if err := connector.Ping(); err != nil {
		return nil, err
	}

	return &DB{
		DB:     connector,
		logger: logger,
	}, nil
}

func (db *DB) Connect(ctx context.Context, options *config.Config) error {
	if db.conf == nil {
		conf, err := config.WithInitialValuesAndOptions(ctx, defaultDBConfig, options)
		if err != nil {
			return fmt.Errorf("could not initialize config: %w", err)
		}
		db.conf = conf
	}

	if db.logger == nil {
		loggerConf, _ := db.conf.GetConfig(ctx, "LOGGER")
		logger, err := log.Init(ctx, loggerConf)
		if err != nil {
			return fmt.Errorf("could not initializing logger: %w", err)
		}
		db.logger = logger
	}

	conn, err := resolveDBConnector(ctx, db.conf)
	if err != nil {
		return fmt.Errorf("could not resolve database connector: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return ErrCouldNotConnect
	}

	db.DB = conn
	return nil
}

func (db *DB) Close(ctx context.Context) error {
	if err := db.DB.Close(); err != nil {
		db.logger.Error(ctx, err.Error())
	}
	db.logger.Info(ctx, "Database connection closed")
	if err := db.logger.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

func NewBuilder(ctx context.Context, dbType string) squirrel.StatementBuilderType {
	builder := squirrel.StatementBuilder

	switch dbType {
	case PostgresDBType:
		builder = builder.PlaceholderFormat(squirrel.Dollar)
	case MysqlDBType:
		builder = builder.PlaceholderFormat(squirrel.Question)
	case MssqlDBType:
		builder = builder.PlaceholderFormat(squirrel.Dollar)
	}

	return builder
}

type TransactionFunc func(tx *sql.Tx) error

func (db *DB) Transaction(ctx context.Context, transaction TransactionFunc, options *sql.TxOptions) error {
	tx, err := db.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	db.logger.Debug(ctx, "transaction started")

	if err := transaction(tx); err != nil {
		db.logger.Error(ctx, err.Error())
		if err := tx.Rollback(); err != nil {
			return err
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	db.logger.Debug(ctx, "transaction committed")

	return nil
}

func (db *DB) CheckTableExists(ctx context.Context, table string) (bool, error) {
	builder := squirrel.StatementBuilder
	if dbType, _ := db.conf.Get(ctx, "TYPE"); dbType == "postgres" {
		builder = builder.PlaceholderFormat(squirrel.Dollar)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	var name string
	err = builder.
		Select("table_name").
		From("information_schema.tables").
		Where(squirrel.Eq{"table_name": table}).
		RunWith(tx).QueryRow().Scan(&name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		db.logger.Error(ctx, err.Error())
		return false, err
	} else if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return name == table, nil
}

func resolveDBConnector(ctx context.Context, conf *config.Config) (*sql.DB, error) {
	dbType, _ := conf.Get(ctx, "TYPE")
	urlGen, ok := dbTypeLookup[dbType]
	if !ok {
		return nil, ErrUnknownDBType
	}

	connector, err := urlGen(ctx, conf)
	if err != nil {
		return nil, err
	}
	var driver string
	if dbType == PostgresDBType {
		driver = "pgx"
	} else {
		driver = dbType
	}
	conn, err := sql.Open(driver, connector)
	if err != nil {
		return nil, ErrCouldNotConnect
	}
	poolConfig, _ := conf.GetConfig(ctx, "POOL")
	return applyPoolConfig(ctx, conn, poolConfig), nil
}

func applyPoolConfig(ctx context.Context, db *sql.DB, conf *config.Config) *sql.DB {
	maxOpenConnsRaw, _ := conf.Get(ctx, "CONNS_OPEN")
	maxOpenConns, err := strconv.Atoi(maxOpenConnsRaw)
	if err != nil {
		maxOpenConns = 1
		conf.Set(ctx, "CONNS_OPEN", "1", true)
	}
	db.SetMaxOpenConns(maxOpenConns)

	maxIdleConnsRaw, _ := conf.Get(ctx, "CONNS_IDLE")
	maxIdleConns, err := strconv.Atoi(maxIdleConnsRaw)
	if err != nil {
		maxIdleConns = 1
		conf.Set(ctx, "CONNS_IDLE", "1", true)
	}
	db.SetMaxIdleConns(maxIdleConns)

	maxLifetimeRaw, _ := conf.Get(ctx, "MAX_LIFETIME")
	maxLifetime, err := time.ParseDuration(maxLifetimeRaw)
	if err != nil {
		maxLifetime = -1
		conf.Set(ctx, "MAX_LIFETIME", "-1", true)
	}
	db.SetConnMaxLifetime(maxLifetime)

	maxIdleTimeRaw, _ := conf.Get(ctx, "MAX_IDLE")
	maxIdleTime, err := time.ParseDuration(maxIdleTimeRaw)
	if err != nil {
		maxIdleTime = -1
		conf.Set(ctx, "MAX_IDLE", "-1", true)
	}
	db.SetConnMaxIdleTime(maxIdleTime)
	return db
}

func newPostgresConnector(ctx context.Context, conf *config.Config) (url string, err error) {
	user, _ := conf.Get(ctx, "USERNAME")
	password, _ := conf.Get(ctx, "PASSWORD")
	host, _ := conf.Get(ctx, "HOST")
	port, _ := conf.Get(ctx, "PORT")
	dbName, _ := conf.Get(ctx, "NAME")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, dbName)
	if sslMode, err := conf.Get(ctx, "SSLMODE"); err == nil {
		dsn += fmt.Sprintf("?sslmode=%s", sslMode)
	} else {
		dsn += "?sslmode=disable"
	}
	if timezone, err := conf.Get(ctx, "TIMEZONE"); err == nil {
		dsn += fmt.Sprintf("&TimeZone=%s", timezone)
	}
	return dsn, nil
}

func newMysqlConnector(ctx context.Context, conf *config.Config) (url string, err error) {
	user, _ := conf.Get(ctx, "USERNAME")
	password, _ := conf.Get(ctx, "PASSWORD")
	host, _ := conf.Get(ctx, "HOST")
	port, _ := conf.Get(ctx, "PORT")
	dbName, _ := conf.Get(ctx, "NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbName)

	if charset, _ := conf.Get(ctx, "CHARSET"); charset != "" {
		dsn += fmt.Sprintf("?charset=%s", charset)
	}

	return dsn, nil
}

func newMssqlConnector(ctx context.Context, conf *config.Config) (url string, err error) {
	user, _ := conf.Get(ctx, "USERNAME")
	password, _ := conf.Get(ctx, "PASSWORD")
	host, _ := conf.Get(ctx, "HOST")
	port, _ := conf.Get(ctx, "PORT")
	dbName, _ := conf.Get(ctx, "NAME")

	dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s", user, password, host, port, dbName)

	if charset, _ := conf.Get(ctx, "CHARSET"); charset != "" {
		dsn += fmt.Sprintf("&charset=%s", charset)
	}

	return dsn, nil
}
