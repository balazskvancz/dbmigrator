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
		shouldRun bool
	}

	tt := []testCase{
		{
			name: "the compared version is older = run",
			c: &command{
				version: newSemver("2.1.1."),
			},
			cmp:       newSemver("1.1.1"),
			shouldRun: true,
		},
		{
			name: "the compared version is older = no run",
			c: &command{
				version: newSemver("1.1.1."),
			},
			cmp:       newSemver("1.2.1"),
			shouldRun: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			shouldRun := tc.c.ShouldRun(tc.cmp)

			if shouldRun != tc.shouldRun {
				t.Errorf("expected shouldRun: %t; got: %t\n", tc.shouldRun, shouldRun)
			}
		})
	}
}
