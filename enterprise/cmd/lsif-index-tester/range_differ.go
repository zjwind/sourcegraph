package main

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

func sameLine(a, b Location) bool {
	return a.Range.Start.Line == b.Range.Start.Line &&
		a.Range.End.Line == b.Range.End.Line &&
		a.Range.Start.Line == a.Range.End.Line
}

func header(l Location) string {
	return fmt.Sprintf("%s:%d", l.URI, l.Range.Start.Line)
}
func lineCarets(r Range, name string) string {
	return fmt.Sprintf("%s%s %s",
		strings.Repeat(" ", r.Start.Character),
		strings.Repeat("^", r.End.Character-r.Start.Character),
		name,
	)

}

// src/header.c:1
// void exported_funct() {
//       ^^^^^^^^^^^^^^^ expected
//      ^^^^^^^^^^^^^^^^ actual
//
// Only operates on locations with the same URI.
//    It doesn't make sense to diff anything here when we don't have that.
func DrawLocations(contents string, expected, actual Location) (string, error) {
	if expected.URI != actual.URI {
		return "", errors.New("Must pass in two locations with the same URI")
	}

	if expected == actual {
		return "", errors.New("You can't pass in two locations that are the same")
	}

	splitLines := strings.Split(contents, "\n")
	if sameLine(expected, actual) {
		line := expected.Range.Start.Line

		if line > len(splitLines) {
			return "", errors.New("Line does not exist in contents")
		}

		text := fmt.Sprintf("%s\n%s\n%s\n%s",
			header(expected),
			splitLines[line],
			lineCarets(expected.Range, "expected"),
			lineCarets(actual.Range, "actual"),
		)

		return text, nil
	}

	return "failed", nil
}
