package smtcore

// Source: src/smt/theory_arith_aux.h:1035-1146 (theory_opt members:
// add_objective, value, maximize, mk_gt, mk_ge), kept apart from
// smt_theory_arith_aux.go only because they adapt to the Go objective
// interface (smtArithmeticObjectiveTheory).

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtArithValueToInfinitesimal converts an inf_numeral to the simplex
// objective value (a boundary to math/big).
func smtArithValueToInfinitesimal(q *numeral.RationalManager, v *smtArithmeticValue) (simplex.Infinitesimal, error) {
	var infinity, rational, infinitesimal big.Rat
	if err := q.BigRatOf(&v.infinity, &infinity); err != nil {
		return simplex.Infinitesimal{}, err
	}
	if err := q.BigRatOf(&v.rational, &rational); err != nil {
		return simplex.Infinitesimal{}, err
	}
	if err := q.BigRatOf(&v.infinitesimal, &infinitesimal); err != nil {
		return simplex.Infinitesimal{}, err
	}
	return simplex.NewInfiniteInfinitesimal(&infinity, &rational, &infinitesimal), nil
}

// objectiveValue is a finite SMTArithmeticObjectiveValue of v.
func (t *SMTTheoryArith) objectiveValue(v *smtArithmeticValue) (SMTArithmeticObjectiveValue, error) {
	finite, err := smtArithValueToInfinitesimal(t.q, v)
	return SMTArithmeticObjectiveValue{Finite: finite}, err
}

// AddObjective is add_objective: internalize the term and return its
// variable, or null_theory_var when the term is not linear.
func (t *SMTTheoryArith) AddObjective(term ast.ASTHandle) (SMTTheoryVar, error) {
	v, err := t.internalizeTermCore(term)
	if err != nil || v == SMTNullTheoryVar {
		return SMTNullTheoryVar, err
	}
	u, err := NewSMTTheoryOptimizationUtil(t.state.manager)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	linear, err := u.IsLinear(term)
	if err != nil || !linear {
		return SMTNullTheoryVar, err
	}
	return v, nil
}

// ObjectiveValue is value(v): inf_eps_rational(get_value(v)).
func (t *SMTTheoryArith) ObjectiveValue(v SMTTheoryVar) (SMTArithmeticObjectiveValue, error) {
	return t.objectiveValue(t.getValue(v))
}

// MaximizeObjective is maximize(v, blocker, has_shared).
func (t *SMTTheoryArith) MaximizeObjective(v SMTTheoryVar) (SMTArithmeticObjectiveValue, ast.ASTHandle, bool, error) {
	hasShared := false
	if len(t.nlMonomials) != 0 {
		blocker, err := t.mkGt(v)
		if err != nil {
			return SMTArithmeticObjectiveValue{}, blocker, true, err
		}
		value, err := t.objectiveValue(t.getValue(v))
		return value, blocker, true, err
	}
	r, err := t.maxMin(v, true, true, &hasShared)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	if r == smtArithUnbounded {
		blocker, err := t.state.manager.MkFalse()
		return SMTArithmeticObjectiveValue{Unbounded: true}, blocker, false, err
	}
	blocker, err := t.mkGt(v)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, blocker, hasShared, err
	}
	value, err := t.objectiveValue(t.getValue(v))
	return value, blocker, hasShared, err
}

// mkGt creates an atom enforcing v > val, as an arithmetic expression.
func (t *SMTTheoryArith) mkGt(v SMTTheoryVar) (result ast.ASTHandle, err error) {
	m := t.state.manager
	val := t.getValue(v)
	obj := t.var2expr(v)
	r := smtQZero()
	defer smtQDel(t.q, &r)
	if err := t.q.Copy(&r, &val.rational); err != nil {
		return ast.ASTHandle{}, err
	}
	isIntObj, err := t.isIntSort(obj)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	x := smtArithExprs{m: m}
	// The blocker is returned to the caller, which roots it; locals only.
	defer func() { err = errors.Join(err, x.release()) }()
	if isIntObj {
		if r.IsInt() {
			err = t.q.Inc(&r)
		} else {
			err = t.q.Ceil(&r, &r)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		e, err := t.mkNumeralQ(&x, &r, true)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return m.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{obj, e})
	}
	// obj is over the reals.
	e, err := t.mkNumeralQ(&x, &r, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if val.infinitesimal.Sign() < 0 {
		return m.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{obj, e})
	}
	return m.MkFamilyApp(t.FamilyID(), smtArithOpGT, nil, []ast.ASTHandle{obj, e})
}

// MakeObjectiveLowerBound is mk_ge: a fresh Boolean constant named
// "val <= v" whose atom enforces val <= v directly, which also handles
// non-standard numbers.
func (t *SMTTheoryArith) MakeObjectiveLowerBound(fm *tactic.FilterModelConverter, v SMTTheoryVar, value simplex.Infinitesimal) (result ast.ASTHandle, resultErr error) {
	s := t.state
	if fm == nil {
		return result, ast.ErrASTHandle
	}
	// opt_solver::mk_ge passes theory_inf_arith the whole inf_eps,
	// theory_mi_arith val.get_numeral() and theory_i_arith
	// val.get_rational().
	if !t.infExt {
		var numeral simplex.Infinitesimal
		numeral.Main.Set(&value.Main)
		if !t.integerExt {
			numeral.Epsilon.Set(&value.Epsilon)
		}
		value = numeral
	}
	var name bytes.Buffer
	name.WriteString(smtArithmeticBoundName(value))
	name.WriteString(" <= ")
	printer, err := ast.NewSimpleASTPrinterContext(s.manager, &name, &name)
	if err != nil {
		return result, err
	}
	if err = printer.DisplayExpr(&name, t.var2expr(v), 0); err != nil {
		return result, err
	}
	symbol, err := s.manager.Symbols.Intern(name.String())
	if err != nil {
		return result, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return result, err
	}
	if result, err = s.manager.MkConst(symbol, boolean); err != nil {
		return ast.ASTHandle{}, err
	}
	if _, found := s.BoolVar(result); found {
		return result, nil
	}
	root, err := s.manager.NewRef(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&root)) }()
	decl, err := s.manager.GetDecl(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fm.Insert(decl); err != nil {
		return ast.ASTHandle{}, err
	}
	bv, err := s.MkBoolVar(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.SetVariableTheory(bv, t.FamilyID()); err != nil {
		return ast.ASTHandle{}, err
	}
	k := newSMTArithmeticValue()
	defer k.del(t.q)
	if err = t.q.SetBigRat(&k.rational, &value.Main); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.q.SetBigRat(&k.infinitesimal, &value.Epsilon); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.q.SetBigRat(&k.infinity, &value.Infinity); err != nil {
		return ast.ASTHandle{}, err
	}
	a, err := newSMTArithmeticAtom(t.q, bv, v, &k, smtArithmeticLowerAtom)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.mkBoundAxioms(a); err != nil {
		return ast.ASTHandle{}, err
	}
	t.unassignedAtoms[v]++
	t.varOccs[v] = append(t.varOccs[v], a)
	t.atoms = append(t.atoms, a)
	t.insertBv2a(bv, a)
	return result, nil
}

// ConflictMinimize is conflict_minimize: the least recorded upper bound.
func (t *SMTTheoryArith) ConflictMinimize() (simplex.Infinitesimal, bool) {
	v := newSMTArithmeticValue()
	defer v.del(t.q)
	t.keep(t.conflictMinimize(&v))
	finite, err := smtArithValueToInfinitesimal(t.q, &v)
	t.keep(err)
	return finite, v.infinity.IsZero()
}

// EnableRecordConflict is enable_record_conflict(bound).
func (t *SMTTheoryArith) EnableRecordConflict(bound ast.ASTHandle) error {
	t.enableRecordConflict(bound)
	return nil
}
