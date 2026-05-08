// Package registry implements the workflow registry: storing workflows and
// their immutable, SemVer'd versions, and detecting breaking changes between
// versions.
package registry

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SemVer is a parsed Major.Minor.Patch[-Pre][+Build].
type SemVer struct {
	Major, Minor, Patch int
	Pre                 string
	Build               string
}

// ErrInvalidSemVer indicates the input did not parse as SemVer 2.0.
var ErrInvalidSemVer = errors.New("invalid_semver")

// ParseSemVer parses a SemVer 2.0 string.
func ParseSemVer(s string) (SemVer, error) {
	if s == "" {
		return SemVer{}, ErrInvalidSemVer
	}
	main := s
	build := ""
	if idx := strings.Index(s, "+"); idx >= 0 {
		main = s[:idx]
		build = s[idx+1:]
	}
	pre := ""
	if idx := strings.Index(main, "-"); idx >= 0 {
		pre = main[idx+1:]
		main = main[:idx]
	}
	parts := strings.Split(main, ".")
	if len(parts) != 3 {
		return SemVer{}, ErrInvalidSemVer
	}
	mj, e1 := strconv.Atoi(parts[0])
	mi, e2 := strconv.Atoi(parts[1])
	pa, e3 := strconv.Atoi(parts[2])
	if e1 != nil || e2 != nil || e3 != nil || mj < 0 || mi < 0 || pa < 0 {
		return SemVer{}, ErrInvalidSemVer
	}
	return SemVer{Major: mj, Minor: mi, Patch: pa, Pre: pre, Build: build}, nil
}

// String renders the canonical form.
func (v SemVer) String() string {
	out := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		out += "-" + v.Pre
	}
	if v.Build != "" {
		out += "+" + v.Build
	}
	return out
}

// Compare returns -1/0/+1 — pre-release ordering follows SemVer 2.0.
func (v SemVer) Compare(o SemVer) int {
	switch {
	case v.Major != o.Major:
		return cmpInt(v.Major, o.Major)
	case v.Minor != o.Minor:
		return cmpInt(v.Minor, o.Minor)
	case v.Patch != o.Patch:
		return cmpInt(v.Patch, o.Patch)
	}
	switch {
	case v.Pre == "" && o.Pre == "":
		return 0
	case v.Pre == "" && o.Pre != "":
		return 1 // release > pre
	case v.Pre != "" && o.Pre == "":
		return -1
	}
	return strings.Compare(v.Pre, o.Pre)
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// BumpKind describes the level of change between two versions.
type BumpKind string

const (
	BumpMajor BumpKind = "major"
	BumpMinor BumpKind = "minor"
	BumpPatch BumpKind = "patch"
)

// MinNextVersion returns the smallest SemVer that satisfies a given bump.
func MinNextVersion(prev SemVer, bump BumpKind) SemVer {
	switch bump {
	case BumpMajor:
		return SemVer{Major: prev.Major + 1, Minor: 0, Patch: 0}
	case BumpMinor:
		return SemVer{Major: prev.Major, Minor: prev.Minor + 1, Patch: 0}
	default:
		return SemVer{Major: prev.Major, Minor: prev.Minor, Patch: prev.Patch + 1}
	}
}
