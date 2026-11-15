package shell

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type DatalogFrontendTimings struct{ Parsing, Other, Total int64 }

func (f *DatalogFrontend) Timings() (DatalogFrontendTimings, error) {
	if f == nil || !f.saturated {
		return DatalogFrontendTimings{}, ErrDatalogFrontend
	}
	parsing := f.parseFinished.Sub(f.started).Milliseconds()
	other := f.runFinished.Sub(f.parseFinished).Milliseconds()
	return DatalogFrontendTimings{Parsing: parsing, Other: other, Total: parsing + other}, nil
}

func (f *DatalogFrontend) WriteStatistics() error {
	timing, err := f.Timings()
	if err != nil {
		return err
	}
	// display_statistics' IF_VERBOSE(2) sections go to its out, std::cout
	// (AUDIT.md FUZZ-F5; g3trace only).
	if diagnostic.VerboseProgress {
		enabled, verr := diagnostic.ProcessVerboseEnabled(2)
		if verr != nil {
			return verr
		}
		if enabled {
			if err = f.context.WriteFixedpointDatalogStatisticsDump(f.output); err != nil {
				return err
			}
		}
	}
	_, err = fmt.Fprintf(f.output, "Time: %dms\nParsing: %dms, other: %dms\n", timing.Total, timing.Parsing, timing.Other)
	return err
}
