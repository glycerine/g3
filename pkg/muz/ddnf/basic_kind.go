package ddnf

// Source: src/muz/ddnf/ddnf.cpp process_todo, process_atomic and
// compile_expr test m.is_and/is_or/is_iff/is_not/is_implies/is_eq, which
// check the basic family before the decl kind.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// isBasicApp reports whether expression is an application of a basic-family
// declaration whose kind is one of kinds.
func isBasicApp(manager *ast.ASTManager, expression ast.ASTHandle, kinds ...int32) (bool, error) {
	family, err := manager.GetFamilyID(expression)
	if err != nil || family != manager.BasicFamilyID() {
		return false, err
	}
	operator, err := manager.GetDeclKind(expression)
	if err != nil {
		return false, err
	}
	for _, kind := range kinds {
		if operator == kind {
			return true, nil
		}
	}
	return false, nil
}

// verboseCouldNotHandle is process_todo's
// IF_VERBOSE(0, verbose_stream() << "Could not handle: " << mk_pp(e, m) << "\n").
func verboseCouldNotHandle(manager *ast.ASTManager, expression ast.ASTHandle) (result error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(0)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString("Could not handle: "); err != nil {
		return err
	}
	context, globals, err := manager.RewriterParameterServices()
	if err != nil {
		return err
	}
	var format ast.ASTHandle
	if context != nil && globals != nil {
		format, err = manager.SMT2PPExprTreeWithParams(expression, nil, context, globals, nil)
	} else {
		format, err = manager.SMT2PPExprTree(expression, nil)
	}
	if err != nil {
		return err
	}
	formats, err := manager.GetFormatManager()
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
		err = manager.PrintFormat(out, format, context, &empty, globals)
	} else {
		err = manager.PrintFormat(out, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	return out.AppendCString("\n")
}
