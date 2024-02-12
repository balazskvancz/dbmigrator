package dbmigrator

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/balazskvancz/dbmigrator/database"
	"github.com/balazskvancz/dbmigrator/repositories"
)

type direction = string

const (
	versionProlog         string = "#v"
	singleLineComment     string = "--"
	multiLineCommentStart string = "/*"
	multiLineCommentEnd   string = "*/"

	upCommand   string = "#[UP]"
	downCommand string = "#[DOWN]"

	DirectionUp   direction = "up"
	DirectionDown direction = "down"
)

var (
	ErrBadVersioning      error = errors.New("versions must follow `#vX.X.X` format")
	ErrConfigIsNil        error = errors.New("given config is <nil>")
	ErrInvalidLastVersion error = errors.New("invalid latest stored version")
	ErrNoFilePath         error = errors.New("missing migrations file path")
	ErrNothingToRun       error = errors.New("no command to run")
)

// Basic semver, which holds the minimum version.
var bottomVersion Semver = newSemver("0.0.0")

type Logger interface {
	Info(string)
	Error(string)
}

type engine struct {
	logger Logger

	conf          *Config
	repositories  *repositories.Repositories
	db            database.Database
	dir           direction
	targetVersion Semver
}

type EngineOptFunc func(*engine)

type Engine interface {
	SetupDatabase() error
	GetLines() ([]string, error)
	ParseLines([]string) ([]Command, error)
	CloseDatabase()
	Process() error
	ProcessWithDirection(direction) error
	ProcessWithTargetVersion(string) error
}

var (
	_ Engine = (*engine)(nil)
	_ Logger = (*engine)(nil)
)

// WithLogger attaches the given logger entity to the engine instance.
func WithLogger(l Logger) EngineOptFunc {
	return func(e *engine) {
		e.logger = l
	}
}

// WithDirection sets the given direction to the engine instance.
func WithDirection(d direction) EngineOptFunc {
	return func(e *engine) {
		e.dir = d
	}
}

// WithTargetVersion sets the given target version to the engine instance.
func WithTargetVersion(v string) EngineOptFunc {
	return func(e *engine) {
		if sv := newSemver(v); sv != nil {
			e.targetVersion = sv
		}
	}
}

// NewFromJsonConfig creates a new instance from the config at the given path.
func NewFromJsonConfig(path string, opts ...EngineOptFunc) (Engine, error) {
	config, err := loadJsonConfig(path)
	if err != nil {
		return nil, err
	}

	return New(config, opts...)
}

// NewFromEnv creates a new instance based on enviromental variables.
func NewFromEnv(opts ...EngineOptFunc) (Engine, error) {
	config, err := loadFromEnv()
	if err != nil {
		return nil, err
	}

	return New(config, opts...)
}

// New creates a new instance based upon the given config.
func New(c *Config, opts ...EngineOptFunc) (Engine, error) {
	if c == nil {
		return nil, ErrConfigIsNil
	}

	db, err := database.New(context.Background(), database.DatabaseConfig{
		Driver:   c.DriverName,
		Host:     c.Host,
		Port:     c.Port,
		Database: c.Database,
		Username: c.Username,
		Password: c.Password,
	})
	if err != nil {
		return nil, err
	}

	rep := repositories.New(db, c.MigrationsTableName)

	e := &engine{
		conf:         c,
		repositories: rep,
		db:           db,
		dir:          DirectionUp,
	}

	for _, o := range opts {
		o(e)
	}

	return e, nil
}

// ProcessWithDirection is a wrapper to Process. Firstly, it sets
// the direction, secondly calls Process.
func (e *engine) ProcessWithDirection(d direction) error {
	e.dir = d

	// Resetting the direction back to default.
	defer func() {
		e.dir = DirectionUp
	}()

	return e.Process()
}

// ProcessWithTargetVersion is a wrapper to Process. Firstly, it sets
// the desired version, secondly calls Process.
func (e *engine) ProcessWithTargetVersion(v string) error {
	sv := newSemver(v)
	if sv == nil {
		return ErrBadVersioning
	}

	e.targetVersion = sv

	// Making sure to reset the version in case
	// of the engine instance reuse.
	defer func() {
		e.targetVersion = nil
	}()

	return e.Process()
}

// Process acts a bootstrapper and the main worker. It sets up
// the appropiate database table – if it does not exist – reads the
// migration file, then parses it, then executes the commands that need to run.
func (e *engine) Process() error {
	if err := e.SetupDatabase(); err != nil {
		return err
	}

	lines, err := e.GetLines()
	if err != nil {
		return err
	}

	commands, err := e.ParseLines(lines)
	if err != nil {
		return err
	}

	current := e.repositories.Migrations.GetLatest()

	var currentVersion Semver

	if current != nil {
		latestSemver := newSemver(current.Version)

		// In this case the stored latest version is somehow invalid.
		if latestSemver == nil {
			return ErrInvalidLastVersion
		}

		currentVersion = latestSemver
	}

	if currentVersion == nil {
		e.Info("-- no prestored migration history --")

		currentVersion = bottomVersion
	} else {
		e.Info(fmt.Sprintf("-- prestored migration version: %s", currentVersion.ToString()))
	}

	// If the given target version is smaller than the latest version,
	// we manually have to set the direction.
	if e.targetVersion != nil && currentVersion.GreaterThan(e.targetVersion) {
		e.dir = DirectionDown
	}

	filteredCommands := filterCommands(currentVersion, commands, e.dir, e.targetVersion)

	if len(filteredCommands) == 0 {
		return ErrNothingToRun
	}

	if e.conf.WithTransaction {
		if err := e.db.StartTransaction(); err != nil {
			return err
		}
	}

	if err := runCommands(e, filteredCommands, e.conf.WithTransaction); err != nil {
		if e.conf.WithTransaction {
			if err := e.db.Rollback(); err != nil {
				return err
			}
		}

		return err
	}

	// Then must save the latest version.
	newLatestVersion := func() Semver {
		if e.targetVersion != nil {
			return e.targetVersion
		}

		if e.dir == DirectionUp {
			return getLatestVersion(commands)
		}

		// Else we would have to scan for previous version
		// compared to the stored one.
		return getPreviousSemver(currentVersion, commands)
	}()

	if err := e.repositories.Migrations.Insert(newLatestVersion.ToString()); err != nil {
		// If there was an error during the insertion of
		// the new latest version, then should a rollback.
		// However, it is only possible, if the a transaction was started.
		if e.conf.WithTransaction {
			if rollbackErr := e.db.Rollback(); rollbackErr != nil {
				return rollbackErr
			}
		}

		return err
	}

	if e.conf.WithTransaction {
		if err := e.db.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// SetupDatabase tries to setup the database states.
// Checks, if the migrations table exists, and tries to
// create if not.
func (e *engine) SetupDatabase() error {
	// If the migrations table exists, then we are all set.
	if e.repositories.Migrations.DoesExists() {
		return nil
	}
	// Otherwise, must create the table.
	return e.repositories.Migrations.CreateTable()
}

// GetLines returns all the nonempty lines read from the path
// set at the config.
func (e *engine) GetLines() ([]string, error) {
	if e.conf.MigrationsFilePath == "" {
		return nil, ErrNoFilePath
	}

	f, err := os.Open(e.conf.MigrationsFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		scanner = bufio.NewScanner(f)
		lines   = make([]string, 0)
	)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, nil
}

// ParseLines creates the version-commands map based upon the reead file.
// The input represents the read lines splitted by newline.
func (e *engine) ParseLines(lines []string) ([]Command, error) {
	if lines == nil {
		return nil, nil
	}

	var (
		currentVersion Semver
		lineStack      = make([]string, 0)
		commandStack   = make([]Command, 0)

		isInsideMultiLineComment = false

		dir direction = DirectionUp
	)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == upCommand {
			dir = DirectionUp

			continue
		}

		if line == downCommand {
			dir = DirectionDown

			continue
		}

		if !strings.HasPrefix(line, versionProlog) {
			if idx := strings.Index(line, singleLineComment); idx != -1 {
				line = line[:idx]
			}

			if idx := strings.Index(line, multiLineCommentStart); idx != -1 {
				isInsideMultiLineComment = true
			}

			if idx := strings.Index(line, multiLineCommentEnd); idx != -1 {
				isInsideMultiLineComment = false
				continue
			}

			if isInsideMultiLineComment {
				continue
			}

			// If currently read line is not empty,
			// then it is simply pushed to the stack.
			if line == "" {
				continue
			}

			lineStack = append(lineStack, line)

			if strings.HasSuffix(line, ";") {
				if currentVersion != nil {
					query := strings.Join(lineStack, " ")

					commandStack = append(commandStack, newCommand(e.db, query, currentVersion, dir))
				}

				lineStack = lineStack[:0]

			}

			continue
		}

		spl := strings.Split(line, versionProlog)

		if len(spl) != 2 {
			return nil, ErrBadVersioning
		}

		// If there was a version before this iteration
		// then, the accumulated stack must be
		// put into the map with the version string.
		if currentVersion != nil {
			lineStack = lineStack[:0]
		}

		sv := newSemver(spl[1])
		if sv == nil {
			return nil, ErrBadVersioning
		}
		currentVersion = sv

		// Setting the direction back to default, whenever a new version is read.
		dir = DirectionUp
	}

	return commandStack, nil
}

// Info implements the info branch of logging.
func (e *engine) Info(line string) {
	if e.logger != nil {
		e.Info(line)
	}
}

// Error implements the error branch of logging.
func (e *engine) Error(line string) {
	if e.logger != nil {
		e.Error(line)
	}
}

// CloseDatabase closes the database connection.
func (e *engine) CloseDatabase() { e.db.Close() }

func filterCommands(
	version Semver,
	commands []Command,
	dir direction,
	targetVersion Semver,
) []Command {
	filtered := make([]Command, 0)

	for _, c := range commands {
		if c.GetDirection() != dir {
			continue
		}

		if version == nil || c.ShouldRun(version, dir, targetVersion) {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

func runCommands(logger Logger, commands []Command, withTransaction bool) error {
	for _, c := range commands {
		if err := c.Run(); err != nil {
			// The transaction must stop at the first problem.
			if withTransaction {
				return err
			}

			logger.Error(fmt.Sprintf("execution error: %v", err))
		}
	}

	return nil
}

func getLatestVersion(commands []Command) Semver {
	var sv Semver = bottomVersion

	for _, c := range commands {
		if sv == nil {
			sv = c.Semver()

			continue
		}

		if c.Semver().GreaterThan(sv) {
			sv = c.Semver()
		}
	}

	return sv
}

func getPreviousSemver(current Semver, commands []Command) Semver {
	var sv Semver = bottomVersion

	for _, c := range commands {
		commandSv := c.Semver()

		if current.WouldRollback(commandSv) || commandSv.Equals(current) {
			continue
		}

		if commandSv.GreaterThan(sv) {
			sv = commandSv
		}
	}

	return sv
}
