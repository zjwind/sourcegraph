package errutil

import "errors"

// Matches returns true if the predicate returns true for any error in the given error chain.
func Matches(err error, predicate func(err error) bool) bool {
	for err != nil {
		if predicate(err) {
			return true
		}

		err = errors.Unwrap(err)
	}

	return false
}
