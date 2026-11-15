package cmd_context

// Source: context::register_predicate/set_argument_names and
// rel_context::display_output_facts. The command session owns source names
// after the file parser has released its temporary AST references.

import (
	"fmt"
	"io"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// rememberDatalogDisplay registers the parsed predicates with the display
// owner. The parser numbers constants in the owner's sort domains as it
// reads them (context::get_constant_number).
func (c *Context) rememberDatalogDisplay(p *datalogParser) error {
	owner := c.datalogDisplay
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	declarations, err := engine.Predicates()
	if err != nil {
		return err
	}
	for _, declaration := range declarations {
		known, err := owner.IsPredicate(declaration)
		if err != nil {
			return err
		}
		if known {
			continue
		}
		if err = owner.RegisterPredicate(declaration, false); err != nil {
			return err
		}
		arity, err := p.manager.GetArity(declaration)
		if err != nil {
			return err
		}
		arguments := make([]usymbol.Symbol, arity)
		for i := range arguments {
			arguments[i], err = engine.ArgumentName(declaration, uint32(i))
			if err != nil {
				return err
			}
		}
		if err = owner.SetArgumentNames(declaration, arguments); err != nil {
			return err
		}
	}
	return nil
}

// FixedpointDatalogPrintTuples reads the current source output option before
// the shell starts its query-local saturation attempt.
func (c *Context) FixedpointDatalogPrintTuples() (bool, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return false, err
	}
	return engine.DatalogPrintTuples()
}

// WriteFixedpointDatalogTuples prints the original pragma outputs without
// issuing another query or changing the copied numeric result snapshots.
func (c *Context) WriteFixedpointDatalogTuples(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if c.datalogDisplay == nil {
		return fmt.Errorf("datalog: no parsed display domains")
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.WriteDatalogOutputTables(c.datalogDisplay, out)
}

// WriteFixedpointDatalogStatisticsDump writes read_datalog's IF_VERBOSE(2)
// display_statistics sections (rules, instructions, big relations,
// relation sizes) for the last saturation.
func (c *Context) WriteFixedpointDatalogStatisticsDump(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.WriteDatalogStatisticsDump(nil, out)
}
