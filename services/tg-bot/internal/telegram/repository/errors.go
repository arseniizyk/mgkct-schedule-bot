package repository

import (
	"errors"
	"fmt"
)

var (
	ErrBuildQuery = errors.New("repository: failed to build SQL query")
	ErrExec       = errors.New("repository: failed to exec SQL query")
	ErrQuery      = errors.New("repository: failed to execute SQL query")
	ErrScan       = errors.New("repository: failed to scan row")
	ErrNoGroup    = errors.New("repository: user has no group")
	ErrRows       = errors.New("repository: failed to parse rows")
)

func wrap(base, err error, msg ...string) error {
	if err == nil {
		return nil
	}
	if len(msg) > 0 {
		return fmt.Errorf("%w (%s): %v", base, msg[0], err)
	}
	return fmt.Errorf("%w: %v", base, err)
}
