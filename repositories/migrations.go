package repositories

import (
	"fmt"
	"time"

	"github.com/balazskvancz/dbmigrator/database"
	"github.com/balazskvancz/dbmigrator/models"
)

type MigrationsRepository interface {
	Insert(string) error
	GetLatest() *models.Migration
	DoesExists() bool
	CreateTable() error
}

type migrationsRepository struct {
	tableName string
	db        database.Database
}

func newMigrationsRepository(tableName string, db database.Database) MigrationsRepository {
	return &migrationsRepository{
		tableName: tableName,
		db:        db,
	}
}

// Insert saves the version of the latest migration defined in the input.
func (mr *migrationsRepository) Insert(version string) error {
	_, err := mr.db.Exec(fmt.Sprintf(`
		INSERT INTO %s SET
			version 	= ?
			createdAt = NOW()
	`, mr.tableName), version)

	return err
}

// GetLatest returns the lates migration entity stored in the database.
func (mr *migrationsRepository) GetLatest() *models.Migration {
	row := mr.db.QueryRow(fmt.Sprintf(`
		SELECT
			id,
			version,
			createdAt
		FROM %s
		ORDER BY createdAt DESC	
		LIMIT 1
	`, mr.tableName))
	if row == nil {
		return nil
	}

	var (
		id        int64
		version   string
		createdAt time.Time
	)

	if err := row.Scan(&id, &version, &createdAt); err != nil {
		return nil
	}

	return &models.Migration{
		Id:        id,
		Version:   version,
		CreatedAt: createdAt,
	}
}

// DoesExists returns if the migrations table exists in the current database.
func (mr *migrationsRepository) DoesExists() bool {
	row := mr.db.QueryRow(`
		SELECT
			TABLE_SCHEMA
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = ?
		AND TABLE_NAME = ?
	`, mr.db.GetDatabaseName(), mr.tableName)

	var name string

	return row.Scan(&name) != nil
}

// CreateTable creates the migrations table.
func (mr *migrationsRepository) CreateTable() error {
	_, err := mr.db.Exec(fmt.Sprintf(`
		CREATE TABLE %s (
			id 				INTEGER 			AUTO_INCREMENT,
			version 	VARCHAR (10)	NOT NULL,
			createdAt	DATETIME			NOT NULL,

			PRIMARY KEY (id)
		)
	`, mr.tableName))

	return err
}
