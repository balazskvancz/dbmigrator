package dbmigrator

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"

	"github.com/balazskvancz/dbmigrator/database"
	"github.com/balazskvancz/dbmigrator/repositories"
)

const (
	versionProlog string = "#v"
)

var (
	ErrNoFilePath    error = errors.New("missing migrations file path")
	ErrBadVersioning error = errors.New("versions must follow `#vX.X.X` format")
	ErrNothingToRun  error = errors.New("no command to run")
)

type engine struct {
	conf         *Config
	repositories *repositories.Repositories
	db           database.Database
}

type Engine interface {
	Process() error
	SetupDatabase() error
	GetLines() ([]string, error)
	ParseLines(lines []string) ([]Command, error)
}

// NewFromJsonConfig craetes a new instance from the config at the given path.
func NewFromJsonConfig(path string) (Engine, error) {
	config, err := loadJsonConfig(path)
	if err != nil {
		return nil, err
	}

	return New(config)
}

// New creates a new instance based upon the given config.
func New(c *Config) (Engine, error) {
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

	rep := repositories.New(db)

	return &engine{
		conf:         c,
		repositories: rep,
	}, nil
}

func (e *engine) Process() error {
	lines, err := e.GetLines()
	if err != nil {
		return err
	}

	commands, err := e.ParseLines(lines)
	if err != nil {
		return nil
	}

	latest := e.repositories.Migrations.GetLatest()

	var latestVersion string

	if latest != nil {
		latestVersion = latest.Version
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

	if err := runCommands(commands, e.conf.WithTransaction); err != nil {
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
		currentVersion string
		lineStack      = make([]string, 0)
		commandStack   = make([]Command, 0)
	)

	for _, line := range lines {
		if !strings.HasPrefix(line, versionProlog) {
			// If currently read line is not empty,
			// then it is simply pushed to the stack.
			if line != "" {
				lineStack = append(lineStack, line)

				continue
			}

			if len(lineStack) != 0 {
				query := strings.Join(lineStack, " ")

				commandStack = append(commandStack, newCommand(e.db, query, currentVersion))

				lineStack = lineStack[:0]
			}
		}

		spl := strings.Split(line, versionProlog)

		if len(spl) != 2 {
			return nil, ErrBadVersioning
		}

		// If there was a version before this iteration
		// then, the accumulated stack must be
		// put into the map with the version string.
		if currentVersion != "" {
			commandStack = commandStack[:0]
			lineStack = lineStack[:0]
		}

		currentVersion = spl[1]
	}

	return commandStack, nil
}

func filterCommands(version string, commands []Command) []Command {
	filtered := make([]Command, 0)

	for _, c := range commands {
		if c.ShouldRun(version) {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

func runCommands(commands []Command, withTransaction bool) error {
	for _, c := range commands {
		if err := c.Run(); err != nil {
			// The transaction must stop at the first problem.
			if withTransaction {
				return err
			}

			// Othewise, only log the got error, to stdout.
			// TODO:
		}
	}

	return nil
}
