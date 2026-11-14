package smtcore

// Source: src/smt/theory_str.cpp instantiate_axiom_CharAt and
// instantiate_axiom_Substr. These use context/default rewriters respectively.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (t *SMTStringTheory) rewriteStringAxiom(expression ast.ASTHandle, rw *rewriter.TheoryRewriter) (result error) {
	if rw == nil {
		return ast.ErrASTHandle // A native SMT context always owns a rewriter.
	}
	m := t.state.manager
	root, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&root)) }()
	if err := rw.Rewrite(expression, root); err != nil {
		return err
	}
	value, err := m.RefGet(root)
	if err != nil {
		return err
	}
	return t.assertAxiom(value)
}

func (t *SMTStringTheory) instantiateCharAt(node *SMTEnode) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	word, index, matched, err := m.SequenceBinaryOperationArguments(expression, 8)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	// Source owners, in declaration order: cond, and_item (a vector),
	// thenBranch, elseBranch, axiom, reductionVar, finalAxiom. Scope exit
	// releases them in reverse; the vector releases its items forward.
	// Argument temporaries (mk_ge, mk_lt, mk_strlen, mk_int) are unowned.
	var owned [][]ast.ASTRef
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			for j := range owned[i] {
				result = errors.Join(result, m.DelRef(&owned[i][j]))
			}
		}
	}()
	own := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		ref, err := m.NewRef(value)
		if err != nil {
			return value, err
		}
		owned = append(owned, []ast.ASTRef{ref})
		return value, nil
	}
	variables := [3]ast.ASTHandle{}
	for i, name := range []string{"ts0", "ts1", "ts2"} {
		variables[i], err = t.freshStringVariable(name)
		if err != nil {
			return err
		}
	}
	// GCC evaluates mk_and's arguments right to left: mk_lt (with
	// mk_strlen(arg0)) before mk_ge (with mk_int(0)).
	length, err := m.MkSequenceLength(word)
	if err != nil {
		return err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	belowLength, err := m.MkFamilyApp(family, 4, nil, []ast.ASTHandle{index, length})
	if err != nil {
		return err
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return err
	}
	nonnegative, err := m.MkArithmeticGE(index, zero)
	if err != nil {
		return err
	}
	condition, err := own(m.MkAnd(nonnegative, belowLength))
	if err != nil {
		return err
	}
	itemsAt := len(owned)
	owned = append(owned, nil)
	item := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		ref, err := m.NewRef(value)
		if err != nil {
			return value, err
		}
		owned[itemsAt] = append(owned[itemsAt], ref)
		return value, nil
	}
	tail, err := t.concatTerm(variables[1], variables[2])
	if err != nil {
		return err
	}
	whole, err := t.concatTerm(variables[0], tail)
	if err != nil {
		return err
	}
	wordEquality, err := item(t.state.MkEqAtom(word, whole))
	if err != nil {
		return err
	}
	prefixLength, err := m.MkSequenceLength(variables[0])
	if err != nil {
		return err
	}
	indexEquality, err := item(t.state.MkEqAtom(index, prefixLength))
	if err != nil {
		return err
	}
	// ctx.mk_eq_atom(mk_strlen(ts1), mk_int(1)): mk_int(1) first.
	one, err := m.ArithmeticIntegerNumeralText("1")
	if err != nil {
		return err
	}
	charLength, err := m.MkSequenceLength(variables[1])
	if err != nil {
		return err
	}
	charEquality, err := item(t.state.MkEqAtom(charLength, one))
	if err != nil {
		return err
	}
	thenBranch, err := own(m.MkAndArray([]ast.ASTHandle{wordEquality, indexEquality, charEquality}))
	if err != nil {
		return err
	}
	empty, err := t.stringLiteral("")
	if err != nil {
		return err
	}
	elseBranch, err := own(t.state.MkEqAtom(variables[1], empty))
	if err != nil {
		return err
	}
	choice, err := own(m.MkIte(condition, thenBranch, elseBranch))
	if err != nil {
		return err
	}
	reduction, err := own(t.state.MkEqAtom(expression, variables[1]))
	if err != nil {
		return err
	}
	// finalAxiom is owned only by the reference the rewriter overwrites.
	final, err := m.MkAnd(choice, reduction)
	if err != nil {
		return err
	}
	return t.rewriteStringAxiom(final, t.state.theoryRewriter)
}

func (t *SMTStringTheory) instantiateSubstring(node *SMTEnode) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	op, own, err := t.operation(expression)
	if err != nil || !own || op != 6 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	word, err := m.GetArg(expression, 0)
	if err != nil {
		return err
	}
	position, err := m.GetArg(expression, 1)
	if err != nil {
		return err
	}
	count, err := m.GetArg(expression, 2)
	if err != nil {
		return err
	}
	// Source owners in declaration order: zero, minusOne,
	// argumentsValid_terms, lenOutOfBounds, argumentsValid, case1_premise,
	// case1_conclusion, case1, case2_conclusion, case2,
	// case3_conclusion_terms, case3_conclusion, case3, and the inner block
	// rw, case1_rw, case2_rw, case3_rw. Argument temporaries are unowned and
	// call arguments are evaluated right to left (GCC). The expr_refs t0..t4
	// are not modeled: mk_str_var keeps them on the theory trail.
	scope := smtStringScope{theory: t}
	defer func() { result = errors.Join(result, scope.close()) }()
	zero, err := scope.own(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return err
	}
	minusOne, err := scope.own(m.ArithmeticIntegerNumeralText("-1"))
	if err != nil {
		return err
	}
	validTerms := scope.vector()
	if _, err := validTerms.push(m.MkArithmeticGE(position, zero)); err != nil {
		return err
	}
	length, err := m.MkSequenceLength(word)
	if err != nil {
		return err
	}
	negativeLength, err := m.MkArithmeticMul(minusOne, length)
	if err != nil {
		return err
	}
	difference, err := m.MkArithmeticAdd(position, negativeLength)
	if err != nil {
		return err
	}
	outside, err := m.MkArithmeticGE(difference, zero)
	if err != nil {
		return err
	}
	if _, err := validTerms.push(m.MkNot(outside)); err != nil {
		return err
	}
	if _, err := validTerms.push(m.MkArithmeticGE(count, zero)); err != nil {
		return err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	length, err = m.MkSequenceLength(word)
	if err != nil {
		return err
	}
	negativeLength, err = m.MkArithmeticMul(minusOne, length)
	if err != nil {
		return err
	}
	endDifference, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{position, count, negativeLength})
	if err != nil {
		return err
	}
	endOutside, err := scope.own(m.MkArithmeticGE(endDifference, zero))
	if err != nil {
		return err
	}
	valid, err := scope.own(m.MkAndArray(validTerms.values))
	if err != nil {
		return err
	}
	invalid, err := scope.own(m.MkNot(valid))
	if err != nil {
		return err
	}
	empty, err := t.stringLiteral("")
	if err != nil {
		return err
	}
	emptyResult, err := scope.own(t.state.MkEqAtom(expression, empty))
	if err != nil {
		return err
	}
	case1, err := scope.own(m.MkImplies(invalid, emptyResult))
	if err != nil {
		return err
	}
	variables := [5]ast.ASTHandle{}
	for i, name := range []string{"t0", "t1"} {
		variables[i], err = t.freshStringVariable(name)
		if err != nil {
			return err
		}
	}
	// m.mk_and(eq(base, concat(t0,t1)), eq(strlen(t0), pos), eq(expr, t1)):
	// GCC evaluates the three arguments right to left.
	wideResult, err := t.state.MkEqAtom(expression, variables[1])
	if err != nil {
		return err
	}
	prefixLength, err := m.MkSequenceLength(variables[0])
	if err != nil {
		return err
	}
	widePrefix, err := t.state.MkEqAtom(prefixLength, position)
	if err != nil {
		return err
	}
	whole, err := t.concatTerm(variables[0], variables[1])
	if err != nil {
		return err
	}
	wideWord, err := t.state.MkEqAtom(word, whole)
	if err != nil {
		return err
	}
	wideConclusion, err := scope.own(m.MkAndArray([]ast.ASTHandle{wideWord, widePrefix, wideResult}))
	if err != nil {
		return err
	}
	wideGuard, err := m.MkAnd(valid, endOutside)
	if err != nil {
		return err
	}
	case2, err := scope.own(m.MkImplies(wideGuard, wideConclusion))
	if err != nil {
		return err
	}
	for i, name := range []string{"t2", "t3", "t4"} {
		variables[i+2], err = t.freshStringVariable(name)
		if err != nil {
			return err
		}
	}
	narrow := scope.vector()
	tail, err := t.concatTerm(variables[3], variables[4])
	if err != nil {
		return err
	}
	whole, err = t.concatTerm(variables[2], tail)
	if err != nil {
		return err
	}
	if _, err := narrow.push(t.state.MkEqAtom(word, whole)); err != nil {
		return err
	}
	prefixLength, err = m.MkSequenceLength(variables[2])
	if err != nil {
		return err
	}
	if _, err := narrow.push(t.state.MkEqAtom(prefixLength, position)); err != nil {
		return err
	}
	resultLength, err := m.MkSequenceLength(variables[3])
	if err != nil {
		return err
	}
	if _, err := narrow.push(t.state.MkEqAtom(resultLength, count)); err != nil {
		return err
	}
	if _, err := narrow.push(t.state.MkEqAtom(expression, variables[3])); err != nil {
		return err
	}
	narrowConclusion, err := scope.own(m.MkAndArray(narrow.values))
	if err != nil {
		return err
	}
	endInside, err := m.MkNot(endOutside)
	if err != nil {
		return err
	}
	narrowGuard, err := m.MkAnd(valid, endInside)
	if err != nil {
		return err
	}
	case3, err := scope.own(m.MkImplies(narrowGuard, narrowConclusion))
	if err != nil {
		return err
	}
	if t.state.theoryRewriter == nil {
		return ast.ErrASTHandle
	}
	rw, err := t.state.theoryRewriter.FreshDefault()
	if rw != nil {
		scope.hook(rw.Close)
	}
	if err != nil {
		return err
	}
	for _, axiom := range []ast.ASTHandle{case1, case2, case3} {
		if err := scope.rewriteOwned(axiom, rw); err != nil {
			return err
		}
	}
	return nil
}
