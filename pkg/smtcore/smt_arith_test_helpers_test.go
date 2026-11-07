package smtcore

// Test helpers shared by the arithmetic, objective, LRA and bit-vector
// tests (formerly kept with the replay arithmetic tests).

import (
	"fmt"
	"math/big"
	"sort"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
)

type arithmeticObjectiveFixture struct {
	t       *testing.T
	m       *ast.ASTManager
	s       *SMTInternalizerState
	a       *SMTTheoryArith
	sort    ast.ASTHandle
	integer bool
}

func newArithmeticObjectiveFixture(t *testing.T, integer bool) *arithmeticObjectiveFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a, err := NewSMTTheoryArith(family, integer)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(a))
	kind := int32(0)
	if integer {
		kind = 1
	}
	sort, err := m.MkFamilySort(family, kind, nil)
	astTestMust(t, err)
	return &arithmeticObjectiveFixture{t: t, m: m, s: s, a: a, sort: sort, integer: integer}
}

func (f *arithmeticObjectiveFixture) pin(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	astTestMust(f.t, err)
	r, err := f.m.NewRef(h)
	astTestMust(f.t, err)
	f.t.Cleanup(func() { astTestMust(f.t, f.m.DelRef(&r)) })
	return h
}

func (f *arithmeticObjectiveFixture) variable(name string) ast.ASTHandle {
	f.t.Helper()
	return f.pin(f.m.MkConst(astTestSymbol(f.t, f.m, name), f.sort))
}

func (f *arithmeticObjectiveFixture) number(text string) ast.ASTHandle {
	f.t.Helper()
	if f.integer {
		return f.pin(f.m.ArithmeticIntegerNumeralText(text))
	}
	return f.pin(f.m.ArithmeticRealNumeralText(text))
}

func (f *arithmeticObjectiveFixture) app(op int32, args ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	return f.pin(f.m.MkFamilyApp(f.a.FamilyID(), op, nil, args))
}

func (f *arithmeticObjectiveFixture) assert(h ast.ASTHandle) {
	f.t.Helper()
	astTestMust(f.t, f.s.Internalize(h, true))
	v, found := f.s.BoolVar(h)
	if !found {
		f.t.Fatal("arithmetic bound was not internalized")
	}
	astTestMust(f.t, f.a.Assign(v, true))
}

func (f *arithmeticObjectiveFixture) objective(h ast.ASTHandle) SMTTheoryVar {
	f.t.Helper()
	v, err := f.a.AddObjective(h)
	astTestMust(f.t, err)
	if v == SMTNullTheoryVar {
		f.t.Fatal("linear objective refused")
	}
	astTestMust(f.t, f.a.Propagate())
	if f.s.inconsistent {
		f.t.Fatal("objective fixture is inconsistent")
	}
	return v
}

func checkArithmeticObjective(t *testing.T, value SMTArithmeticObjectiveValue, main, epsilon string) {
	t.Helper()
	r, ok := new(big.Rat).SetString(main)
	if !ok {
		t.Fatal(main)
	}
	e, ok := new(big.Rat).SetString(epsilon)
	if !ok {
		t.Fatal(epsilon)
	}
	if value.Unbounded || value.Finite.Main.Cmp(r) != 0 || value.Finite.Epsilon.Cmp(e) != 0 {
		t.Fatal("objective value", value.Unbounded, value.Finite.String(), "want", main, epsilon)
	}
}

func boundClauseSet(ls []SMTLiteral) string {
	copy := append([]SMTLiteral(nil), ls...)
	sort.Slice(copy, func(i, j int) bool { return copy[i].Index() < copy[j].Index() })
	return fmt.Sprint(copy)
}

func smtProofTheoryRuleNode(t *testing.T, m *ast.ASTManager, proof ast.ASTHandle, rule string) ast.ASTHandle {
	t.Helper()
	stack := []ast.ASTHandle{proof}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[node] {
			continue
		}
		seen[node] = true
		kind, err := m.GetKind(node)
		astTestMust(t, err)
		if kind != ast.ASTApp {
			continue
		}
		decl, err := m.GetDecl(node)
		astTestMust(t, err)
		parameters, err := m.GetDeclParametersCopy(decl)
		astTestMust(t, err)
		matched := false
		for i := range parameters {
			kind, err := m.ParameterKind(parameters[i])
			astTestMust(t, err)
			if kind == ast.ASTParameterSymbol {
				symbol, err := m.ParameterSymbol(parameters[i])
				astTestMust(t, err)
				text, _, err := m.Symbols.BareString(symbol)
				astTestMust(t, err)
				matched = matched || text == rule
			}
			astTestMust(t, m.DelParameter(&parameters[i]))
		}
		if matched {
			return node
		}
		count, err := m.GetNumArgs(node)
		astTestMust(t, err)
		for i := uint32(0); i < count; i++ {
			arg, err := m.GetArg(node, i)
			astTestMust(t, err)
			stack = append(stack, arg)
		}
	}
	return ast.ASTHandle{}
}

func smtTestArithmeticExpressionContains(t *testing.T, m *ast.ASTManager, expression, target ast.ASTHandle) bool {
	t.Helper()
	stack := []ast.ASTHandle{expression}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == target {
			return true
		}
		if seen[node] {
			continue
		}
		seen[node] = true
		kind, err := m.GetKind(node)
		astTestMust(t, err)
		if kind != ast.ASTApp {
			continue
		}
		count, err := m.GetNumArgs(node)
		astTestMust(t, err)
		for i := uint32(0); i < count; i++ {
			argument, err := m.GetArg(node, i)
			astTestMust(t, err)
			stack = append(stack, argument)
		}
	}
	return false
}

func infiniteArithmeticValue(i, r, e int64) simplex.Infinitesimal {
	return simplex.NewInfiniteInfinitesimal(big.NewRat(i, 1), big.NewRat(r, 1), big.NewRat(e, 1))
}
func checkInfiniteArithmetic(t *testing.T, got simplex.Infinitesimal, i, r, e int64) {
	t.Helper()
	want := infiniteArithmeticValue(i, r, e)
	if got.Cmp(&want) != 0 {
		t.Fatal(got.String(), "want", want.String())
	}
}

func objectiveBoundFilter(t *testing.T, m *ast.ASTManager) *tactic.FilterModelConverter {
	t.Helper()
	f, err := tactic.NewFilterModel(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.Close()) })
	return f
}

// internalizeVar internalizes an arithmetic constant as theory_arith does
// when an arithmetic term mentions it (internalize_var): the core creates
// the enode and the theory attaches a variable. A bare constant
// internalized by the core alone has no theory variable (apply_sort_cnstr
// does nothing), as in the source.
func (f *arithmeticObjectiveFixture) internalizeVar(h ast.ASTHandle) {
	f.t.Helper()
	astTestMust(f.t, f.s.Internalize(h, false))
	if _, err := f.a.internalizeTermCore(h); err != nil {
		f.t.Fatal(err)
	}
}
