package smtcore

// Source: src/smt/arith_eq_adapter.cpp mk_axioms, used when the adapter
// belongs to SMTTheoryArith. The adapter itself creates t1 - t2, the atoms
// t1 - t2 <= 0 and t1 - t2 >= 0, and the three triangle axioms; the
// replay theory instead derives them from the internalized equality.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (a *smtArithmeticEqualityAdapter) mkAxiomsSource(n1, n2 *SMTEnode) (result error) {
	state, err := a.owner.State()
	if err != nil {
		return err
	}
	m := state.manager
	if n1 == n2 {
		return ast.ErrASTDomain
	}
	id1, err := n1.OwnerID()
	if err != nil {
		return err
	}
	id2, err := n2.OwnerID()
	if err != nil {
		return err
	}
	tags := a.traceTags()
	exprs := func(out io.Writer, hs ...ast.ASTHandle) error {
		for _, h := range hs {
			if err := smtArithEqDisplayExpression(m, out, h); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		return nil
	}
	always := func() bool { return true }
	if err = smtArithEqTrace(tags, "arith_eq_adapter_mk_axioms", "mk_axioms", 86, always, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, fmt.Sprintf("#%d #%d\n", id1, id2)); err != nil {
			return err
		}
		return exprs(out, n1.owner, n2.owner)
	}); err != nil {
		return err
	}
	if id1 > id2 {
		n1, n2 = n2, n1
		id1, id2 = id2, id1
	}
	t1, t2 := n1.owner, n2.owner
	v1, err := m.IsValue(t1)
	if err != nil {
		return err
	}
	v2, err := m.IsValue(t2)
	if err != nil {
		return err
	}
	if v1 && v2 {
		return nil // no axioms are needed for 2 = 3
	}
	if t1 == t2 {
		return nil
	}
	relevant := func(n *SMTEnode) bool { return state.relevancy == nil || state.relevancy.IsRelevant(n.owner) }
	if err = smtArithEqTrace(tags, "arith_eq_adapter_relevancy", "mk_axioms", 102, func() bool {
		return !(relevant(n1) && relevant(n2))
	}, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, fmt.Sprintf("is_relevant(n1): #%d %d\nis_relevant(n2): #%d %d\n", id1, smtDisplayBool(relevant(n1)), id2, smtDisplayBool(relevant(n2)))); err != nil {
			return err
		}
		if err := exprs(out, n1.owner, n2.owner); err != nil {
			return err
		}
		return state.DisplayContext(out)
	}); err != nil {
		return err
	}
	pair := smtArithmeticEqualityPair{first: t1, second: t2}
	if _, found := a.processed[pair]; found {
		return nil
	}
	if err = smtArithEqTrace(tags, "arith_eq_adapter_profile", "mk_axioms", 116, always, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("mk #%d #%d %d %d\n", id1, id2, len(a.processed), state.scopeLevel))
	}); err != nil {
		return err
	}
	a.numAxioms++
	if err = smtArithEqTrace(tags, "arith_eq_adapter_profile_detail", "mk_axioms", 121, always, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "mk_detail "); err != nil {
			return err
		}
		if err := smtArithEqDisplayBounded(m, out, t1, 5); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err := smtArithEqDisplayBounded(m, out, t2, 5); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	x := smtArithExprs{m: m}
	defer func() { result = errors.Join(result, x.release()) }()
	t1EqT2, err := x.hold(state.mkEqualityAtom(t1, t2)) // ctx.mk_eq_atom: theory_arith::mk_eq_atom is arith_util::mk_eq
	if err != nil {
		return err
	}
	if err = smtArithEqTrace(tags, "arith_eq_adapter_bug", "mk_axioms", 130, always, func(out io.Writer) error {
		for _, h := range []ast.ASTHandle{t1EqT2, t1, t2} {
			if err := smtArithEqDisplayBounded(m, out, h, ^uint32(0)); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	family := a.owner.FamilyID()
	mkApp := func(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
		return x.hold(m.MkFamilyApp(family, op, nil, args))
	}
	isNumeral := func(h ast.ASTHandle) bool {
		_, _, matched, err := m.ArithmeticNumeralText(h)
		return err == nil && matched
	}
	var le, ge ast.ASTHandle
	if isNumeral(t1) {
		t1, t2 = t2, t1
	}
	if isNumeral(t2) {
		if le, err = mkApp(smtArithOpLE, t1, t2); err != nil {
			return err
		}
		if ge, err = mkApp(smtArithOpGE, t1, t2); err != nil {
			return err
		}
	} else {
		sort, err := m.GetSort(t1)
		if err != nil {
			return err
		}
		kind, err := m.GetDeclKind(sort)
		if err != nil {
			return err
		}
		numeral := func(r *big.Rat) (ast.ASTHandle, error) {
			if kind == smtArithIntSort {
				return x.hold(m.ArithmeticIntegerNumeralText(r.RatString()))
			}
			return x.hold(m.ArithmeticRealNumeralText(r.RatString()))
		}
		minusOne, err := numeral(big.NewRat(-1, 1))
		if err != nil {
			return err
		}
		zero, err := numeral(new(big.Rat))
		if err != nil {
			return err
		}
		t3, err := mkApp(smtArithOpMul, minusOne, t2)
		if err != nil {
			return err
		}
		s, err := mkApp(smtArithOpAdd, t1, t3)
		if err != nil {
			return err
		}
		if le, err = mkApp(smtArithOpLE, s, zero); err != nil {
			return err
		}
		if ge, err = mkApp(smtArithOpGE, s, zero); err != nil {
			return err
		}
	}
	if err = smtArithEqTrace(tags, "arith_eq_adapter_perf", "mk_axioms", 174, always, func(out io.Writer) error {
		return exprs(out, t1EqT2, le, ge)
	}); err != nil {
		return err
	}
	// already_processed_trail
	if err = a.markProcessed(m, pair); err != nil {
		return err
	}
	a.trail = append(a.trail, pair)
	if err = smtArithEqTrace(tags, "arith_eq_adapter_profile", "mk_axioms", 179, always, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("insert #%d #%d\n", id1, id2))
	}); err != nil {
		return err
	}
	if err = state.Internalize(t1EqT2, true); err != nil {
		return err
	}
	bv, found := state.BoolVar(t1EqT2)
	if !found {
		return ast.ErrASTDomain
	}
	eqLit := NewSMTLiteral(bv, false)
	if err = smtArithEqTrace(tags, "interface_eq", "mk_axioms", 182, always, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "core should try true phase first for the equality: "); err != nil {
			return err
		}
		if err := smtArithEqDisplayLiteral(out, eqLit); err != nil {
			return err
		}
		return smtWriteTheoryText(out, fmt.Sprintf("\n#%d == #%d\ntry_true_first: %d\n", id1, id2, smtDisplayBool(state.booleanData[bv].TryTrueFirst())))
	}); err != nil {
		return err
	}
	if err = smtArithEqTrace(tags, "arith_eq_adapter_bug", "mk_axioms", 186, always, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "le: "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, le); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, "\nge: "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, ge); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	if err = state.Internalize(le, true); err != nil {
		return err
	}
	if err = state.Internalize(ge, true); err != nil {
		return err
	}
	leLit, err := state.Literal(le)
	if err != nil {
		return err
	}
	geLit, err := state.Literal(ge)
	if err != nil {
		return err
	}
	data, err := state.BooleanData(eqLit.Var())
	if err != nil {
		return err
	}
	if data.TryTrueFirst() {
		// Propagate the try_true_first flag to le and ge, otherwise model
		// based theory combination is ineffective.
		if _, err = state.setScopedTrueFirst(leLit.Var()); err != nil {
			return err
		}
		if _, err = state.setScopedTrueFirst(geLit.Var()); err != nil {
			return err
		}
	}
	var hint []ast.ASTParameter
	if proofs, err := m.ProofsEnabled(); err != nil {
		return err
	} else if proofs {
		sym, err := m.Symbols.Intern("triangle-eq")
		if err != nil {
			return err
		}
		p, err := m.NewParameterSymbol(sym)
		if err != nil {
			return err
		}
		hint = []ast.ASTParameter{p}
		defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, hint)) }()
	}
	for _, clause := range [][]SMTLiteral{
		{eqLit.Negated(), leLit},
		{eqLit.Negated(), geLit},
		{eqLit, leLit.Negated(), geLit.Negated()},
	} {
		if err = state.addTheoryAxiomWithParameters(family, clause, hint); err != nil {
			return err
		}
	}
	if err = smtArithEqTrace(tags, "arith_eq_adapter", "mk_axioms", 208, always, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "internalizing: "); err != nil {
			return err
		}
		for i, h := range []ast.ASTHandle{le, ge, t1EqT2} {
			if err := smtWriteTheoryText(out, " "); err != nil {
				return err
			}
			if err := smtArithEqDisplayExpression(m, out, h); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, ": "); err != nil {
				return err
			}
			if err := smtArithEqDisplayLiteral(out, []SMTLiteral{leLit, geLit, eqLit}[i]); err != nil {
				return err
			}
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	if a.binary {
		if err = smtArithEqTrace(tags, "arith_eq_adapter", "mk_axioms", 214, always, func(out io.Writer) error {
			return smtWriteTheoryText(out, "adding binary bounds...\n")
		}); err != nil {
			return err
		}
		if err = state.addTheoryAxiomWithParameters(family, []SMTLiteral{leLit, geLit}, hint); err != nil {
			return err
		}
	}
	if state.relevancyLevel > 0 { // ctx.relevancy()
		// arith_eq_relevancy_eh: when n1 and n2 are relevant, so are the
		// equality and both bounds.
		if err = a.AddRelevancy(n1.owner, n2.owner, t1EqT2, le, ge); err != nil {
			return err
		}
	}
	if !a.lazy && state.searching && state.scopeLevel > state.searchBaseLevel &&
		n1.InternalScopeLevel() <= state.searchBaseLevel && n2.InternalScopeLevel() <= state.searchBaseLevel {
		a.restart = append(a.restart, pair)
	}
	return smtArithEqTraceContext(tags, "arith_eq_adapter_detail", "mk_axioms", 226, state)
}
