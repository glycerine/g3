package cmd_context

// Source: src/cmd_context/tactic_cmds.cpp:309-347, apply result views.

import (
	"io"

	"github.com/glycerine/g3/pkg/tactic"
)

func (c *Context) writeAppliedTacticResult(out io.Writer, applied *tactic.TacticResult, settings applyTacticSettings) error {
	if settings.print {
		if _, err := io.WriteString(out, "(goals\n"); err != nil {
			return err
		}
		for _, goal := range applied.Goals {
			// goal::display(ast_printer_context& ctx): ctx is the
			// cmd_context, whose pp_env prints declarations.
			err := c.withPPEnv(func() error {
				if settings.printDependencies {
					return goal.DisplayWithDependencies(out)
				}
				return goal.DisplayWithAttributes(out)
			})
			if err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	// Source accepts print_proof but leaves its implementation as TODO.
	_ = settings.printProof
	if settings.printBenchmark {
		if err := c.writeAppliedTacticBenchmark(out, applied.Goals); err != nil {
			return err
		}
	}
	if settings.printModelConverter && applied.ModelConverter != nil {
		return applied.ModelConverter.Display(out)
	}
	return nil
}
