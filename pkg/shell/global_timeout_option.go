package shell

// Source: src/shell/main.cpp:207-214, the -T option branch.

import (
	"context"
	"errors"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/glycerine/g3/pkg/util/text"
)

var ErrMissingGlobalTimeout = errors.New("option argument (-T:timeout) is missing.")
var ErrGlobalTimeoutRange = errors.New("global timeout seconds exceed the defined signed-long multiplication range")

// parseGlobalTimeoutOption consumes the colon argument after shell/main has
// selected -T. The source reads a signed decimal prefix with strtol, multiplies
// by 1000, then passes that long to set_timeout. Go's strconv handles the
// decimal conversion; overflowing source multiplication is an undefined C++
// domain and is rejected here before it can silently wrap.
func parseGlobalTimeoutOption(argument *string) (uint32, error) {
	if argument == nil {
		return 0, ErrMissingGlobalTimeout
	}
	s := (*argument)[:text.CStringLength(*argument)]
	i := 0
	for i < len(s) {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\v' && s[i] != '\f' && s[i] != '\r' {
			break
		}
		i++
	}
	start := i
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	var seconds int64
	if i != digits {
		parsed, err := strconv.ParseInt(s[start:i], 10, 64)
		if err != nil {
			if s[start] == '-' {
				seconds = math.MinInt64
			} else {
				seconds = math.MaxInt64
			}
		} else {
			seconds = parsed
		}
	}
	if seconds > math.MaxInt64/1000 || seconds < math.MinInt64/1000 {
		return 0, ErrGlobalTimeoutRange
	}
	return uint32(seconds * 1000), nil
}

// WithTimeout derives one cancellable context for the command-line invocation.
// Zero and UINT_MAX disable the shell deadline while preserving the parent's.
func (c *CommandLine) WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	ms := c.TimeoutMilliseconds
	if ms == 0 || ms == math.MaxUint32 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, time.Duration(ms)*time.Millisecond)
}

func reportDeadline(ctx context.Context, output io.Writer) bool {
	if ctx.Err() != context.DeadlineExceeded {
		return false
	}
	_, _ = io.WriteString(output, "timeout\n")
	return true
}
