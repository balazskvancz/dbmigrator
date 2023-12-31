package dbmigrator

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/balazskvancz/dbmigrator/repositories"
)

type mockMigrationsRepository struct {
	doesExists  bool
	createError error

	repositories.MigrationsRepository
}

func (mr *mockMigrationsRepository) DoesExists() bool {
	return mr.doesExists
}

func (mr *mockMigrationsRepository) CreateTable() error {
	return mr.createError
}

func newMockRepo(doesExists bool, createError error) *repositories.Repositories {
	return &repositories.Repositories{
		Migrations: &mockMigrationsRepository{
			doesExists:  doesExists,
			createError: createError,
		},
	}
}

func TestGetLatestVersion(t *testing.T) {
	type testCase struct {
		name           string
		commands       []Command
		expectedSemver Semver
	}

	var (
		ver1 Semver = newSemver("1.1.0")
		ver2 Semver = newSemver("2.1.1")
		ver3 Semver = newSemver("3.0.1")
	)

	tt := []testCase{
		{
			name:           "returns bottomVersion, in case of empty slice",
			commands:       []Command{},
			expectedSemver: bottomVersion,
		},

		{
			name: "returns the actual latest, in case of non-empty slice",
			commands: []Command{
				newCommand(nil, "", ver1),
				newCommand(nil, "", ver3),
				newCommand(nil, "", ver2),
			},
			expectedSemver: ver3,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			gotSemver := getLatestVersion(tc.commands)

			if !reflect.DeepEqual(gotSemver, tc.expectedSemver) {
				t.Error("not expected result")
			}
		})
	}
}

func TestSetupDatabase(t *testing.T) {
	type testCase struct {
		name          string
		repo          *repositories.Repositories
		expectedError error
	}

	var createError error = errors.New("mock-error")

	tt := []testCase{
		{
			name:          "the function returns <nil> if the table exists",
			repo:          newMockRepo(true, nil),
			expectedError: nil,
		},
		{
			name:          "the function returns error, if table creation returns error",
			repo:          newMockRepo(false, createError),
			expectedError: createError,
		},
		{
			name:          "the function returns <nik>, if table creation no returnin an error",
			repo:          newMockRepo(false, nil),
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			e := &engine{
				repositories: tc.repo,
			}

			gotErr := e.SetupDatabase()
			if !errors.Is(gotErr, tc.expectedError) {
				t.Errorf("expected error: %v; got error: %v\n", tc.expectedError, gotErr)
			}
		})
	}
}

func TestParseLines(t *testing.T) {
	type testCase struct {
		name  string
		lines []string

		expectedCommands []Command
		expectedError    error
	}

	tt := []testCase{
		{
			name: "returns error in case of malformed versioning",
			lines: []string{
				"",
				"",
				"#v",
				"CREATE TABLE foo (",
				"id INTEGER NOT NULL,",
				"",
				"PRIMARY KEY (id)",
				");",
			},
			expectedCommands: nil,
			expectedError:    ErrBadVersioning,
		},

		{
			name: "returns error in case of bad versioning",
			lines: []string{
				"",
				"",
				"#va.a",
				"CREATE TABLE foo (",
				"id INTEGER NOT NULL,",
				"",
				"PRIMARY KEY (id)",
				");",
			},
			expectedCommands: nil,
			expectedError:    ErrBadVersioning,
		},

		{
			name: "returns empty slice in case of missing versioning",
			lines: []string{
				"",
				"",
				"CREATE TABLE foo (",
				"id INTEGER NOT NULL,",
				"",
				"PRIMARY KEY (id)",
				");",
			},
			expectedCommands: []Command{},
			expectedError:    nil,
		},

		{
			name: "returns only one command",
			lines: []string{
				"",
				"#v1.1.1",
				"-- creation of table foo",
				"CREATE TABLE foo (",
				"id INTEGER NOT NULL,",
				"",
				"PRIMARY KEY (id)",
				");",
			},
			expectedCommands: []Command{
				newCommand(nil, "CREATE TABLE foo ( id INTEGER NOT NULL, PRIMARY KEY (id) );", newSemver("1.1.1")),
			},
			expectedError: nil,
		},

		{
			name: "returns multiple commands",
			lines: []string{
				"",
				"#v1",
				"CREATE TABLE foo (",
				"id INTEGER NOT NULL,",
				"",
				"PRIMARY KEY (id)",
				");",
				"",
				"/*",
				"multiline comment example",
				"*/",
				"ALTER TABLE foo ADD COLUMN bar VARCHAR (10) DEFAULT NULL;",
				"",
				"#v1.2",
				"ALTER TABLE foo DROP COLUMN bar;",
			},
			expectedCommands: []Command{
				newCommand(nil, "CREATE TABLE foo ( id INTEGER NOT NULL, PRIMARY KEY (id) );", newSemver("1")),
				newCommand(nil, "ALTER TABLE foo ADD COLUMN bar VARCHAR (10) DEFAULT NULL;", newSemver("1")),
				newCommand(nil, "ALTER TABLE foo DROP COLUMN bar;", newSemver("1.2")),
			},
			expectedError: nil,
		},

		{
			name: "returns multiple commands with #[UP] & #[DOWN] semantics",
			lines: []string{
				"",
				"#v1",
				"#[UP]",
				"CREATE TABLE foo (",
				"id INTEGER NOT NULL,",
				"",
				"PRIMARY KEY (id)",
				");",
				"",
				"/*",
				"multiline comment example",
				"*/",
				"ALTER TABLE foo ADD COLUMN bar VARCHAR (10) DEFAULT NULL;",
				"",
				"#[DOWN]",
				"ALTER TABLE DROP foo DROP COLUMN bar;",
				"",
				"#v1.2",
				"#[UP]",
				"ALTER TABLE foo DROP COLUMN bar;",
				"",
				"#[DOWN]",
				"ALTER TABLE foo ADD COLUMN bar VARCHAR (10) DEFAULT NULL;",
				"#[UP]",
				"ALTER TABLE foo ADD COLUMN baz INTEGER NOT NULL;",
				"",
				"#v2",
				"#[UP]",
				"CREATE TABLE version_2 (",
				"id INTEGER NOT NULL",
				");",
				"#[DOWN]",
				"DROP TABLE version_2;",
			},
			expectedCommands: []Command{
				newCommand(nil, "CREATE TABLE foo ( id INTEGER NOT NULL, PRIMARY KEY (id) );", newSemver("1"), DirectionUp),
				newCommand(nil, "ALTER TABLE foo ADD COLUMN bar VARCHAR (10) DEFAULT NULL;", newSemver("1"), DirectionUp),
				newCommand(nil, "ALTER TABLE DROP foo DROP COLUMN bar;", newSemver("1"), DirectionDown),
				newCommand(nil, "ALTER TABLE foo DROP COLUMN bar;", newSemver("1.2"), DirectionUp),
				newCommand(nil, "ALTER TABLE foo ADD COLUMN bar VARCHAR (10) DEFAULT NULL;", newSemver("1.2"), DirectionDown),
				newCommand(nil, "ALTER TABLE foo ADD COLUMN baz INTEGER NOT NULL;", newSemver("1.2"), DirectionUp),
				newCommand(nil, "CREATE TABLE version_2 ( id INTEGER NOT NULL );", newSemver("2"), DirectionUp),
				newCommand(nil, "DROP TABLE version_2;", newSemver("2"), DirectionDown),
			},
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			e := &engine{}

			commands, err := e.ParseLines(tc.lines)

			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v; got error: %v\n", tc.expectedError, err)
			}

			if !reflect.DeepEqual(commands, tc.expectedCommands) {
				t.Error("not expected return value")
			}
		})
	}
}

func TestFilterCommands(t *testing.T) {
	type testCase struct {
		name     string
		version  Semver
		commands []Command
		dir      direction
		target   Semver

		expectedCommands []Command
	}

	var (
		c1 Command = newCommand(nil, "", newSemver("1.1.1"))
		c2 Command = newCommand(nil, "", newSemver("2.0.1"))
		c3 Command = newCommand(nil, "", newSemver("3.4.1"))
		c4 Command = newCommand(nil, "", newSemver("4.1.2"))
		c5 Command = newCommand(nil, "", newSemver("4.1.2"), DirectionDown)
	)

	tt := []testCase{
		{
			name:    "every command is returned, if the given semver is <nil>",
			version: nil,
			commands: []Command{
				c1,
				c2,
				c3,
				c4,
				c5,
			},
			dir: DirectionUp,
			expectedCommands: []Command{
				c1,
				c2,
				c3,
				c4,
			},
		},
		{
			name:    "only the newer commands are returned",
			version: newSemver("1.5.1"),
			commands: []Command{
				c1,
				c2,
				c3,
				c4,
				c5,
			},
			dir: DirectionUp,
			expectedCommands: []Command{
				c2,
				c3,
				c4,
			},
		},
		{
			name:    "empty slice is returned, if every command is already done",
			version: newSemver("4.1.2"),
			commands: []Command{
				c1,
				c2,
				c3,
				c4,
				c5,
			},
			dir:              DirectionUp,
			expectedCommands: []Command{},
		},
		{
			name:    "only the down command is returned",
			version: newSemver("4.1.2"),
			commands: []Command{
				c1,
				c2,
				c3,
				c4,
				c5,
			},
			dir:              DirectionDown,
			expectedCommands: []Command{c5},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			filtered := filterCommands(tc.version, tc.commands, tc.dir, tc.target)

			if !reflect.DeepEqual(filtered, tc.expectedCommands) {
				fmt.Println(filtered)
				t.Error("not expected return value")
			}
		})
	}
}

func TestGetPreviousSemver(t *testing.T) {
	type testCase struct {
		name     string
		cr       Semver
		commands []Command
		prev     Semver
	}

	tt := []testCase{
		{
			name:     "expecting bottomVersion in case of empty slice",
			cr:       newSemver("1.0.0"),
			commands: nil,
			prev:     bottomVersion,
		},
		{
			name: "expecting the right version",
			cr:   newSemver("1.2.1"),
			commands: []Command{
				newCommand(nil, "", newSemver("1.2.1")),
				newCommand(nil, "", newSemver("1.4.1")),
				newCommand(nil, "", newSemver("1.3.1")),
				newCommand(nil, "", newSemver("1.0.1")),
			},
			prev: newSemver("1.0.1"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := getPreviousSemver(tc.cr, tc.commands)

			if !reflect.DeepEqual(got, tc.prev) {
				t.Errorf("expected prev semver: %v; got: %v\n", tc.prev, got)
			}
		})
	}
}
