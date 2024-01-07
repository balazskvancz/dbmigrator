package dbmigrator

import (
	"github.com/balazskvancz/dbmigrator/database"
)

type command struct {
	db      database.Database
	query   string
	version Semver
	dir     direction
}

type Command interface {
	Run() error
	ShouldRun(Semver, direction, Semver) bool
	Semver() Semver
	GetDirection() direction
}

func newCommand(db database.Database, query string, semver Semver, dir ...direction) Command {
	// By default, every commands direction is up.
	direction := DirectionUp

	if len(dir) == 1 {
		direction = dir[0]
	}

	return &command{
		db:      db,
		query:   query,
		version: semver,
		dir:     direction,
	}
}

// Run executes the stored query.
func (c *command) Run() error {
	_, err := c.db.Exec(c.query)

	return err
}

// ShouldRun returns whether a certain command should run based upon
// the latest stored version, direction and target version.
func (c *command) ShouldRun(version Semver, dir direction, target Semver) bool {
	// In case of given target version, have to make sure, that
	// the commands version is in the appropriate interval.
	if target != nil {
		if dir == DirectionUp {
			return c.version.GreaterThan(version) && !c.version.GreaterThan(target)
		}

		return !c.version.GreaterThan(version) && c.version.GreaterThan(target)
	}

	// In case of down direction, only those semvers
	// should run which are equal to the current version
	if dir == DirectionDown {
		return c.version.Equals(version)
	}

	return c.version.GreaterThan(version)
}

// Semver returns the command's semver.
func (c *command) Semver() Semver { return c.version }

// GetDirection returns the command' direction.
func (c *command) GetDirection() direction { return c.dir }
