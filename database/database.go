package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	errTxIsNil error = errors.New("no transaction to commit")
)

type Database interface {
	Exec(string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	GetDatabaseName() string
	Connect() error
	Close()

	StartTransaction() error
	Commit() error
	Rollback() error
}

type database struct {
	*sql.DB

	ctx  context.Context
	conf DatabaseConfig
	tx   *sql.Tx
}

const (
	defaultDriverName string = "mysql"
)

type DatabaseConfig struct {
	Driver   string
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

// New returns a new instance of Database based upon the given config.
func New(ctx context.Context, c DatabaseConfig) (Database, error) {
	if c.Driver == "" {
		c.Driver = defaultDriverName
	}

	db := &database{
		ctx:  ctx,
		conf: c,
	}

	if err := db.Connect(); err != nil {
		return nil, err
	}

	return db, nil
}

// Connect tries to connect to the database given by the config.
func (d *database) Connect() error {
	c := d.conf

	source := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", c.Username, c.Password, c.Host, c.Port, c.Database)

	sqlDb, err := sql.Open(d.conf.Driver, source)
	if err != nil {
		return err
	}

	d.DB = sqlDb

	return nil
}

// GetDatabaseName returns the name of the connected database.
func (d *database) GetDatabaseName() string {
	return d.conf.Database
}

// Close closes the database connection.
func (d *database) Close() { d.DB.Close() }

// Exec executes the given command with the associated values.
// It is executed via the opened transaction, if there is any.
func (d *database) Exec(query string, values ...any) (sql.Result, error) {
	if d.tx != nil {
		return d.tx.Exec(query, values...)
	}
	return d.DB.Exec(query, values...)
}

// Query implements query, done via the started opened transaction,
// if there is one.
func (d *database) Query(query string, values ...any) (*sql.Rows, error) {
	if d.tx != nil {
		return d.tx.Query(query, values...)
	}
	return d.DB.Query(query, values...)
}

// QueryRow implements a single row query, done via the started opened transaction,
// if there is one.
func (d *database) QueryRow(query string, values ...any) *sql.Row {
	if d.tx != nil {
		return d.tx.QueryRow(query, values...)
	}
	return d.DB.QueryRow(query, values...)
}

// StartTransaction tries to start a transaction on the given database connection.
func (d *database) StartTransaction() error {
	tx, err := d.DB.BeginTx(d.ctx, nil)
	if err != nil {
		return err
	}

	d.tx = tx

	return nil
}

// Commit tries to close the transaction with a commit message.
func (d *database) Commit() error {
	if d.tx == nil {
		return errTxIsNil
	}
	return d.tx.Commit()
}

// Rollback rolls back all the executed SQL queries in the given transaction.
func (d *database) Rollback() error {
	if d.tx == nil {
		return errTxIsNil
	}
	return d.tx.Rollback()
}
