package cmd_context

// Source: src/opt/opt_context.cpp:1458-1528 (context::to_string).

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/opt"

	"github.com/glycerine/g3/pkg/ast"
	usmt2 "github.com/glycerine/g3/pkg/util/smt2"
)

// WriteOptimizationSMT2 is opt::context::to_string() (Z3_optimize_to_string)
// over the scoped state: ast_pp_util collects the hard constraints and each
// objective's term(s), prints the declarations and the asserts (pp.neat),
// then the objectives, the opt parameters with display_smt2 and
// (check-sat).
func (c *Context) WriteOptimizationSMT2(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	m, err := c.Manager()
	if err != nil {
		return err
	}
	hard := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		hard[i] = c.assertions[i].expression
	}
	visitor, err := ast.NewASTPPUtil(m)
	if err != nil {
		return err
	}
	if err = visitor.CollectAll(hard); err != nil {
		return err
	}
	for i := range c.objectives {
		objective := &c.objectives[i]
		if objective.soft {
			for j := range objective.softTerms {
				if err = visitor.Collect(objective.softTerms[j].expression); err != nil {
					return err
				}
			}
			continue
		}
		if err = visitor.Collect(objective.expression); err != nil {
			return err
		}
	}
	if err = visitor.DisplayDeclarations(out); err != nil {
		return err
	}
	// m_pp_neat: opt_params(m_params).pp_neat().
	optimization, err := opt.NewParams(c.config.Params, c.config.Globals, &c.params)
	if err != nil {
		return err
	}
	neat, err := optimization.PPNeat()
	if err = errors.Join(err, optimization.Close()); err != nil {
		return err
	}
	if err = visitor.DisplayAssertions(out, hard, neat); err != nil {
		return err
	}
	// ast_smt2_pp with smt2_pp_environment_dbg.
	printer, err := ast.NewSimpleASTPrinterContext(m, out, nil)
	if err != nil {
		return err
	}
	write := func(text string) error {
		_, e := io.WriteString(out, text)
		return e
	}
	for i := range c.objectives {
		objective := &c.objectives[i]
		if !objective.soft {
			command := "(minimize "
			if objective.maximize {
				command = "(maximize "
			}
			if err = write(command); err != nil {
				return err
			}
			if err = printer.DisplayExpr(out, objective.expression, 0); err != nil {
				return err
			}
			if err = write(")\n"); err != nil {
				return err
			}
			continue
		}
		for j := range objective.softTerms {
			term := &objective.softTerms[j]
			if err = write("(assert-soft "); err != nil {
				return err
			}
			if err = printer.DisplayExpr(out, term.expression, 0); err != nil {
				return err
			}
			// w.display_decimal(out << " :weight ", 3, true)
			weight, e := c.optQ().DecimalString(&term.weight, 3, true)
			if e != nil {
				return e
			}
			if err = write(" :weight " + weight); err != nil {
				return err
			}
			if id, idErr := c.softObjectiveIDText(objective); idErr != nil {
				return idErr
			} else if id != "" {
				if err = write(" :id " + id); err != nil {
					return err
				}
			}
			if err = write(")\n"); err != nil {
				return err
			}
		}
	}
	if err = c.writeOptimizationParamsSMT2(out); err != nil {
		return err
	}
	return write("(check-sat)\n")
}

// softObjectiveIDText is the " :id " text of an objective: nothing for
// symbol::null, mk_smt2_quoted_symbol when is_smt2_quoted_symbol (the
// non-null empty symbol prints ||), else the symbol.
func (c *Context) softObjectiveIDText(objective *optimizationObjective) (string, error) {
	if objective.softKey.emptySymbol {
		return "||", nil
	}
	if objective.id == "" {
		return "", nil
	}
	symbol, err := c.manager.Symbols.Intern(objective.id)
	if err != nil {
		return "", err
	}
	quoted, err := usmt2.IsSMT2QuotedSymbol(c.manager.Symbols, symbol)
	if err != nil || !quoted {
		return objective.id, err
	}
	return usmt2.MkSMT2QuotedSymbol(c.manager.Symbols, symbol, nil)
}

// writeOptimizationParamsSMT2 is m_params.display_smt2(out, "opt", descrs):
// "(set-option :opt.<name> <value>)" for each parameter the opt module
// describes.
func (c *Context) writeOptimizationParamsSMT2(out io.Writer) error {
	descrs, err := c.config.Globals.SourceModuleDescriptorsName("opt")
	if err != nil {
		return err
	}
	module := "opt"
	text, err := c.config.Params.DisplaySMT2(&c.params, &module, descrs)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
