//go:build g3trace

package sat

// Source: src/sat/sat_solver.cpp, solver::exchange_par's IF_VERBOSE(1,
// ...) line. New verbose ports are built only with the g3trace tag.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func satSyncVerbose(numOut, numIn uint32) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(sat-sync out: "); err != nil {
		return err
	}
	if err = out.AppendUint32(numOut); err != nil {
		return err
	}
	if err = out.AppendCString(" in: "); err != nil {
		return err
	}
	if err = out.AppendUint32(numIn); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}
