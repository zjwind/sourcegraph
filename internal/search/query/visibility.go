package query

import (
	"fmt"
	"strings"
)

type RepoVisibility int8

const (
	Any RepoVisibility = iota
	Private
	Public
)

func ParseVisibility(s string) (RepoVisibility, error) {
	switch strings.ToLower(s) {
	case "private":
		return Private, nil
	case "public":
		return Public, nil
	case "any":
		return Any, nil
	}

	return Any, fmt.Errorf("invalid value %q for field visibility", s)
}
