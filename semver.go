package dbmigrator

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	versionSeparator string = "."
)

type semver struct {
	major int
	minor int
	patch int
}

type Semver interface {
	GreaterThan(s Semver) bool
	ToString() string

	GetMajor() int
	GetMinor() int
	GetPatch() int
}

func newSemver(str string) Semver {
	if str == "" {
		return nil
	}

	var (
		sv  = &semver{}
		spl = strings.Split(str, versionSeparator)
	)

	for i, e := range spl {
		conv, err := strconv.Atoi(e)
		if err != nil {
			continue
		}

		// Version cant be less than zero.
		if conv < 0 {
			continue
		}

		switch i {
		case 0:
			sv.major = conv
		case 1:
			sv.minor = conv
		case 2:
			sv.patch = conv
		}
	}

	// At least one should be higher than zero.
	if sv.major == 0 && sv.minor == 0 && sv.patch == 0 {
		return nil
	}

	return sv
}

func (sv *semver) GreaterThan(cmp Semver) bool {
	if sv.major > cmp.GetMajor() {
		return true
	}

	if sv.major == cmp.GetMajor() && sv.minor > cmp.GetMinor() {
		return true
	}

	if sv.major == cmp.GetMajor() && sv.minor == cmp.GetMinor() && sv.patch > cmp.GetPatch() {
		return true
	}

	return false
}

func (sv *semver) ToString() string {
	return fmt.Sprintf("%d.%d.%d", sv.major, sv.minor, sv.patch)
}

// GetMajor return the major version of the semver.
func (sv *semver) GetMajor() int { return sv.major }

// GetMinor return the minor version of the semver.
func (sv *semver) GetMinor() int { return sv.minor }

// GetPatch return the patch version of the semver.
func (sv *semver) GetPatch() int { return sv.patch }
