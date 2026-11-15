package fp

// Source: src/muz/rel/rel_context.cpp rel_context::saturate:
//
//	if (m_context.print_aig().size()) {
//	    aig_exporter aig(m_context.get_rules(), get_context(), &m_table_facts);
//	    std::ofstream strm(filename, std::ios_base::binary);
//	    aig(strm);
//	    exit(0);
//	}
//
// exit(0) does not run the ofstream destructor, so the buffered AAG text is
// never flushed: for every output smaller than the stream buffer z3 4.7.1
// leaves an empty file. g3 reproduces that observable result (the file is
// created or truncated, then the process stops with status 0).

import (
	"errors"
	"os"

	"github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// SetDatalogFrontend marks an engine driven by the Datalog file frontend
// (read_datalog), which compiles and executes rules without
// rel_context::saturate.
func (e *PropositionalEngine) SetDatalogFrontend(enabled bool) { e.datalogFrontend = enabled }

func (e *PropositionalEngine) datalogPrintAIG() (resultErr error) {
	if e.params == nil || e.globals == nil || e.datalogFrontend {
		return nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return err
	}
	selection, err := view.Symbol("print_aig", symbol.Symbol{})
	if err = errors.Join(err, view.Close()); err != nil {
		return err
	}
	if selection.Value == 0 {
		return nil
	}
	filename, _, err := e.manager.Symbols.BareString(selection)
	if err != nil {
		return err
	}
	if filename == "" {
		return nil
	}
	if file, openErr := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666); openErr == nil {
		_ = file.Close()
	}
	return &z3exception.ProcessExit{Status: 0}
}
