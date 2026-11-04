package tactic

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/text"
)

// writeSubpavingSMT2Expression uses the AST manager's SMT-LIB2 printer for
// mapped terms that are not plain variables, numerals, or uninterpreted apps.
func writeSubpavingSMT2Expression(out io.Writer, manager *ast.ASTManager, expression ast.ASTHandle) (result error) {
	format, err := manager.SMT2PPExprTree(expression, nil)
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
	stream, err := text.NewNumericTextStream(text.NumericTextStreamConfig{Budget: manager.Budget(), BorrowedWriter: out})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	return manager.PrintFormat(stream, format, nil, nil, nil)
}
