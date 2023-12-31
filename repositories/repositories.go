package repositories

import "github.com/balazskvancz/dbmigrator/database"

const (
	defaultMigrationsTableName string = "__migrations__"
)

type Repositories struct {
	Migrations MigrationsRepository
}

// New creates an instace of the common repository holder.
func New(db database.Database, migrationsTableName string) *Repositories {
	finalTableName := defaultMigrationsTableName
	if migrationsTableName != "" {
		finalTableName = migrationsTableName
	}

	return &Repositories{
		Migrations: newMigrationsRepository(finalTableName, db),
	}
}
