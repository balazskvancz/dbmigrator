package dbmigrator

import (
	"github.com/balazskvancz/dbmigrator/database"
)

type command struct {
	db      database.Database
	query   string
	version Semver
}

type Command interface {
	Run() error
	ShouldRun(Semver) bool
	Semver() Semver
}

func newCommand(db database.Database, query string, semver Semver) Command {
	return &command{
		db:      db,
		query:   query,
		version: semver,
	}
}

// Run executes the stored query.
func (c *command) Run() error {
	_, err := c.db.Exec(c.query)

	return err
}

// ShouldRun returns whether a certain command should run based upon
// the latest stored versions.
func (c *command) ShouldRun(version Semver) bool {
	return c.version.GreaterThan(version)
}

func (c *command) Semver() Semver {
	return c.version
}
