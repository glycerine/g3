package tab

// Source: src/ast/ast_pp.h mk_pp, the SMT2 pretty printer (mk_ismt2_pp)
// used by tb::clause::display, select_predicate and display_certificate.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// writeMkPP writes mk_pp(h, m) to out (without a trailing newline).
func writeMkPP(out io.Writer, m *ast.ASTManager, h ast.ASTHandle) (result error) {
	context, globals, err := m.RewriterParameterServices()
	if err != nil {
		return err
	}
	var format ast.ASTHandle
	if context != nil && globals != nil {
		format, err = m.SMT2PPExprTreeWithParams(h, nil, context, globals, nil)
	} else {
		format, err = m.SMT2PPExprTree(h, nil)
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
	defer func() {
		if e := formats.DelRef(&ref); e != nil {
			result = errors.Join(result, e)
		}
	}()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() {
		if e := stream.Close(); e != nil {
			result = errors.Join(result, e)
		}
	}()
	if context != nil && globals != nil {
		empty := context.EmptyValue()
		err = m.PrintFormat(stream, format, context, &empty, globals)
	} else {
		err = m.PrintFormat(stream, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
