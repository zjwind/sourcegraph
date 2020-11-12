package errutil

import (
	"github.com/hashicorp/go-multierror"
)

// Combine returns a multierror containing all of the non-nil error parameter values.
//
// This method should be used over multierror when it is not guaranteed that the original
// error was non-nil. multierror.Append creates a non-nil error even if it is empty).
func Combine(errs ...error) (err error) {
	for _, e := range errs {
		if e == nil {
			continue
		}

		if err == nil {
			err = e
		} else {
			err = multierror.Append(err, e)
		}
	}

	return err
}
