package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func objectiveBoundFilter(t *testing.T, m *ast.ASTManager) *tactic.FilterModelConverter {
	t.Helper()
	f, err := tactic.NewFilterModel(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.Close()) })
	return f
}

func TestSMTObjectiveLowerBoundExistingBooleanAndDomains(t *testing.T) {
	f := newArithmeticObjectiveFixture(t, false)
	x := f.variable("bound-existing-x")
	v := f.objective(x)
	filter := objectiveBoundFilter(t, f.m)
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	existing := f.pin(f.m.MkConst(astTestSymbol(t, f.m, "5 <= bound-existing-x"), boolean))
	astTestMust(t, f.s.Internalize(existing, true))
	count := len(f.a.atoms)
	var before, after bytes.Buffer
	astTestMust(t, filter.Display(&before))
	h, err := f.a.MakeObjectiveLowerBound(filter, v, simplex.RationalInfinitesimal(big.NewRat(5, 1)))
	astTestMust(t, err)
	astTestMust(t, filter.Display(&after))
	if h != existing || len(f.a.atoms) != count || before.String() != after.String() {
		t.Fatal("native pre-internalized Boolean guard changed")
	}
	if _, err := f.a.MakeObjectiveLowerBound(nil, v, simplex.Infinitesimal{}); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil filter accepted", err)
	}
	if _, err := f.a.MakeObjectiveLowerBound(filter, SMTNullTheoryVar, simplex.Infinitesimal{}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid objective accepted", err)
	}
}

func TestSMTObjectiveLowerBoundIntegralRealCastUsesUnitStep(t *testing.T) {
	s, f := smtModelUpdateFixture(t, false)
	startSMTModelUpdate(t, s, f)
	family := f.a.FamilyID()
	integer, err := f.m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x := f.pin(f.m.MkConst(astTestSymbol(t, f.m, "bound-cast-int-x"), integer))
	cast := f.app(smtArithOpToReal, x)
	astTestMust(t, s.Assert(f.app(smtArithOpLE, cast, f.number("10"))))
	v, err := f.a.AddObjective(cast)
	astTestMust(t, err)
	filter := objectiveBoundFilter(t, f.m)
	h, err := f.a.MakeObjectiveLowerBound(filter, v, simplex.RationalInfinitesimal(big.NewRat(5, 1)))
	astTestMust(t, err)
	astTestMust(t, s.Assert(f.pin(f.m.MkNot(h))))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	xnode := f.s.context.enodes.get(x)
	value, ok := f.a.solver.Upper(f.a.simplexVariables[xnode.TheoryVar(f.a.FamilyID())])
	if !ok || value.Main.Cmp(big.NewRat(4, 1)) != 0 || value.Epsilon.Sign() != 0 {
		t.Fatal("integral Real atom used an infinitesimal complement", ok, value.String())
	}
}

func TestSMTObjectiveLowerBoundRecoversSharedOptimization(t *testing.T) {
	s, f := smtModelUpdateFixture(t, false)
	startSMTModelUpdate(t, s, f)
	x := f.variable("bound-shared-x")
	five := f.number("5")
	decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "bound-shared-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
	f5 := f.pin(f.m.MkApp(decl, []ast.ASTHandle{five}))
	equal := f.pin(f.m.MkEq(fx, f5))
	astTestMust(t, s.Assert(f.pin(f.m.MkNot(equal))))
	astTestMust(t, s.Assert(f.app(smtArithOpLE, x, five)))
	v, err := f.a.AddObjective(x)
	astTestMust(t, err)
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	_, _, shared, err := f.a.MaximizeObjective(v)
	astTestMust(t, err)
	if !shared {
		t.Fatal("missing shared local optimum")
	}
	updated, err := s.UpdateModel(shared)
	astTestMust(t, err)
	if updated {
		t.Fatal("shared local optimum unexpectedly valid")
	}
	filter := objectiveBoundFilter(t, f.m)
	astTestMust(t, s.Push())
	h, err := f.a.MakeObjectiveLowerBound(filter, v, simplex.NewInfinitesimal(big.NewRat(5, 1), big.NewRat(-1, 1)))
	astTestMust(t, err)
	astTestMust(t, s.Assert(h))
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("infinitesimal decrement did not recover a consistent shared model", status)
	}
	md := acquireUpdateModel(t, s)
	ev, err := model.NewModelEvaluator(md, s.kernel.modelConfig)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, ev.Close()) })
	for _, condition := range []ast.ASTHandle{f.app(smtArithOpLT, x, five), f.pin(f.m.MkNot(equal))} {
		ref, err := ev.EvaluateValue(condition)
		astTestMust(t, err)
		h, err := f.m.RefGet(ref)
		astTestMust(t, err)
		valid := f.m.IsTrue(h)
		astTestMust(t, f.m.DelRef(&ref))
		if !valid {
			t.Fatal("decremented shared model violates its original constraints")
		}
	}
	checkNativeArithmeticObjective(t, "arithmetic_shared_optimum.smt2", `sat
 (objectives (x (+ 5.0 (* (- 1.0) epsilon))))
 (((< x 5) true) ((distinct (f x) (f 5)) true))`)

	astTestMust(t, s.Pop(1))
}

func TestSMTObjectiveLowerBoundThresholdAndComplement(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for _, truth := range []bool{false, true} {
			for _, eps := range []int64{-2, 0, 2} {
				if integer && eps != 0 {
					continue
				}
				t.Run(fmt.Sprint(integer, "/", truth, "/", eps), func(t *testing.T) {
					f := newArithmeticObjectiveFixture(t, integer)
					x := f.variable("objective-bound-x")
					v := f.objective(x)
					filter := objectiveBoundFilter(t, f.m)
					value := simplex.NewInfinitesimal(big.NewRat(5, 1), big.NewRat(eps, 1))
					h, err := f.a.MakeObjectiveLowerBound(filter, v, value)
					astTestMust(t, err)
					f.pin(h, nil)
					bv, found := f.s.BoolVar(h)
					if !found {
						t.Fatal("generated bound has no Boolean variable")
					}
					count, queue := len(f.a.atoms), len(f.a.newBoundAtoms)
					var before, after bytes.Buffer
					astTestMust(t, filter.Display(&before))
					again, err := f.a.MakeObjectiveLowerBound(filter, v, value)
					astTestMust(t, err)
					astTestMust(t, filter.Display(&after))
					if again != h || len(f.a.atoms) != count || len(f.a.newBoundAtoms) != queue || before.String() != after.String() {
						t.Fatal("reused bound created another atom/filter entry")
					}
					value.Main.SetInt64(100)
					value.Epsilon.SetInt64(100)
					astTestMust(t, f.a.Assign(bv, truth))
					astTestMust(t, f.a.Propagate())
					column := f.a.simplexVariables[v]
					got, ok := f.a.solver.Upper(column)
					if truth {
						got, ok = f.a.solver.Lower(column)
					}
					main, epsilon := int64(5), eps
					if !truth {
						if integer {
							main--
						} else {
							epsilon--
						}
					}
					if !ok || got.Main.Cmp(big.NewRat(main, 1)) != 0 || got.Epsilon.Cmp(big.NewRat(epsilon, 1)) != 0 {
						t.Fatal("native bound/complement", ok, got.String(), main, epsilon)
					}
				})
			}
		}
	}
}

func TestSMTObjectiveLowerBoundCompoundOffsetAndName(t *testing.T) {
	f := newArithmeticObjectiveFixture(t, false)
	x := f.variable("objective-bound-offset")
	obj := f.app(smtArithOpAdd, x, f.number("1/2"))
	v := f.objective(obj)
	filter := objectiveBoundFilter(t, f.m)
	h, err := f.a.MakeObjectiveLowerBound(filter, v, simplex.NewInfinitesimal(big.NewRat(3, 2), big.NewRat(-2, 1)))
	astTestMust(t, err)
	f.pin(h, nil)
	decl, err := f.m.GetDecl(h)
	astTestMust(t, err)
	name, err := f.m.GetASTName(decl)
	astTestMust(t, err)
	text, err := f.m.Symbols.Display(name)
	astTestMust(t, err)
	if text != "(3/2 -e*2) <= (+ objective-bound-offset (/ 1.0 2.0))" {
		t.Fatal("native bound name", text)
	}
	bv, _ := f.s.BoolVar(h)
	astTestMust(t, f.a.Assign(bv, true))
	astTestMust(t, f.a.Propagate())
	xcol, err := f.a.ensureVariable(x)
	astTestMust(t, err)
	value, ok := f.a.solver.Lower(xcol)
	if !ok || value.Main.Cmp(big.NewRat(1, 1)) != 0 || value.Epsilon.Cmp(big.NewRat(-2, 1)) != 0 {
		t.Fatal("compound objective constant was not subtracted", ok, value.String())
	}
}

func TestSMTObjectiveLowerBoundSolverScopesAndModelFilter(t *testing.T) {
	s, f := smtModelUpdateFixture(t, false)
	startSMTModelUpdate(t, s, f)
	x := f.variable("objective-bound-model-x")
	astTestMust(t, s.Assert(f.app(smtArithOpLT, x, f.number("5"))))
	v, err := f.a.AddObjective(x)
	astTestMust(t, err)
	filter := objectiveBoundFilter(t, f.m)
	for repeat := 0; repeat < 2; repeat++ {
		astTestMust(t, s.Push())
		h, err := f.a.MakeObjectiveLowerBound(filter, v, simplex.NewInfinitesimal(big.NewRat(5, 1), big.NewRat(-2, 1)))
		astTestMust(t, err)
		f.pin(h, nil)
		astTestMust(t, s.Assert(h))
		status, err := s.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("nonstandard interval should be feasible", status)
		}
		md := acquireUpdateModel(t, s)
		decl, err := f.m.GetDecl(h)
		astTestMust(t, err)
		present, err := md.HasInterpretation(decl)
		astTestMust(t, err)
		if !present {
			t.Fatal("generated Boolean was not present before filtering")
		}
		var ref tactic.ModelRef
		astTestMust(t, ref.Set(md))
		astTestMust(t, filter.ApplyUnindexed(&ref))
		filtered, err := ref.Get()
		astTestMust(t, err)
		present, err = filtered.HasInterpretation(decl)
		astTestMust(t, err)
		if present || modelUpdateNumber(t, f.m, filtered, x) != modelUpdateNumber(t, f.m, md, x) {
			t.Fatal("filter did not hide only the generated bound")
		}
		astTestMust(t, ref.Close())
		astTestMust(t, s.Pop(1))
		if _, found := f.s.BoolVar(h); found {
			t.Fatal("generated bound Boolean survived scope pop")
		}
		status, err = s.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("parent did not recover after generated bound pop", status)
		}
	}
}

func TestSMTObjectiveLowerBoundRejectsImproperSupremum(t *testing.T) {
	s, f := smtModelUpdateFixture(t, false)
	startSMTModelUpdate(t, s, f)
	x := f.variable("objective-bound-supremum")
	astTestMust(t, s.Assert(f.app(smtArithOpLT, x, f.number("5"))))
	v, err := f.a.AddObjective(x)
	astTestMust(t, err)
	filter := objectiveBoundFilter(t, f.m)
	h, err := f.a.MakeObjectiveLowerBound(filter, v, simplex.NewInfinitesimal(big.NewRat(5, 1), big.NewRat(2, 1)))
	astTestMust(t, err)
	astTestMust(t, s.Assert(h))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("positive infinitesimal above strict supremum was accepted", status)
	}
}

func TestSMTFiniteObjectiveBoundRejectsInfinityCoordinate(t *testing.T) {
	value := simplex.NewInfiniteInfinitesimal(big.NewRat(2, 3), big.NewRat(1, 1), big.NewRat(-1, 1))
	f := newArithmeticObjectiveFixture(t, false)
	x := f.variable("finite-rejects-infinity")
	v := f.objective(x)
	filter := objectiveBoundFilter(t, f.m)
	if _, err := f.a.MakeObjectiveLowerBound(filter, v, value); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("finite mi_ext bound silently projected infinity", err)
	}
	d := newDiffObjectiveFixture(t, false)
	y := d.variable("sparse-rejects-infinity")
	w, err := d.r.AddObjective(y)
	astTestMust(t, err)
	if _, err = d.r.MakeObjectiveLowerBound(nil, w, value); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("finite sparse bound silently projected infinity", err)
	}
	dense := newDenseObjectiveFixture(t, false)
	z := dense.variable("dense-rejects-infinity")
	q, err := dense.d.AddObjective(z)
	astTestMust(t, err)
	if _, err = dense.d.MakeObjectiveLowerBound(nil, q, value); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("finite dense bound silently projected infinity", err)
	}
}
