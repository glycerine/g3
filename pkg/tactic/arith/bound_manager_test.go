package arith

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func boundManagerFixture(t *testing.T) (*BoundManager, *ast.ASTManager) {
	t.Helper()
	base, m := fixDLFixture(t, false)
	b, err := NewBoundManager(m, base.context, base.globals)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, b.Close()) })
	return b, m
}
func TestBoundManagerComparisonSemantics(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for op := int32(2); op <= 5; op++ {
			for _, swapped := range []bool{false, true} {
				for negations := 0; negations < 3; negations++ {
					t.Run(fmt.Sprintf("int%v/op%d/swap%v/not%d", integer, op, swapped, negations), func(t *testing.T) {
						b, m := boundManagerFixture(t)
						x := fixDLConst(t, m, integer, "x")
						n := fixDLNumber(t, m, integer, "3")
						lhs, rhs := x, n
						if swapped {
							lhs, rhs = n, x
						}
						form := fixDLApp(t, m, op, lhs, rhs)
						for i := 0; i < negations; i++ {
							h, err := m.MkNot(form)
							equationMust(t, err)
							fixDLPin(t, m, h)
							form = h
						}
						equationMust(t, b.Collect(form, ast.ASTDependency{}))
						lower, ls, hasL, err := b.Lower(x)
						equationMust(t, err)
						upper, us, hasU, err := b.Upper(x)
						equationMust(t, err)
						if hasL == hasU {
							t.Fatal("expected exactly one bound")
						}
						for i := -10; i <= 10; i++ {
							value := big.NewRat(int64(i), 1)
							if !integer {
								value.Quo(value, big.NewRat(2, 1))
							}
							cmp := value.Cmp(big.NewRat(3, 1))
							if swapped {
								cmp = -cmp
							}
							want := false
							switch op {
							case 2:
								want = cmp <= 0
							case 3:
								want = cmp >= 0
							case 4:
								want = cmp < 0
							case 5:
								want = cmp > 0
							}
							if negations%2 != 0 {
								want = !want
							}
							got := true
							if hasL {
								c := value.Cmp(&lower)
								got = c > 0 || c == 0 && !ls
							}
							if hasU {
								c := value.Cmp(&upper)
								got = got && (c < 0 || c == 0 && !us)
							}
							if got != want {
								t.Fatal("bound changed the atom's meaning", value, lower.RatString(), ls, upper.RatString(), us)
							}
						}
					})
				}
			}
		}
	}
}

func TestBoundManagerEqualityDisjunctionNegationAndOrder(t *testing.T) {
	b, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, false, "y")
	var alternatives []ast.ASTHandle
	for i, text := range []string{"-3", "5", "2"} {
		number := fixDLNumber(t, m, true, text)
		var eq ast.ASTHandle
		var err error
		if i == 1 {
			eq, err = m.MkEq(number, x)
		} else {
			eq, err = m.MkEq(x, number)
		}
		equationMust(t, err)
		fixDLPin(t, m, eq)
		alternatives = append(alternatives, eq)
	}
	or, err := m.MkOrArgs(alternatives)
	equationMust(t, err)
	fixDLPin(t, m, or)
	equationMust(t, b.Collect(or, ast.ASTDependency{}))
	lo, strict, found, err := b.Lower(x)
	equationMust(t, err)
	if !found || strict || lo.Cmp(big.NewRat(-3, 1)) != 0 {
		t.Fatal("disjunction minimum", lo, strict, found)
	}
	hi, strict, found, err := b.Upper(x)
	equationMust(t, err)
	if !found || strict || hi.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("disjunction maximum", hi, strict, found)
	}
	// Unary minus is recognized recursively, without requiring simplify.
	number := fixDLNumber(t, m, false, "7/3")
	number = fixDLApp(t, m, 8, fixDLApp(t, m, 8, number))
	eq, err := m.MkEq(y, number)
	equationMust(t, err)
	fixDLPin(t, m, eq)
	equationMust(t, b.Collect(eq, ast.ASTDependency{}))
	lo, _, found, err = b.Lower(y)
	equationMust(t, err)
	if !found || lo.Cmp(big.NewRat(7, 3)) != 0 {
		t.Fatal("real equality", lo, found)
	}
	variables, err := b.Variables()
	equationMust(t, err)
	if len(variables) != 2 || variables[0] != x || variables[1] != y {
		t.Fatal("first-bound order", variables)
	}
	variables[0] = y
	lo.SetInt64(100)
	again, _, _, err := b.Lower(y)
	equationMust(t, err)
	if again.Cmp(big.NewRat(7, 3)) != 0 {
		t.Fatal("borrowed rational storage escaped")
	}
	variables, err = b.Variables()
	equationMust(t, err)
	if variables[0] != x {
		t.Fatal("borrowed iterator storage escaped")
	}
	// Special equality/disjunction recognition precedes NOT stripping.
	equationMust(t, b.Reset())
	not, err := m.MkNot(eq)
	equationMust(t, err)
	fixDLPin(t, m, not)
	twice, err := m.MkNot(not)
	equationMust(t, err)
	fixDLPin(t, m, twice)
	equationMust(t, b.Collect(twice, ast.ASTDependency{}))
	variables, err = b.Variables()
	equationMust(t, err)
	if len(variables) != 0 {
		t.Fatal("double-negated equality widened source recognition")
	}
	var reals []ast.ASTHandle
	for _, text := range []string{"1/2", "3/2"} {
		h, e := m.MkEq(y, fixDLNumber(t, m, false, text))
		equationMust(t, e)
		fixDLPin(t, m, h)
		reals = append(reals, h)
	}
	or, err = m.MkOrArgs(reals)
	equationMust(t, err)
	fixDLPin(t, m, or)
	equationMust(t, b.Collect(or, ast.ASTDependency{}))
	variables, err = b.Variables()
	equationMust(t, err)
	if len(variables) != 0 {
		t.Fatal("real disjunction accepted")
	}
}

func TestBoundManagerImprovementDependenciesAndAccumulation(t *testing.T) {
	b, m := boundManagerFixture(t)
	x := fixDLConst(t, m, false, "x")
	var dependencies []ast.ASTDependency
	for i := 0; i < 2; i++ {
		sort, err := m.MkBoolSort()
		equationMust(t, err)
		name, err := m.Symbols.Intern(fmt.Sprintf("label%d", i))
		equationMust(t, err)
		label, err := m.MkConst(name, sort)
		equationMust(t, err)
		fixDLPin(t, m, label)
		d, err := m.DependencyLeaf(label)
		equationMust(t, err)
		equationMust(t, m.DependencyInc(d))
		t.Cleanup(func() { equationMust(t, m.DependencyDec(d)) })
		dependencies = append(dependencies, d)
	}
	for _, lower := range []bool{true, false} {
		equationMust(t, b.Reset())
		op := int32(2)
		values := []string{"5", "4", "4", "6"}
		if lower {
			op = 3
			values = []string{"1", "2", "2", "0"}
		}
		deps := []ast.ASTDependency{dependencies[0], {}, dependencies[1], dependencies[1]}
		for i, text := range values {
			form := fixDLApp(t, m, op, x, fixDLNumber(t, m, false, text))
			equationMust(t, b.Collect(form, deps[i]))
			var got ast.ASTDependency
			var err error
			if lower {
				got, err = b.LowerDependency(x)
			} else {
				got, err = b.UpperDependency(x)
			}
			equationMust(t, err)
			if got != dependencies[0] {
				t.Fatal("null/equal/weaker update replaced borrowed dependency", lower, i, got)
			}
		}
		strictOp := int32(4)
		if lower {
			strictOp = 5
		}
		equationMust(t, b.Collect(fixDLApp(t, m, strictOp, x, fixDLNumber(t, m, false, values[1])), dependencies[1]))
		var dep ast.ASTDependency
		var err error
		if lower {
			dep, err = b.LowerDependency(x)
		} else {
			dep, err = b.UpperDependency(x)
		}
		equationMust(t, err)
		if dep != dependencies[1] {
			t.Fatal("strict improvement did not replace dependency")
		}
	}
	equationMust(t, b.Reset())
	first := fixDLGoal(t, m, false, false, false, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "1")))
	second := fixDLGoal(t, m, false, false, false, fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "9")))
	equationMust(t, b.CollectGoal(first))
	equationMust(t, b.CollectGoal(second))
	hasL, err := b.HasLower(x)
	equationMust(t, err)
	hasU, err := b.HasUpper(x)
	equationMust(t, err)
	if !hasL || !hasU {
		t.Fatal("goal collection reset prior bounds")
	}
}

func TestBoundManagerDisplayTraceAndOwnership(t *testing.T) {
	b, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, true, "y")
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("bound_manager")
	equationMust(t, b.BindTrace(tags))
	equationMust(t, b.Collect(fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "-2")), ast.ASTDependency{}))
	equationMust(t, b.Collect(fixDLApp(t, m, 2, y, fixDLNumber(t, m, true, "4")), ast.ASTDependency{}))
	var out bytes.Buffer
	equationMust(t, b.Display(&out))
	if out.String() != "-2 <= x < oo\n-oo < y <= 4\n" {
		t.Fatal("display", out.String())
	}
	if tags.ShouldTrace("bound_manager") {
		record := func(line int, body string) string {
			return fmt.Sprintf("-------- [bound_manager] operator() src/tactic/arith/bound_manager.cpp:%d ---------\n%s------------------------------------------------\n", line, body)
		}
		want := record(92, "processing:\n(>= x (- 2))\n") + record(127, "found bound for:\nx\n") + record(92, "processing:\n(<= y 4)\n") + record(127, "found bound for:\ny\n")
		if trace.String() != want {
			t.Fatal("source traces", trace.String(), want)
		}
	} else if trace.Len() != 0 {
		t.Fatal("erased trace wrote output")
	}
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.EnableAll(true)
	equationMust(t, b.BindTrace(refused))
	equationMust(t, b.Collect(fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "0")), ast.ASTDependency{}))
	copy := *b
	if err := copy.Reset(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owner", err)
	}
	equationMust(t, b.Close())
	if _, err := b.Variables(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed owner", err)
	}
}

func TestBoundManagerKeepsExactlyOneVariableReferenceUntilReset(t *testing.T) {
	b, m := boundManagerFixture(t)
	number := fixDLNumber(t, m, true, "2")
	sort, err := m.GetSort(number)
	equationMust(t, err)
	name, err := m.Symbols.Intern("bounded-only-owner")
	equationMust(t, err)
	x, err := m.MkConst(name, sort)
	equationMust(t, err)
	external, err := m.NewRef(x)
	equationMust(t, err)
	form, err := m.MkFamilyApp(b.family, 3, nil, []ast.ASTHandle{x, number})
	equationMust(t, err)
	formula, err := m.NewRef(form)
	equationMust(t, err)
	equationMust(t, b.Collect(form, ast.ASTDependency{}))
	equationMust(t, b.Collect(form, ast.ASTDependency{}))
	equationMust(t, m.DelRef(&formula))
	equationMust(t, m.DelRef(&external))
	if _, err = m.GetKind(x); err != nil {
		t.Fatal("bound manager did not retain its variable", err)
	}
	equationMust(t, b.Reset())
	if m.IsLiveHandle(x) {
		t.Fatal("reset retained an extra variable reference")
	}
	variables, err := b.Variables()
	equationMust(t, err)
	if len(variables) != 0 {
		t.Fatal("reset did not finalize iteration storage")
	}
}
