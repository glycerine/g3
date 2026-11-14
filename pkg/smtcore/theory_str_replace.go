package smtcore

// Source: src/smt/theory_str.cpp mk_contains and instantiate_axiom_Replace.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) stringContainsTerm(haystack, needle ast.ASTHandle) (ast.ASTHandle, error) {
	expression, err := t.state.manager.MkSequenceContains(haystack, needle)
	if err != nil {
		return expression, err
	}
	if err := t.retain(expression); err != nil {
		return expression, err
	}
	if err := t.state.Internalize(expression, false); err != nil {
		return expression, err
	}
	return expression, t.setUpAxioms(expression)
}

func (t *SMTStringTheory) instantiateReplace(node *SMTEnode) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	word, err := m.GetArg(expression, 0)
	if err != nil {
		return err
	}
	needle, err := m.GetArg(expression, 1)
	if err != nil {
		return err
	}
	replacement, err := m.GetArg(expression, 2)
	if err != nil {
		return err
	}
	prefix, err := t.freshStringVariable("x1")
	if err != nil {
		return err
	}
	tail, err := t.freshStringVariable("x2")
	if err != nil {
		return err
	}
	index, err := t.freshIntegerVariable("i1")
	if err != nil {
		return err
	}
	value, err := t.freshStringVariable("result")
	if err != nil {
		return err
	}
	// Source owners in declaration order: condAst, thenItems, tmpLen,
	// elseBranch, rw, breakdownAssert, breakdownAssert_rw, reduceToResult,
	// reduceToResult_rw. Call arguments are evaluated right to left (GCC).
	scope := smtStringScope{theory: t}
	defer func() { result = errors.Join(result, scope.close()) }()
	condition, err := scope.own(t.stringContainsTerm(word, needle))
	if err != nil {
		return err
	}
	items := scope.vector()
	rest, err := t.concatTerm(needle, tail)
	if err != nil {
		return err
	}
	whole, err := t.concatTerm(prefix, rest)
	if err != nil {
		return err
	}
	if _, err := items.push(t.state.MkEqAtom(word, whole)); err != nil {
		return err
	}
	prefixLength, err := m.MkSequenceLength(prefix)
	if err != nil {
		return err
	}
	if _, err := items.push(t.state.MkEqAtom(index, prefixLength)); err != nil {
		return err
	}
	beforeMatch, err := t.freshStringVariable("x3")
	if err != nil {
		return err
	}
	fromMatch, err := t.freshStringVariable("x4")
	if err != nil {
		return err
	}
	// m_autil.mk_add(i1, mk_strlen(arg1), mk_int(-1)): mk_int(-1) first.
	minusOne, err := m.ArithmeticIntegerNumeralText("-1")
	if err != nil {
		return err
	}
	needleLength, err := m.MkSequenceLength(needle)
	if err != nil {
		return err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	beforeLength, err := scope.own(m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{index, needleLength, minusOne}))
	if err != nil {
		return err
	}
	whole, err = t.concatTerm(beforeMatch, fromMatch)
	if err != nil {
		return err
	}
	if _, err := items.push(t.state.MkEqAtom(word, whole)); err != nil {
		return err
	}
	length, err := m.MkSequenceLength(beforeMatch)
	if err != nil {
		return err
	}
	if _, err := items.push(t.state.MkEqAtom(length, beforeLength)); err != nil {
		return err
	}
	earlierMatch, err := t.stringContainsTerm(beforeMatch, needle)
	if err != nil {
		return err
	}
	if _, err := items.push(m.MkNot(earlierMatch)); err != nil {
		return err
	}
	rest, err = t.concatTerm(replacement, tail)
	if err != nil {
		return err
	}
	newWord, err := t.concatTerm(prefix, rest)
	if err != nil {
		return err
	}
	if _, err := items.push(t.state.MkEqAtom(value, newWord)); err != nil {
		return err
	}
	elseBranch, err := scope.own(t.state.MkEqAtom(value, word))
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
	thenBranch, err := m.MkAndArray(items.values)
	if err != nil {
		return err
	}
	breakdown, err := scope.own(m.MkIte(condition, thenBranch, elseBranch))
	if err != nil {
		return err
	}
	if err := scope.rewriteOwned(breakdown, rw); err != nil {
		return err
	}
	reduction, err := scope.own(t.state.MkEqAtom(expression, value))
	if err != nil {
		return err
	}
	return scope.rewriteOwned(reduction, rw)
}
