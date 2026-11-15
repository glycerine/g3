package fp

// Source: rel_context::display_output_facts and
// relation_manager::display_output_tables for the live finite frontend.

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/rel"
	"io"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) WriteDatalogOutputTables(names *base.DatalogContext, out io.Writer) error {
	// Readers outside the query flush see the eager copies of queued rules.
	if err := e.materializePendingRules(); err != nil {
		return err
	}
	if err := e.open(); err != nil {
		return err
	}
	if e.backend != "datalog" || !e.queried {
		return ErrPropositionalEngine
	}
	// The query scope has restored the source output-pragmas here. In
	// particular, QueryPredicates(all) must not print every requested result.
	return e.relations.DisplayOutputTables(names, e.set, out)
}

// SetDatalogConstantNames borrows the front end's datalog context for
// context::print_constant_name in instruction displays.
func (e *PropositionalEngine) SetDatalogConstantNames(names *base.DatalogContext) {
	if e != nil {
		e.constantNames = names
	}
}

// WriteDatalogStatisticsDump is read_datalog's display_statistics sections
// under IF_VERBOSE(2): the original and generated rules, the compiled
// instructions displayed against the retained execution context, its big
// relations, and the relation sizes. code.process_all_costs() runs first.
// (display_statistics' output_profile/profile_milliseconds_threshold
// update names unprefixed keys, which the fixedpoint parameters ignore.)
func (e *PropositionalEngine) WriteDatalogStatisticsDump(original *base.RuleSet, out io.Writer) error {
	if err := e.open(); err != nil {
		return err
	}
	if e.datalogCode != nil {
		if err := e.datalogCode.ProcessAllCosts(); err != nil {
			return err
		}
	}
	generated := e.set
	if e.datalogTransformedRules != nil {
		generated = e.datalogTransformedRules
	}
	if _, err := io.WriteString(out, "--------------\noriginal rules\n"); err != nil {
		return err
	}
	if original == nil {
		original = e.set
	}
	if err := original.WriteDisplay(out); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "---------------\ngenerated rules\n"); err != nil {
		return err
	}
	if err := generated.WriteDisplay(out); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "--------------\ninstructions  \n"); err != nil {
		return err
	}
	if e.datalogCode != nil {
		if err := e.datalogCode.DisplayIndented(e.datalogExecution, out, ""); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(out, "--------------\nbig relations \n"); err != nil {
		return err
	}
	// read_datalog's ex_ctx exists even when no code ran; an empty one
	// reports no bytes.
	execution := e.datalogExecution
	if execution == nil {
		empty := rel.NewExecutionContext(nil)
		err := empty.ReportBigRelations(1000, out)
		if err = errors.Join(err, empty.Close()); err != nil {
			return err
		}
	} else if err := execution.ReportBigRelations(1000, out); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "--------------\nrelation sizes\n"); err != nil {
		return err
	}
	return e.relations.WriteRelationSizes(out)
}
