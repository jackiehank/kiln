//go:build windows

package cli

import (
	"errors"
	"io"
	"syscall"
)

// errnoNoData is Windows' ERROR_NO_DATA. The syscall package names
// ERROR_BROKEN_PIPE (109) but not this one (232), and a consumer that closed
// its end can surface either.
const errnoNoData = syscall.Errno(232)

// isBrokenPipe reports whether err means "the consumer closed its end of the
// pipe". Windows has no SIGPIPE, so the write returns an error instead of
// terminating the process: treating it as a failure would make
// `kiln action log | head` print an error that Unix never shows.
func isBrokenPipe(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ERROR_BROKEN_PIPE) ||
		errors.Is(err, errnoNoData)
}
