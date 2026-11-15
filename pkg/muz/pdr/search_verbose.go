package pdr

// Source: pdr_context.cpp and pdr_generalizers.cpp IF_VERBOSE sites of the
// search (solve_impl, check_reachability, expand_node, create_children,
// backtrack_level) and the core generalizers.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// verboseText is IF_VERBOSE(minimum, verbose_stream() << text).
func verboseText(minimum uint32, format string, args ...any) error {
	out, enabled, err := pdrVerboseStream(minimum)
	if err != nil || !enabled {
		return err
	}
	return out.stream.AppendCString(fmt.Sprintf(format, args...))
}

// verbosePP is IF_VERBOSE(minimum, verbose_stream() << prefix <<
// mk_pp(expression, m) << suffix).
func verbosePP(m *ast.ASTManager, minimum uint32, prefix string, expression ast.ASTHandle, suffix string) error {
	out, enabled, err := pdrVerboseStream(minimum)
	if err != nil || !enabled {
		return err
	}
	if err = out.stream.AppendCString(prefix); err != nil {
		return err
	}
	if err = writeVerbosePP(m, out, expression); err != nil {
		return err
	}
	return out.stream.AppendCString(suffix)
}

// writeVerbosePP writes mk_pp(expression, m) to the verbose stream.
func writeVerbosePP(m *ast.ASTManager, out pdrVerboseWriter, expression ast.ASTHandle) (result error) {
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
		return m.PrintFormat(out.stream, format, context, &empty, globals)
	}
	return m.PrintFormat(out.stream, format, nil, nil, nil)
}

// verboseConvexGeneralized is core_convex_hull_generalizer::method1's
// IF_VERBOSE(0, state << "Generalized to:" << mk_and(conv2)).
func verboseConvexGeneralized(m *ast.ASTManager, state ast.ASTHandle, core []ast.ASTHandle) (result error) {
	generalized, err := m.MkAndArray(core)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(generalized)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	if err = verbosePP(m, 0, "", state, "\n"); err != nil {
		return err
	}
	return verbosePP(m, 0, "Generalized to:\n", generalized, "\n")
}

// verboseArithmeticInductive is core_arith_inductive_generalizer's
// IF_VERBOSE(1, "(non)inductive", "old", core, "new", new_core).
func verboseArithmeticInductive(m *ast.ASTManager, inductive bool, old, current []ast.ASTHandle) error {
	out, enabled, err := pdrVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	prefix := "non"
	if inductive {
		prefix = ""
	}
	if err = out.stream.AppendCString(prefix + "inductive\nold\n"); err != nil {
		return err
	}
	for _, literal := range old {
		if err = verbosePP(m, 1, "", literal, "\n"); err != nil {
			return err
		}
	}
	if err = out.stream.AppendCString("new\n"); err != nil {
		return err
	}
	for _, literal := range current {
		if err = verbosePP(m, 1, "", literal, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func verboseNoModel(node *ModelNode) error {
	out, enabled, err := pdrVerboseStream(1)
	if err != nil || !enabled {
		return err
	}
	if err = out.stream.AppendCString("no model:\n"); err != nil {
		return err
	}
	if err = node.WriteDisplay(out, 0); err != nil {
		return err
	}
	return verbosePP(node.transformer.manager, 1, "", node.state, "\n")
}
