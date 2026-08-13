package toolchain

import (
	"fmt"
	"regexp"
	"strconv"
)

var numericVersionPattern = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?([-+][0-9A-Za-z.-]+)?$`)

type Version struct {
	Raw   string
	Major int
	Minor int
	Patch int

	prerelease bool
}

func ParseVersion(value string) (Version, error) {
	match := numericVersionPattern.FindStringSubmatch(value)
	if len(match) == 0 {
		return Version{}, fmt.Errorf("%q is not a supported numeric version", value)
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch := 0
	if match[3] != "" {
		patch, _ = strconv.Atoi(match[3])
	}
	return Version{Raw: value, Major: major, Minor: minor, Patch: patch, prerelease: len(match[4]) > 0 && match[4][0] == '-'}, nil
}

func (v Version) Compare(other Version) int {
	left := []int{v.Major, v.Minor, v.Patch}
	right := []int{other.Major, other.Minor, other.Patch}
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	if v.prerelease != other.prerelease {
		if v.prerelease {
			return -1
		}
		return 1
	}
	return 0
}

func (v Version) Line() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}
