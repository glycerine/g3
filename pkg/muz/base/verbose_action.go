package base

// Source: src/muz/base/dl_util.cpp verbose_action: IF_VERBOSE(lvl,
// "msg...") with a started stopwatch on construction, and
// IF_VERBOSE(lvl, sec << "s\n") on destruction, where sec below 0.001 is
// written as 0. AUDIT.md FUZZ-F5: only g3trace builds print it.

import (
	"time"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// VerboseActionDefaultLevel is verbose_action's default lvl argument.
const VerboseActionDefaultLevel = 11

// VerboseAction is one live verbose_action. The zero value is inactive.
type VerboseAction struct {
	level  uint32
	start  time.Time
	active bool
}

// StartVerboseAction is verbose_action(msg, level).
func StartVerboseAction(msg string, level uint32) (VerboseAction, error) {
	if !diagnostic.VerboseProgress {
		return VerboseAction{}, nil
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(level)
	if err != nil || !enabled {
		return VerboseAction{}, err
	}
	if err = diagnostic.ProgressVerbose(level, msg+"..."); err != nil {
		return VerboseAction{}, err
	}
	return VerboseAction{level: level, start: time.Now(), active: true}, nil
}

// End is ~verbose_action.
func (a VerboseAction) End() error {
	if !diagnostic.VerboseProgress || !a.active {
		return nil
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(a.level)
	if err != nil || !enabled {
		return err
	}
	seconds := time.Since(a.start).Seconds()
	if seconds < 0.001 {
		seconds = 0
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendFloat64(seconds); err != nil {
		return err
	}
	if err = out.AppendCString("s\n"); err != nil {
		return err
	}
	return out.Flush()
}
