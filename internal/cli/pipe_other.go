//go:build !windows

package cli

import (
	"errors"
	"io"
	"syscall"
)

// isBrokenPipe reports whether err means "the consumer closed its end of the
// pipe". On Unix a write to a closed stdout normally raises SIGPIPE and ends the
// process before this check is reached (the same convention git and ripgrep
// follow); the check still covers writers that hand back EPIPE directly, such as
// tests and non-stdout destinations.
func isBrokenPipe(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) || errors.Is(err, syscall.EPIPE)
}
