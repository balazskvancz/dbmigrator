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

const (
	versionProlog         string = "#v"
	singleLineComment     string = "--"
	multiLineCommentStart string = "/*"
	multiLineCommentEnd   string = "*/"
)

var (
	ErrBadVersioning      error = errors.New("versions must follow `#vX.X.X` format")
	ErrConfigIsNil        error = errors.New("given config is <nil>")
	ErrInvalidLastVersion error = errors.New("invalid latest stored version")
	ErrNoFilePath         error = errors.New("missing migrations file path")
	ErrNothingToRun       error = errors.New("no command to run")
)

type Logger interface {
	Info(string)
	Error(string)
}

type engine struct {
	logger Logger

	conf         *Config
	repositories *repositories.Repositories
	db           database.Database
}

type EngineOptFunc func(*engine)

func WithLogger(l Logger) EngineOptFunc {
	return func(e *engine) {
		e.logger = l
	}
}

type Engine interface {
	Process() error
	SetupDatabase() error
	GetLines() ([]string, error)
	ParseLines(lines []string) ([]Command, error)
	CloseDatabase()
}

var (
	_ Engine = (*engine)(nil)
	_ Logger = (*engine)(nil)
)

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
	}

	for _, o := range opts {
		o(e)
	}

	return e, nil
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

	latest := e.repositories.Migrations.GetLatest()

	var latestVersion Semver

	if latest != nil {
		latestSemver := newSemver(latest.Version)

		// In this case the stored latest version is somehow invalid.
		if latestSemver == nil {
			return ErrInvalidLastVersion
		}

		latestVersion = latestSemver
	}

	if latestVersion == nil {
		e.Info("-- no prestored migration history --")
	} else {
		e.Info(fmt.Sprintf("-- prestored migration version: %s", latestVersion.ToString()))
	}

	filteredCommands := filterCommands(latestVersion, commands)

	if len(filteredCommands) == 0 {
		return ErrNothingToRun
	}

	if e.conf.WithTransaction {
		if err := e.db.StartTransaction(); err != nil {
			return err
		}
	}

	if err := runCommands(e, commands, e.conf.WithTransaction); err != nil {
		if e.conf.WithTransaction {
			if err := e.db.Rollback(); err != nil {
				return err
			}
		}

		return err
	}

	// Then must save the latest version.
	newLatestVersion := getLatestVersion(commands)

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
	)

	for _, line := range lines {
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

			if strings.HasSuffix(strings.TrimSpace(line), ";") {
				if currentVersion != nil {
					query := strings.Join(lineStack, " ")

					commandStack = append(commandStack, newCommand(e.db, query, currentVersion))
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

func filterCommands(version Semver, commands []Command) []Command {
	filtered := make([]Command, 0)

	for _, c := range commands {
		if version == nil || c.ShouldRun(version) {
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
	var sv Semver
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
