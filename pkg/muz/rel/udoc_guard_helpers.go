package rel

// Source: src/muz/rel/udoc_relation.cpp (mk_and of the guard's
// remainders; apply_guard's failure_case).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func udocMakeAnd(manager *ast.ASTManager, terms []ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(terms) {
	case 0:
		return manager.MkTrue()
	case 1:
		return terms[0], nil
	default:
		return manager.MkAndArgs(terms)
	}
}

// udocGuardNotHandled is udoc_relation::apply_guard's failure_case:
// throw default_exception("Guard expression is not handled" << mk_pp(g, m)).
func udocGuardNotHandled(m *ast.ASTManager, guard ast.ASTHandle) (result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if err = out.AppendCString("Guard expression is not handled"); err != nil {
		return err
	}
	formatted, err := m.SMT2PPExprTree(guard, nil)
	if err != nil {
		return err
	}
	if err = m.PrintFormatTransient(out, formatted, nil, nil, nil); err != nil {
		return err
	}
	text, err := out.String()
	if err != nil {
		return err
	}
	return z3exception.NewDefaultException(text)
}
