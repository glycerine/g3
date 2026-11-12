package ast

// Source: src/ast/well_sorted.cpp:63-71.

import (
	"errors"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) wellSortedMismatchWarning(w wellSortedWarning, c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService) (message string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if err = out.AppendCString("Sort mismatch for argument "); err != nil {
		return "", err
	}
	if err = out.AppendUint32(w.argument); err != nil {
		return "", err
	}
	if err = out.AppendCString(" of "); err != nil {
		return "", err
	}
	if err = m.DisplayLowLevel(out, w.app, false, true); err != nil {
		return "", err
	}
	if err = out.AppendCString("\nExpected sort: "); err != nil {
		return "", err
	}
	printSort := func(h ASTHandle) error {
		formatted, e := m.SMT2PPSort(h)
		if e != nil {
			return e
		}
		return m.PrintFormatTransient(out, formatted, c, p, g)
	}
	if err = printSort(w.want); err != nil {
		return "", err
	}
	if err = out.AppendCString("\nActual sort:   "); err != nil {
		return "", err
	}
	if err = printSort(w.actual); err != nil {
		return "", err
	}
	if err = out.AppendCString("\nFunction sort: "); err != nil {
		return "", err
	}
	decl, e := m.SMT2PPDeclareFun(w.decl)
	if e != nil {
		return "", e
	}
	if err = m.PrintFormatTransient(out, decl, c, p, g); err != nil {
		return "", err
	}
	if err = out.AppendCString("."); err != nil {
		return "", err
	}
	return out.String()
}

// SMT2PPSortString is mk_pp(s, m) for a sort with the default parameters.
func (m *ASTManager) SMT2PPSortString(sort ASTHandle) (text string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	formatted, err := m.SMT2PPSort(sort)
	if err != nil {
		return "", err
	}
	if err = m.PrintFormatTransient(out, formatted, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String()
}

// smt2PPDeclString is mk_pp(f, m) for a function declaration.
func (m *ASTManager) smt2PPDeclString(decl ASTHandle) (text string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	formatted, err := m.SMT2PPDeclareFun(decl)
	if err != nil {
		return "", err
	}
	if err = m.PrintFormatTransient(out, formatted, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String()
}
