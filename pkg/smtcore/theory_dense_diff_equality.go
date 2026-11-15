package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h:222-245 (internalize_eq_eh),
// 280-293 (new_eq_eh, new_diseq_eh), restart_eh, and the adapter's
// src/smt/arith_eq_adapter.cpp mk_axioms.

import "github.com/glycerine/g3/pkg/ast"

// InternalizeEquality is theory_dense_diff_logic::internalize_eq_eh.
func (t *SMTDenseDiffTheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	lhs, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	rhs, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	op, arithmetic, err := t.syntax.operation(lhs)
	if err != nil {
		return err
	}
	if arithmetic && op == smtArithOpAdd {
		count, err := state.manager.GetNumArgs(lhs)
		if err != nil {
			return err
		}
		if count == 2 {
			second, err := state.manager.GetArg(lhs, 1)
			if err != nil {
				return err
			}
			_, negative, err := t.denseTimesMinusOne(second)
			if err != nil {
				return err
			}
			_, isNumeral, err := t.syntax.numeral(rhs)
			if err != nil {
				return err
			}
			if negative && isNumeral {
				// force axioms for (= (+ x (* -1 y)) k)
				// this is necessary because (+ x (* -1 y)) is not a diff logic term.
				return t.adapter.mkAxiomsSource(state.context.enodes.get(lhs), state.context.enodes.get(rhs))
			}
		}
	}
	if t.config.EagerEqAxioms {
		n1 := state.context.enodes.get(lhs)
		n2 := state.context.enodes.get(rhs)
		if n1 != nil && n2 != nil && n1 != n2 &&
			n1.TheoryVar(t.FamilyID()) != SMTNullTheoryVar && n2.TheoryVar(t.FamilyID()) != SMTNullTheoryVar {
			return t.adapter.mkAxiomsSource(n1, n2)
		}
	}
	return nil
}

// NewEquality is new_eq_eh: m_arith_eq_adapter.new_eq_eh(v1, v2).
func (t *SMTDenseDiffTheory) NewEquality(left, right SMTTheoryVar) error {
	return t.adapter.NewEquality(left, right)
}

// NewDisequality is new_diseq_eh: m_arith_eq_adapter.new_diseq_eh(v1, v2).
func (t *SMTDenseDiffTheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.adapter.NewDisequality(left, right)
}

// Restart is restart_eh: m_arith_eq_adapter.restart_eh(), which calls
// mk_axioms on the stored enodes; the bound atoms reattach any theory
// variable lost with a popped scope.
func (t *SMTDenseDiffTheory) Restart() error {
	return t.adapter.Restart()
}
