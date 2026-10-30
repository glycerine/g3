package tactic

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAtom2BoolVarSourceMappingScopeAndInterpretedState(t *testing.T) {
	m := astTestManager(t)
	a, err := NewAtom2BoolVar(m)
	satOK(t, err)
	p := goalTestAtom(t, m, "atom-map-p")
	q := goalTestAtom(t, m, "atom-map-q")
	interpreted := goalTestApp(t, m, ast.ASTOpEq, p, q)
	beforeP, err := m.GetRefCount(p)
	satOK(t, err)
	beforeInterpreted, err := m.GetRefCount(interpreted)
	satOK(t, err)

	satOK(t, a.Insert(p, 3))
	if got, err := a.ToBoolVar(p); err != nil || got != 3 {
		t.Fatal("to_bool_var", got, err)
	}
	if got, err := a.ToBoolVar(q); err != nil || got != sat.NullBoolVar {
		t.Fatal("absent null_bool_var", got, err)
	}
	if yes, err := a.InterpretedAtoms(); err != nil || yes {
		t.Fatal("uninterpreted constant changed map flag", yes, err)
	}
	afterP, err := m.GetRefCount(p)
	satOK(t, err)
	if afterP != beforeP+1 {
		t.Fatal("inherited expr2var key ownership", beforeP, afterP)
	}

	satOK(t, a.Push())
	satOK(t, a.Insert(interpreted, 8))
	if yes, err := a.InterpretedAtoms(); err != nil || !yes {
		t.Fatal("interpreted atom flag", yes, err)
	}
	afterInterpreted, err := m.GetRefCount(interpreted)
	satOK(t, err)
	if afterInterpreted != beforeInterpreted+1 {
		t.Fatal("interpreted key ownership", beforeInterpreted, afterInterpreted)
	}
	entries, err := a.Entries()
	satOK(t, err)
	seen := make(map[ast.ASTHandle]uint32, len(entries))
	for _, entry := range entries {
		seen[entry.Expr] = entry.Variable
	}
	if len(seen) != 2 || seen[p] != 3 || seen[interpreted] != 8 {
		t.Fatal("atom map iterator", seen)
	}

	satOK(t, a.Pop(1))
	if got, err := a.ToBoolVar(interpreted); err != nil || got != sat.NullBoolVar {
		t.Fatal("scoped atom survived pop", got, err)
	}
	if yes, err := a.InterpretedAtoms(); err != nil || !yes {
		t.Fatal("source pop must preserve cumulative interpreted flag", yes, err)
	}
	if got, err := m.GetRefCount(interpreted); err != nil || got != beforeInterpreted {
		t.Fatal("pop key release", got, beforeInterpreted, err)
	}
	if got, err := a.ToBoolVar(p); err != nil || got != 3 {
		t.Fatal("base atom lost during pop", got, err)
	}
	// The inherited map can contain null_bool_var even though to_bool_var uses it as
	// the absence sentinel. Source is_var remains the authoritative membership
	// query for this deliberately degenerate value.
	satOK(t, a.Insert(q, sat.NullBoolVar))
	if got, err := a.ToBoolVar(q); err != nil || got != sat.NullBoolVar {
		t.Fatal("null-valued source entry", got, err)
	}
	if yes, err := a.IsVar(q); err != nil || !yes {
		t.Fatal("null-valued source membership", yes, err)
	}
	entries, err = a.Entries()
	satOK(t, err)
	foundNull := false
	for _, entry := range entries {
		if entry.Expr == q {
			foundNull = entry.Variable == uint32(sat.NullBoolVar)
		}
	}
	if !foundNull {
		t.Fatal("iterator omitted null-valued mapping", entries)
	}
	// reset_recent forgets scope bookkeeping only; it does not change mapping
	// ownership or the cumulative interpreted flag.
	satOK(t, a.ResetRecent())
	if yes, err := a.IsVar(p); err != nil || !yes {
		t.Fatal("reset_recent changed mapping", yes, err)
	}

	satOK(t, a.Reset())
	if yes, err := a.InterpretedAtoms(); err != nil || yes {
		t.Fatal("reset interpreted flag", yes, err)
	}
	if got, err := m.GetRefCount(p); err != nil || got != beforeP {
		t.Fatal("reset key release", got, beforeP, err)
	}
	copyOwner := *a
	if err := copyOwner.Push(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied concrete owner", err)
	}
	satOK(t, a.Close())
	satOK(t, a.Close())
	if _, err := a.Manager(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed owner", err)
	}
}

func TestAtom2BoolVarSourceInversePolaritiesAndSparseSlots(t *testing.T) {
	m := astTestManager(t)
	a, err := NewAtom2BoolVar(m)
	satOK(t, err)
	defer func() { satOK(t, a.Close()) }()
	p := goalTestAtom(t, m, "inverse-p")
	q := goalTestAtom(t, m, "inverse-q")
	satOK(t, a.Insert(p, 0))
	satOK(t, a.Insert(q, 3))
	var inverse satGoalLiteralExpressions
	defer func() { satOK(t, inverse.close(m)) }()
	satOK(t, a.mkInverse(&inverse))
	if len(inverse.values) != 8 {
		t.Fatal("literal-index sparse inverse extent", len(inverse.values))
	}
	for _, test := range []struct {
		variable uint32
		atom     ast.ASTHandle
	}{
		{0, p},
		{3, q},
	} {
		positive := inverse.values[2*test.variable]
		negative := inverse.values[2*test.variable+1]
		if positive != test.atom {
			t.Fatal("positive inverse", test.variable, positive, test.atom)
		}
		yes, inner, err := m.IsNot(negative)
		satOK(t, err)
		if !yes || inner != test.atom {
			t.Fatal("negative inverse", test.variable, yes, inner, test.atom)
		}
	}
	for _, hole := range []int{2, 3, 4, 5} {
		if !inverse.values[hole].IsNull() {
			t.Fatal("sparse inverse hole", hole, inverse.values[hole])
		}
	}
}

func TestAtom2BoolVarActualGoalTranslationRecoveryAndModelConsumers(t *testing.T) {
	m := astTestManager(t)
	s, c, params := satSolverCheckFixture(t)
	key := "elim_vars"
	satOK(t, c.SetBoolName(&params, &key, false))
	satOK(t, s.UpdateParams(&params))
	a, err := NewAtom2BoolVar(m)
	satOK(t, err)
	defer func() { satOK(t, a.Close()) }()
	p := goalTestAtom(t, m, "consumer-p")
	q := goalTestAtom(t, m, "consumer-q")
	formula := goalTestApp(t, m, ast.ASTOpOr, p, q)
	tr, err := NewGoalTranslatorWithAtomMap(m, s, a, false)
	satOK(t, err)
	defer func() { satOK(t, tr.Close()) }()
	satOK(t, tr.Compile([]ast.ASTHandle{formula}))
	for _, atom := range []ast.ASTHandle{p, q} {
		v, err := a.ToBoolVar(atom)
		satOK(t, err)
		if v == sat.NullBoolVar {
			t.Fatal("goal2sat did not publish atom", atom)
		}
	}
	status, err := s.Check(nil)
	satOK(t, err)
	if status != lbool.True {
		t.Fatal("translated Boolean formula", status)
	}
	values, err := s.AppendModel(nil)
	satOK(t, err)
	md, err := BuildBooleanModel(tr, values)
	satOK(t, err)
	defer func() { satOK(t, md.Close()) }()
	for _, atom := range []ast.ASTHandle{p, q} {
		decl, err := m.GetDecl(atom)
		satOK(t, err)
		value, err := md.ConstantInterpretation(decl)
		satOK(t, err)
		if !m.IsTrue(value) && !m.IsFalse(value) {
			t.Fatal("Boolean model omitted atom", atom, value)
		}
	}

	// sat2goal consumes atom2bool_var::mk_inv. With a zero-conflict solver
	// boundary it must recover the original mapped atoms, not fresh helpers.
	recovered := goalTestNew(t, m, false, false)
	satOK(t, recoverSATGoalFormulas(tr, recovered, false, ^uint64(0)))
	n, err := recovered.Size()
	satOK(t, err)
	if n == 0 {
		t.Fatal("sat2goal recovered no clauses")
	}
	seen := make(map[ast.ASTHandle]bool)
	for i := uint32(0); i < n; i++ {
		f, err := recovered.Form(i)
		satOK(t, err)
		if f == p || f == q {
			seen[f] = true
		}
		args, err := m.GetNumArgs(f)
		if err == nil {
			for j := uint32(0); j < args; j++ {
				child, err := m.GetArg(f, j)
				satOK(t, err)
				if child == p || child == q {
					seen[child] = true
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("recovery did not use mapped atoms")
	}
}

func TestSATTacticConcreteAtomMapAndSATConfigPolicyPath(t *testing.T) {
	m, r, output := satTacticFixture(t)
	params, err := r.context.NewValue()
	satOK(t, err)
	defer func() { satOK(t, r.context.Del(&params)) }()
	key := "max_conflicts"
	satOK(t, r.context.SetUIntName(&params, &key, 0))
	key = "dimacs.display"
	satOK(t, r.context.SetBoolName(&params, &key, true))
	key = "phase"
	phase, err := m.Symbols.Intern("always_false")
	satOK(t, err)
	satOK(t, r.context.SetSymbolName(&params, &key, phase))
	key = "random_freq"
	satOK(t, r.context.SetDoubleName(&params, &key, 0))
	satOK(t, r.UpdateParams(&params))
	p := goalTestAtom(t, m, "config-p")
	q := goalTestAtom(t, m, "config-q")
	g := goalTestNew(t, m, false, false)
	satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpOr, p, q)))
	var result tactic.TacticResult
	defer func() { satOK(t, result.Close()) }()
	satOK(t, r.Apply(g, &result))
	if result.DecidedSat() || result.DecidedUnsat() || result.ModelConverter == nil {
		t.Fatal("sat_config max_conflicts did not reach the fresh tactic solver")
	}
	if output.Len() == 0 {
		t.Fatal("sat_config dimacs.display did not reach the tactic output path")
	}
	if r.application != nil {
		t.Fatal("concrete atom map/application survived lexical tactic cleanup")
	}
}
