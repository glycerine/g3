package qe

import (
	"bytes"
	"errors"
	"fmt"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestQMaxRetainedOptimizerSurface(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	q, err := NewQMax(m, nil, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, q.Close()) })
	x := f.constant("qmaxX", f.real)
	three := f.own(m.ArithmeticRealNumeralText("3"))
	form := f.own(m.MkArithmeticLE(x, three))
	forms := predicateRefs(t, f, form)
	var out tactic.ModelRef
	t.Cleanup(func() { fmMust(t, out.Close()) })
	value := simplex.NewOptimum()
	status, err := q.Maximize(*forms, x, &value, &out)
	fmMust(t, err)
	if status != lbool.True || value.Infinite || optimumText(m, &value.Value) != "3" {
		t.Fatal(status, value)
	}
	var st statistics.Statistics
	fmMust(t, q.CollectStatistics(&st))
	if st.Size() == 0 {
		t.Fatal("optimizer statistics missing")
	}
	copy := *q
	if _, err = copy.Maximize(*forms, x, &value, &out); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	fmMust(t, q.Close())
	md, err := out.Get()
	fmMust(t, err)
	qsatKernelEvaluate(t, f, md, form)
	if err = q.CollectStatistics(&st); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}

func TestQMaxQuantifiedRealBoundCPP(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	q, err := NewQMax(m, nil, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, q.Close()) })
	x := f.constant("quantifiedX", f.real)
	y := f.variable(0, f.real)
	three := f.own(m.ArithmeticRealNumeralText("3"))
	body := f.own(m.MkOrArray([]ast.ASTHandle{f.own(m.MkArithmeticGE(y, x)), f.own(m.MkFamilyApp(f.r.arithmetic, 4, nil, []ast.ASTHandle{y, three}))}))
	name, err := m.Symbols.Intern("y")
	fmMust(t, err)
	form := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{name}, Body: body}))
	forms := predicateRefs(t, f, form)
	var out tactic.ModelRef
	t.Cleanup(func() { fmMust(t, out.Close()) })
	value := simplex.NewOptimum()
	status, err := q.Maximize(*forms, x, &value, &out)
	fmMust(t, err)
	if status != lbool.True || value.Infinite || optimumText(m, &value.Value) != "3" || optimumSign(m, &value.Epsilon) != 0 {
		t.Fatal(status, value)
	}
	md, err := out.Get()
	fmMust(t, err)
	decl, err := m.GetDecl(x)
	fmMust(t, err)
	actual, err := md.ConstantInterpretation(decl)
	fmMust(t, err)
	bound := f.own(m.ArithmeticNumeralFromRational(&value.Value, false))
	improvement := f.own(m.MkFamilyApp(f.r.arithmetic, 5, nil, []ast.ASTHandle{x, bound}))
	attainable := f.own(m.MkEq(x, actual))
	format := func(h ast.ASTHandle) string {
		var b bytes.Buffer
		fmMust(t, writeExpression(m, f.r.config, &b, h))
		return b.String()
	}
	script := "(set-logic ALL)\n(declare-const quantifiedX Real)\n(assert " + format(form) + ")\n(push 1)\n(assert " + format(improvement) + ")\n(check-sat-using qsat)\n(pop 1)\n(assert " + format(attainable) + ")\n(check-sat-using qsat)\n"
	if got := projectionCPP(t, script); got != "unsat\nsat" {
		t.Fatal(got, script)
	}
}
func TestQMaxAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "maximize"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 16384 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f := qsatKernelFixture(t)
					m := f.r.manager
					var q *QMax
					var err error
					if phase != "constructor" {
						q, err = NewQMax(m, nil, f.r.config)
						fmMust(t, err)
						t.Cleanup(func() { fmMust(t, q.Close()) })
					}
					x := f.constant("stoppedQmax", f.real)
					form := f.own(m.MkArithmeticLE(x, f.own(m.ArithmeticRealNumeralText("3"))))
					forms := predicateRefs(t, f, form)
					var out tactic.ModelRef
					t.Cleanup(func() { fmMust(t, out.Close()) })
					value := simplex.NewOptimum()
					var status lbool.LBool
					m.Budget().FailAfter(stop)
					if phase == "constructor" {
						q, err = NewQMax(m, nil, f.r.config)
					} else {
						status, err = q.Maximize(*forms, x, &value, &out)
					}
					m.Budget().FailAfter(-1)
					var failure memory.ConstructionFailure
					if errors.As(err, &failure) {
						fmMust(t, failure.Cleanup())
					}
					if phase == "constructor" && q != nil {
						fmMust(t, q.Close())
					}
					if err == nil {
						if phase == "maximize" && status != lbool.True {
							t.Fatal(status)
						}
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						var memout memory.Failure
						if !errors.As(err, &memout) || memout.Kind != memory.FailureMemout {
							t.Fatal(err)
						}
					}
				})
				if t.Failed() {
					t.Fatal("qmax frontier", stop)
				}
			}
			if !success {
				t.Fatal("no successful frontier")
			}
		})
	}
}

// optimumText and optimumSign read an Optimum component, a rational of the
// AST manager's rational manager.
func optimumText(m *ast.ASTManager, r *unumeral.Rational) string {
	text, err := m.NumeralService().Manager.String(r)
	if err != nil {
		panic(err)
	}
	return text
}

func optimumSign(m *ast.ASTManager, r *unumeral.Rational) int {
	sign, err := m.NumeralService().Manager.Sign(r)
	if err != nil {
		panic(err)
	}
	return int(sign)
}
