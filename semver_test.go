package dbmigrator

import (
	"reflect"
	"testing"
)

func TestNewSemver(t *testing.T) {
	type testCase struct {
		name     string
		input    string
		expected *semver
	}

	tt := []testCase{
		{
			name:     "returns <nil> in case of empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "returns <nil> in case of not valid semver",
			input:    "abc.asd.asd",
			expected: nil,
		},
		{
			name:     "returns semver ptr with only major version",
			input:    "12",
			expected: &semver{major: 12},
		},
		{
			name:     "returns semver ptr with major and minor version",
			input:    "1.2",
			expected: &semver{major: 1, minor: 2},
		},
		{
			name:     "returns semver ptr with major, minor and patch version",
			input:    "1.2.3",
			expected: &semver{major: 1, minor: 2, patch: 3},
		},
		{
			name:     "returns semver ptr with major, minor (char) and patch version",
			input:    "1.fo.3",
			expected: &semver{major: 1, minor: 0, patch: 3},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := newSemver(tc.input)

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("not equal to expected value")
			}
		})
	}
}

func TestGreaterThan(t *testing.T) {
	type testCase struct {
		name string
		sw1  *semver
		sw2  *semver

		isGreater bool
	}

	tt := []testCase{
		{
			name:      "greater by major",
			sw1:       &semver{major: 2},
			sw2:       &semver{major: 1},
			isGreater: true,
		},
		{
			name:      "greater by minor",
			sw1:       &semver{major: 1, minor: 2},
			sw2:       &semver{major: 1, minor: 1},
			isGreater: true,
		},
		{
			name:      "greater by patch",
			sw1:       &semver{major: 1, minor: 1, patch: 2},
			sw2:       &semver{major: 1, minor: 1, patch: 1},
			isGreater: true,
		},
		{
			name:      "not greater by major",
			sw1:       &semver{major: 1},
			sw2:       &semver{major: 2},
			isGreater: false,
		},
		{
			name:      "not greater by minor",
			sw1:       &semver{major: 1, minor: 1},
			sw2:       &semver{major: 1, minor: 2},
			isGreater: false,
		},
		{
			name:      "not greater by patch",
			sw1:       &semver{major: 1, minor: 1, patch: 1},
			sw2:       &semver{major: 1, minor: 1, patch: 2},
			isGreater: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.sw1.greaterThan(tc.sw2)

			if got != tc.isGreater {
				t.Errorf("expected to be: %t; got: %t\n", tc.isGreater, got)
			}
		})
	}
}
