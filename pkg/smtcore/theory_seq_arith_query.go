package smtcore

// The arithmetic queries of the sequence and string theories. The source
// (theory_seq::lower_bound, upper_bound, get_num_value) asks the arithmetic
// theory for get_lower, get_upper and get_value of a term's enode. The
// replay theory answers through its interval and fixed-assignment helpers;
// SMTTheoryArith answers as the source does.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type smtSeqArithmeticQuery interface {
	// interval returns the known bounds of an arithmetic term.
	interval(term ast.ASTHandle) (*smtOldInterval, bool, error)
	// fixedValue returns the value of a term known to be fixed.
	fixedValue(term ast.ASTHandle) (*big.Rat, bool, error)
}

// smtSeqArithmetic returns the arithmetic query for the context, or false
// when the arithmetic theory offers none.
func smtSeqArithmetic(state *SMTInternalizerState) (smtSeqArithmeticQuery, bool, error) {
	family, err := state.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, false, err
	}
	switch a := state.Theory(family).(type) {
	case *SMTLinearArithmeticTheory:
		return &smtSeqReplayArithmetic{a: a}, true, nil
	case *SMTTheoryArith:
		return a, true, nil
	}
	return nil, false, nil
}

type smtSeqReplayArithmetic struct {
	a         *SMTLinearArithmeticTheory
	intervals *smtPolynomialIntervals
	fixed     map[uint32]smtFixedPolynomialValue
}

func (q *smtSeqReplayArithmetic) interval(term ast.ASTHandle) (*smtOldInterval, bool, error) {
	if q.intervals == nil {
		intervals, err := q.a.linearVariableIntervals()
		if err != nil {
			return nil, false, err
		}
		q.intervals = intervals
	}
	return q.a.polynomialInterval(term, q.intervals)
}

func (q *smtSeqReplayArithmetic) fixedValue(term ast.ASTHandle) (*big.Rat, bool, error) {
	if q.fixed == nil {
		q.fixed = q.a.fixedLinearAssignments()
	}
	value, _, exact, err := q.a.fixedPolynomialValue(term, q.fixed)
	return value, exact, err
}

// interval is get_lower/get_upper of the term's enode: a bound is known only
// when it has no infinitesimal (to_expr).
func (t *SMTTheoryArith) interval(term ast.ASTHandle) (*smtOldInterval, bool, error) {
	n := t.state.context.enodes.get(term)
	if n == nil {
		return nil, false, nil
	}
	v := n.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar {
		return nil, false, nil
	}
	result, err := newSMTOldInterval(&t.depManager)
	if err != nil {
		return nil, false, err
	}
	known := false
	if l := t.lower(v); l != nil && l.base().value.infinitesimal.Sign() == 0 {
		result.lower, result.lowerOpen = smtFiniteExtended(&l.base().value.rational), false
		known = true
	}
	if u := t.upper(v); u != nil && u.base().value.infinitesimal.Sign() == 0 {
		result.upper, result.upperOpen = smtFiniteExtended(&u.base().value.rational), false
		known = true
	}
	return result, known, nil
}

// fixedValue is get_num_value: the integer value of some member of the
// term's equivalence class.
func (t *SMTTheoryArith) fixedValue(term ast.ASTHandle) (*big.Rat, bool, error) {
	n := t.state.context.enodes.get(term)
	if n == nil {
		return nil, false, nil
	}
	next := n
	for {
		if v := next.TheoryVar(t.FamilyID()); v != SMTNullTheoryVar {
			h, ok, err := t.GetTheoryValue(next)
			if err != nil {
				return nil, false, err
			}
			if ok {
				if r, isNum := t.isNumeral(h); isNum && r.IsInt() {
					return r, true, nil
				}
			}
		}
		next = next.next
		if next == n || next == nil {
			return nil, false, nil
		}
	}
}
