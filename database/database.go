package database

import (
	"database/sql"
	"fmt"
)

type Database interface {
	Exec(string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	GetDatabaseName() string
	Connect() error
}

type database struct {
	conf DatabaseConfig
	*sql.DB
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
func New(c DatabaseConfig) (Database, error) {
	if c.Driver == "" {
		c.Driver = defaultDriverName
	}

	db := &database{conf: c}

	if err := db.Connect(); err != nil {
		return nil, err
	}

	return db, nil
}

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

func (d *database) GetDatabaseName() string {
	return d.conf.Database
}
