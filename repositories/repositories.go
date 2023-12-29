package repositories

import "github.com/balazskvancz/dbmigrator/database"

const (
	defaultMigrationsTableName string = "__migrations__"
)

type Repositories struct {
	Migrations MigrationsRepository
}

// New creates an instace of the common repository holder.
func New(db database.Database) *Repositories {
	return &Repositories{
		Migrations: newMigrationsRepository(defaultMigrationsTableName, db),
	}
}
