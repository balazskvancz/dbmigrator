package dbmigrator

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/balazskvancz/dbmigrator/database"
)

type mockDatabase struct {
	execError error

	database.Database
}

func (md *mockDatabase) Exec(query string, _ ...any) (sql.Result, error) {
	return nil, md.execError
}

func newMockDatabase(execError error) database.Database {
	return &mockDatabase{
		execError: execError,
	}
}

func TestRun(t *testing.T) {
	type testCase struct {
		name          string
		command       Command
		expectedError error
	}

	var err error = errors.New("thrown db error")

	tt := []testCase{
		{
			name: "run not returns error",
			command: &command{
				db: newMockDatabase(nil),
			},
			expectedError: nil,
		},
		{
			name: "run returns error",
			command: &command{
				db: newMockDatabase(err),
			},
			expectedError: err,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.command.Run()

			if !errors.Is(gotErr, tc.expectedError) {
				t.Errorf("expected error: %v; got error: %v\n", tc.expectedError, gotErr)
			}
		})
	}
}

func TestShouldRun(t *testing.T) {
	type testCase struct {
		name      string
		c         Command
		cmp       Semver
		dir       direction
		target    Semver
		shouldRun bool
	}

	tt := []testCase{
		{
			name: "the compared version is older = run (up)",
			c: &command{
				version: newSemver("2.1.1."),
			},
			cmp:       newSemver("1.1.1"),
			dir:       DirectionUp,
			shouldRun: true,
		},
		{
			name: "the compared version is older = no run (down)",
			c: &command{
				version: newSemver("2.1.1."),
			},
			cmp:       newSemver("1.1.1"),
			dir:       DirectionDown,
			shouldRun: false,
		},
		{
			name: "the compared version is newer = no run (up)",
			c: &command{
				version: newSemver("1.1.1."),
			},
			cmp:       newSemver("1.2.1"),
			dir:       DirectionUp,
			shouldRun: false,
		},
		{
			name: "the compared version is newer = no run (down)",
			c: &command{
				version: newSemver("1.1.1."),
			},
			cmp:       newSemver("1.2.1"),
			dir:       DirectionDown,
			shouldRun: false,
		},

		{
			name: "the compared version is the same = no run (up)",
			c: &command{
				version: newSemver("1.1.1."),
			},
			cmp:       newSemver("1.1.1"),
			dir:       DirectionUp,
			shouldRun: false,
		},
		{
			name: "the compared version is the same = run (down)",
			c: &command{
				version: newSemver("1.1.1."),
			},
			cmp:       newSemver("1.1.1"),
			dir:       DirectionDown,
			shouldRun: true,
		},

		{
			name: "the compared version is higher than the target = no run (up)",
			c: &command{
				version: newSemver("1.0.1."),
			},
			cmp:       newSemver("1.1.1"),
			dir:       DirectionUp,
			target:    newSemver("1.1"),
			shouldRun: false,
		},
		{
			name: "the compared version is higher than the target = no run (up) 2",
			c: &command{
				version: newSemver("1.2.1."),
			},
			cmp:       newSemver("1.1.1"),
			dir:       DirectionUp,
			target:    newSemver("1.2"),
			shouldRun: false,
		},
		{
			name: "the compared version is lower than the target = run (up)",
			c: &command{
				version: newSemver("1.2.1."),
			},
			cmp:       newSemver("1.1.1"),
			dir:       DirectionUp,
			target:    newSemver("1.3"),
			shouldRun: true,
		},
		{
			name: "the compared version is lower than the target = no run (down)",
			c: &command{
				version: newSemver("1.2.1."),
			},
			cmp:       newSemver("1.4.1"),
			dir:       DirectionDown,
			target:    newSemver("1.2.2"),
			shouldRun: false,
		},
		{
			name: "the compared version is higher than the target = run (down)",
			c: &command{
				version: newSemver("1.2.1."),
			},
			cmp:       newSemver("1.4.1"),
			dir:       DirectionDown,
			target:    newSemver("1.2"),
			shouldRun: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			shouldRun := tc.c.ShouldRun(tc.cmp, tc.dir, tc.target)

			if shouldRun != tc.shouldRun {
				t.Errorf("expected shouldRun: %t; got: %t\n", tc.shouldRun, shouldRun)
			}
		})
	}
}
