package dbmigrator

import "github.com/balazskvancz/dbmigrator/database"

type command struct {
	db      database.Database
	query   string
	version string
}

type Command interface {
	Run() error
	ShouldRun(version string) bool
}

func newCommand(db database.Database, query string, version string) Command {
	return &command{
		db:      db,
		query:   query,
		version: version,
	}
}

// Run executes the stored query.
func (c *command) Run() error {
	_, err := c.db.Exec(c.query)

	return err
}

// ShouldRun returns whether a certain command should run based upon
// the latest stored versions.
func (c *command) ShouldRun(version string) bool {
	if version == "" {
		return true
	}

	// TODO: comparing semver.
	return false
}
