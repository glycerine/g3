package core

// Source validation for src/tactic/core/tseitin_cnf_tactic.cpp.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func tseitinAtoms(t *testing.T, m *ast.ASTManager, count int) []ast.ASTHandle {
	t.Helper()
	s, err := m.MkBoolSort()
	simplifyMust(t, err)
	r := make([]ast.ASTHandle, count)
	for i := range r {
		name, e := m.Symbols.Intern(string(rune('a' + i)))
		simplifyMust(t, e)
		r[i], e = m.MkConst(name, s)
		simplifyMust(t, e)
		simplifyPin(t, m, r[i])
	}
	return r
}

func TestTseitinCNFNestedIffAndITEChain(t *testing.T) {
	for _, build := range []func(*testing.T, *ast.ASTManager, []ast.ASTHandle) ast.ASTHandle{
		func(t *testing.T, m *ast.ASTManager, a []ast.ASTHandle) ast.ASTHandle {
			iff, e := m.MkEq(a[1], a[2])
			simplifyMust(t, e)
			r, e := m.MkOr(a[0], iff)
			simplifyMust(t, e)
			return r
		},
		func(t *testing.T, m *ast.ASTManager, a []ast.ASTHandle) ast.ASTHandle {
			inner, e := m.MkIte(a[1], a[2], a[3])
			simplifyMust(t, e)
			r, e := m.MkIte(a[0], inner, a[4])
			simplifyMust(t, e)
			return r
		},
	} {
		m, c, g, services := solveEqFixture(t)
		runner, err := NewTseitinCNFCoreTactic(m, c, g, nil, services)
		simplifyMust(t, err)
		t.Cleanup(func() { simplifyMust(t, runner.Close()) })
		a := tseitinAtoms(t, m, 5)
		formula := build(t, m, a)
		goal, err := tactic.NewGoal(m, false, true, false)
		simplifyMust(t, err)
		t.Cleanup(func() { simplifyMust(t, goal.Close()) })
		simplifyMust(t, goal.Assert(formula))
		var result tactic.TacticResult
		t.Cleanup(func() { simplifyMust(t, result.Close()) })
		simplifyMust(t, runner.Apply(goal, &result))
		size, err := goal.Size()
		simplifyMust(t, err)
		if size < 3 {
			t.Fatal("compound Boolean gate was not encoded", size, runner.numAux)
		}
		for i := uint32(0); i < size; i++ {
			h, e := goal.Form(i)
			simplifyMust(t, e)
			if yes, _, e := runner.manager.IsNot(h); e != nil {
				simplifyMust(t, e)
			} else if yes {
				continue
			}
			n, e := runner.manager.GetNumArgs(h)
			simplifyMust(t, e)
			if n == 0 {
				continue
			}
			family, e := runner.manager.GetFamilyID(h)
			simplifyMust(t, e)
			op, e := runner.manager.GetDeclKind(h)
			simplifyMust(t, e)
			if family == runner.manager.BasicFamilyID() && op != ast.ASTOpOr {
				t.Fatal("non-CNF gate survived", op)
			}
		}
	}
}

func TestTseitinCNFCoreDependenciesModelFilterAndGuards(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	runner, err := NewTseitinCNFCoreTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a := tseitinAtoms(t, m, 4)
	iff, err := m.MkEq(a[1], a[2])
	simplifyMust(t, err)
	formula, err := m.MkOr(a[0], iff)
	simplifyMust(t, err)
	dep, err := m.DependencyLeaf(a[3])
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, false, true, true)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.AssertWithProof(formula, ast.ASTHandle{}, dep))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	if result.ModelConverter == nil {
		t.Fatal("auxiliary declaration was not filtered from models")
	}
	size, err := goal.Size()
	simplifyMust(t, err)
	for i := uint32(0); i < size; i++ {
		d, e := goal.Dependency(i)
		simplifyMust(t, e)
		leaves, e := m.DependencyLinearize(d, nil)
		simplifyMust(t, e)
		if len(leaves) != 1 || leaves[0] != a[3] {
			t.Fatal("clause lost core dependency", i, leaves)
		}
	}

	unsupported, err := m.MkAnd(a[0], a[1])
	simplifyMust(t, err)
	unsupported, err = m.MkEq(unsupported, a[2])
	simplifyMust(t, err)
	bad, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	defer func() { simplifyMust(t, bad.Close()) }()
	simplifyMust(t, bad.Assert(unsupported))
	err = runner.Apply(bad, &result)
	var exception tactic.TacticException
	if !errors.As(err, &exception) || exception.Message() != "operator not supported, apply simplifier before invoking this strategy" {
		t.Fatal("core simplifier guard", err)
	}
	full, err := MakeTseitinCNFTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	simplifyMust(t, full.IncRef())
	t.Cleanup(func() { simplifyMust(t, full.DecRef()) })
	simplifyMust(t, full.Apply(bad, &result))
}

func TestTseitinCNFCommonPatternAndBoundedDistributivity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		want  uint32
		build func(*testing.T, *ast.ASTManager, []ast.ASTHandle) ast.ASTHandle
	}{
		{"or-3and", 3, func(t *testing.T, m *ast.ASTManager, a []ast.ASTHandle) ast.ASTHandle {
			pairs := [][2]int{{0, 1}, {0, 2}, {1, 2}}
			terms := make([]ast.ASTHandle, 3)
			for i, p := range pairs {
				inner, e := m.MkOr(a[p[0]], a[p[1]])
				simplifyMust(t, e)
				terms[i], e = m.MkNot(inner)
				simplifyMust(t, e)
			}
			r, e := m.MkOrArray(terms)
			simplifyMust(t, e)
			return r
		}},
		{"distributivity", 2, func(t *testing.T, m *ast.ASTManager, a []ast.ASTHandle) ast.ASTHandle {
			inner, e := m.MkOr(a[1], a[2])
			simplifyMust(t, e)
			n, e := m.MkNot(inner)
			simplifyMust(t, e)
			r, e := m.MkOr(a[0], n)
			simplifyMust(t, e)
			return r
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, c, g, services := solveEqFixture(t)
			runner, err := NewTseitinCNFCoreTactic(m, c, g, nil, services)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, runner.Close()) })
			goal, err := tactic.NewGoal(m, false, false, false)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, goal.Close()) })
			simplifyMust(t, goal.Assert(tc.build(t, m, tseitinAtoms(t, m, 3))))
			var result tactic.TacticResult
			t.Cleanup(func() { simplifyMust(t, result.Close()) })
			simplifyMust(t, runner.Apply(goal, &result))
			size, err := goal.Size()
			simplifyMust(t, err)
			if size != tc.want {
				t.Fatal("source specialized clause count", size, tc.want)
			}
		})
	}
}

// The ite_extra clauses of a chained ite keep the chain context
// (MK_ITE_CLS pushes onto ctx): for (or f (ite a b (ite c d e))) the source
// emits (or a d e (not k!0)) and (or a (not d) (not e) k!0); without a they
// would force d or e even where the chain takes b.
func TestTseitinCNFITEExtraClausesKeepChainContext(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	runner, err := NewTseitinCNFCoreTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a := tseitinAtoms(t, m, 6)
	inner, err := m.MkIte(a[2], a[3], a[4])
	simplifyMust(t, err)
	outer, err := m.MkIte(a[0], a[1], inner)
	simplifyMust(t, err)
	formula, err := m.MkOr(a[5], outer)
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, false, true, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(formula))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	simplifyMust(t, err)
	found := 0
	for i := uint32(0); i < size; i++ {
		h, e := goal.Form(i)
		simplifyMust(t, e)
		n, e := m.GetNumArgs(h)
		simplifyMust(t, e)
		if n != 4 {
			continue
		}
		args := map[ast.ASTHandle]bool{}
		negated := map[ast.ASTHandle]bool{}
		for j := uint32(0); j < n; j++ {
			arg, e := m.GetArg(h, j)
			simplifyMust(t, e)
			if yes, inner, e := m.IsNot(arg); e != nil {
				simplifyMust(t, e)
			} else if yes {
				negated[inner] = true
				continue
			}
			args[arg] = true
		}
		if args[a[0]] && (args[a[3]] && args[a[4]] || negated[a[3]] && negated[a[4]]) {
			found++
		}
	}
	if found != 2 {
		t.Fatal("ite_extra clauses lost the chain context", found, size)
	}
}
