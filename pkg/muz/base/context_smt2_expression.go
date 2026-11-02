package base

// Source: dl_context.cpp display_smt2's PP macro selects mk_smt_pp versus
// ast_smt2_pp; format and stream owners are scoped to each insertion.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *RuleSet) writeContextExpression(out io.Writer, printer *ast.LegacySMTPrinter, h ast.ASTHandle, low bool) (resultErr error) {
	if low {
		return printer.DisplayExpr(out, h, nil)
	}
	m := s.manager.manager
	c, g, err := m.RewriterParameterServices()
	if err != nil {
		return err
	}
	var format ast.ASTHandle
	if c != nil && g != nil {
		format, err = m.SMT2PPExprTreeWithParams(h, nil, c, g, nil)
	} else {
		format, err = m.SMT2PPExprTree(h, nil)
	}
	if err != nil {
		return err
	}
	return s.writeContextFormat(out, format)
}

func (s *RuleSet) writeContextSort(out io.Writer, sort ast.ASTHandle) error {
	format, err := s.manager.manager.SMT2PPSort(sort)
	if err != nil {
		return err
	}
	return s.writeContextFormat(out, format)
}

func (s *RuleSet) writeContextFormat(out io.Writer, format ast.ASTHandle) (resultErr error) {
	m := s.manager.manager
	formats, err := m.GetFormatManager()
	if err != nil {
		return err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, stream.Close()) }()
	c, g, err := m.RewriterParameterServices()
	if err != nil {
		return err
	}
	if c != nil && g != nil {
		empty := c.EmptyValue()
		err = m.PrintFormat(stream, format, c, &empty, g)
	} else {
		err = m.PrintFormat(stream, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}
