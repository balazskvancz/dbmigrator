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

func newSemver(str string) *semver {
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

func (sv *semver) greaterThan(cmp *semver) bool {
	if sv.major > cmp.major {
		return true
	}

	if sv.minor > cmp.minor {
		return true
	}

	if sv.patch > cmp.patch {
		return true
	}

	return false
}

func (sv *semver) toString() string {
	return fmt.Sprintf("%d.%d.%d", sv.major, sv.minor, sv.patch)
}
