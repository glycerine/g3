package duality

// Source: src/duality/duality.h iZ3LogicSolver::assert_axiom.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// InterpolationAxiom is the assert_axiom legacy-solver workaround: a
// top-level distinct with more than ten arguments becomes the conjunction
// f(a_i) = i over one fresh @distinct function. The source converts once,
// when the axiom is asserted, and that converted term is both asserted in
// the interpolating solver and kept as its theory. Other axioms are
// returned unchanged; changed reports a new Expr the caller owns.
func InterpolationAxiom(context *Context, axiom *Expr) (result *Expr, changed bool, resultErr error) {
	if err := axiom.open(); err != nil {
		return nil, false, err
	}
	if axiom.value.object.context != context {
		return nil, false, ErrContextMismatch
	}
	m := context.manager
	h, err := axiom.Raw()
	if err != nil {
		return nil, false, err
	}
	kind, err := m.GetKind(h)
	if err != nil {
		return nil, false, err
	}
	if kind != ast.ASTApp {
		return axiom, false, nil
	}
	family, err := m.GetFamilyID(h)
	if err != nil {
		return nil, false, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return nil, false, err
	}
	if family != m.BasicFamilyID() || op != ast.ASTOpDistinct {
		return axiom, false, nil
	}
	count, err := m.GetNumArgs(h)
	if err != nil {
		return nil, false, err
	}
	if count <= 10 {
		return axiom, false, nil
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	pin := func(value ast.ASTHandle) error {
		ref, e := m.NewRef(value)
		if e == nil {
			refs = append(refs, ref)
		}
		return e
	}
	first, err := m.GetArg(h, 0)
	if err != nil {
		return nil, false, err
	}
	argumentSort, err := m.GetSort(first)
	if err != nil {
		return nil, false, err
	}
	// ctx->int_sort(): no numeral is built before the fresh declaration.
	integerSort, err := m.MkFamilySort(context.families.Arithmetic, 1, nil)
	if err != nil {
		return nil, false, err
	}
	prefix, err := m.Symbols.Intern("@distinct")
	if err != nil {
		return nil, false, err
	}
	declaration, err := m.MkFreshFuncDecl(prefix, usymbol.Symbol{}, []ast.ASTHandle{argumentSort}, integerSort)
	if err != nil {
		return nil, false, err
	}
	if err = pin(declaration); err != nil {
		return nil, false, err
	}
	constraints := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		arg, e := m.GetArg(h, i)
		if e != nil {
			return nil, false, e
		}
		// f(a) == ctx->int_val(i): GCC builds int_val(i) before f(a).
		index, e := m.ArithmeticIntegerNumeralText(strconv.Itoa(int(i)))
		if e != nil {
			return nil, false, e
		}
		if e = pin(index); e != nil {
			return nil, false, e
		}
		image, e := m.MkApp(declaration, []ast.ASTHandle{arg})
		if e != nil {
			return nil, false, e
		}
		if e = pin(image); e != nil {
			return nil, false, e
		}
		constraints[i], e = m.MkEq(image, index)
		if e != nil {
			return nil, false, e
		}
		if e = pin(constraints[i]); e != nil {
			return nil, false, e
		}
	}
	conjunction, err := m.MkAndArray(constraints)
	if err != nil {
		return nil, false, err
	}
	result, err = NewExpr(context, conjunction)
	if err != nil {
		return nil, false, err
	}
	return result, true, nil
}
