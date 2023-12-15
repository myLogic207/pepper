package dbconnect

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	"INITPATH": "db.init.d",
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
		"MAX_LIFETIME":  0,
		"MAX_IDLE_TIME": 0,
	},
	"CACHE": map[string]interface{}{
		"ACTIVE": false,
	},
	"LOGGER": map[string]interface{}{
		"PREFIX":       "DATABASE",
		"PREFIXLENGTH": 20,
		"LEVEL":        "DEBUG",
		"WRITERS": map[string]interface{}{
			"STDOUT": true,
			"FILE": map[string]interface{}{
				"ACTIVE": true,
			},
		},
	},
}

type urlGenerator func(config.Config) (string, error)

var dbTypeLookup = map[string]urlGenerator{
	PostgresDBType: newPostgresConnector,
	MysqlDBType:    newMysqlConnector,
	MssqlDBType:    newMssqlConnector,
}

type DB struct {
	*sql.DB
	conf   config.Config
	logger log.Logger
}

func New(options config.Config) (*DB, error) {
	conf, err := resolveDBConfig(defaultDBConfig, options)
	if err != nil {
		return nil, err
	}

	loggerConf, _ := conf.GetConfig("LOGGER")
	logger, err := log.NewLogger(loggerConf)
	if err != nil {
		return nil, err
	}

	db := &DB{
		logger: logger,
		conf:   conf,
	}

	if err := db.resolveDBConnector(); err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, ErrCouldNotConnect
	}

	return db, nil
}

func (db *DB) Close(ctx context.Context) error {
	if err := db.DB.Close(); err != nil {
		db.logger.Error(ctx, err.Error())
	}
	db.logger.Info(ctx, "Database connection closed")
	if err := db.logger.Shutdown(); err != nil {
		return err
	}
	return nil
}

func NewDBWithConnector(connector *sql.DB, options config.Config) (*DB, error) {
	conf, err := resolveDBConfig(defaultDBConfig, options)
	if err != nil {
		return nil, err
	}

	loggerConf, _ := conf.GetConfig("LOGGER")
	logger, err := log.NewLogger(loggerConf)
	if err != nil {
		return nil, err
	}

	if err := connector.Ping(); err != nil {
		return nil, err
	}

	return &DB{
		DB:     connector,
		logger: logger,
		conf:   conf,
	}, nil
}

func (db *DB) NewBuilder() squirrel.StatementBuilderType {
	builder := squirrel.StatementBuilder
	if dbType, _ := db.conf.GetString("TYPE"); dbType == PostgresDBType {
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
	if dbType, _ := db.conf.GetString("TYPE"); dbType == "postgres" {
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

func resolveDBConfig(defaults map[string]interface{}, options config.Config) (config.Config, error) {
	conf := config.NewWithInitialValues(defaults)
	if err := conf.Merge(options, true); err != nil {
		return nil, err
	}
	if err := conf.HasAllKeys(defaults); err != nil {
		return nil, err
	}
	return conf, nil
}

func (db *DB) resolveDBConnector() error {
	dbType, _ := db.conf.GetString("TYPE")
	urlGen, ok := dbTypeLookup[dbType]
	if !ok {
		return ErrUnknownDBType
	}

	connector, err := urlGen(db.conf)
	if err != nil {
		return err
	}
	var driver string
	if dbType == PostgresDBType {
		driver = "pgx"
	} else {
		driver = dbType
	}
	conn, err := sql.Open(driver, connector)
	if err != nil {
		return ErrCouldNotConnect
	}
	poolConfig, _ := db.conf.GetConfig("POOL")
	db.DB = applyPoolConfig(conn, poolConfig)
	return nil
}

func applyPoolConfig(db *sql.DB, conf config.Config) *sql.DB {
	maxOpenConns, _ := conf.GetInt("CONNS_OPEN")
	db.SetMaxOpenConns(maxOpenConns)
	maxIdleConns, _ := conf.GetInt("CONNS_IDLE")
	db.SetMaxIdleConns(maxIdleConns)
	maxLifetime, _ := conf.GetDuration("MAX_LIFETIME")
	db.SetConnMaxLifetime(maxLifetime)
	maxIdleTime, _ := conf.GetDuration("MAX_IDLE")
	db.SetConnMaxIdleTime(maxIdleTime)
	return db
}

func newPostgresConnector(conf config.Config) (url string, err error) {
	user, _ := conf.GetString("USERNAME")
	password, _ := conf.GetString("PASSWORD")
	host, _ := conf.GetString("HOST")
	port, _ := conf.GetString("PORT")
	dbName, _ := conf.GetString("NAME")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, dbName)
	if sslMode, err := conf.GetString("SSLMODE"); err == nil {
		dsn += fmt.Sprintf("?sslmode=%s", sslMode)
	} else {
		dsn += "?sslmode=disable"
	}
	if timezone, err := conf.GetString("TIMEZONE"); err == nil {
		dsn += fmt.Sprintf("&TimeZone=%s", timezone)
	}
	return dsn, nil
}

func newMysqlConnector(conf config.Config) (url string, err error) {
	user, _ := conf.GetString("USERNAME")
	password, _ := conf.GetString("PASSWORD")
	host, _ := conf.GetString("HOST")
	port, _ := conf.GetString("PORT")
	dbName, _ := conf.GetString("NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbName)

	if charset, _ := conf.GetString("CHARSET"); charset != "" {
		dsn += fmt.Sprintf("?charset=%s", charset)
	}

	return dsn, nil
}

func newMssqlConnector(conf config.Config) (url string, err error) {
	user, _ := conf.GetString("USERNAME")
	password, _ := conf.GetString("PASSWORD")
	host, _ := conf.GetString("HOST")
	port, _ := conf.GetString("PORT")
	dbName, _ := conf.GetString("NAME")

	dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s", user, password, host, port, dbName)

	if charset, _ := conf.GetString("CHARSET"); charset != "" {
		dsn += fmt.Sprintf("&charset=%s", charset)
	}

	return dsn, nil
}
