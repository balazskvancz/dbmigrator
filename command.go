package dbmigrator

import (
	"github.com/balazskvancz/dbmigrator/database"
)

type command struct {
	db      database.Database
	query   string
	version *semver
}

type Command interface {
	Run() error
	ShouldRun(*semver) bool
	Semver() *semver
}

func newCommand(db database.Database, query string, semver *semver) Command {
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
func (c *command) ShouldRun(version *semver) bool {
	return c.version.greaterThan(version)
}

func (c *command) Semver() *semver {
	return c.version
}
