package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp IF_VERBOSE sites of qlinear::check,
// nonlinear::check, nonlinear::get_proof, nonlinear::get_model and
// linear::check.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func bmcVerboseStream(minimum uint32) (*utext.NumericTextStream, bool, error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(minimum)
	if err != nil || !enabled {
		return nil, false, err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	return out, err == nil, err
}

// verboseLine is IF_VERBOSE(minimum, verbose_stream() << format ...).
func verboseLine(minimum uint32, format string, args ...any) error {
	out, enabled, err := bmcVerboseStream(minimum)
	if err != nil || !enabled {
		return err
	}
	return out.AppendCString(fmt.Sprintf(format, args...))
}

// verboseExpression is IF_VERBOSE(minimum, verbose_stream() << mk_pp(e, m) << "\n").
func verboseExpression(m *ast.ASTManager, minimum uint32, expression ast.ASTHandle) (result error) {
	out, enabled, err := bmcVerboseStream(minimum)
	if err != nil || !enabled {
		return err
	}
	context, globals, err := m.RewriterParameterServices()
	if err != nil {
		return err
	}
	var format ast.ASTHandle
	if context != nil && globals != nil {
		format, err = m.SMT2PPExprTreeWithParams(expression, nil, context, globals, nil)
	} else {
		format, err = m.SMT2PPExprTree(expression, nil)
	}
	if err != nil {
		return err
	}
	formats, err := m.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	if context != nil && globals != nil {
		empty := context.EmptyValue()
		err = m.PrintFormat(out, format, context, &empty, globals)
	} else {
		err = m.PrintFormat(out, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	return out.AppendCString("\n")
}

// verboseModel is IF_VERBOSE(minimum, model_smt2_pp(verbose_stream(), m, *md, 0)).
func verboseModel(minimum uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) error {
	out, enabled, err := bmcVerboseStream(minimum)
	if err != nil || !enabled {
		return err
	}
	return md.WriteSMT2(bmcVerboseWriter{out}, services.Context, services.Globals, 0)
}

type bmcVerboseWriter struct{ stream *utext.NumericTextStream }

func (w bmcVerboseWriter) Write(body []byte) (int, error) {
	if err := w.stream.AppendCString(string(body)); err != nil {
		return 0, err
	}
	return len(body), nil
}

func (c *Compiler) verboseRuleFormula(rule *base.Rule) (result error) {
	if _, enabled, err := bmcVerboseStream(1); err != nil || !enabled {
		return err
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&formula)) }()
	handle, err := c.manager.RefGet(formula)
	if err != nil {
		return err
	}
	return verboseExpression(c.manager, 1, handle)
}
