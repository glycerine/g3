package qe

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func qsatKernelFixture(t *testing.T) *fmFixture {
	t.Helper()
	f := newFMFixture(t)
	fmMust(t, f.r.manager.RegisterDeclarationPlugins())
	return f
}
func qsatKernelOwner(t *testing.T, f *fmFixture) *qsatKernel {
	t.Helper()
	k, err := newQSATKernel(f.r.manager, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, k.Close()) })
	return k
}
func qsatKernelModel(t *testing.T, f *fmFixture, k *qsatKernel) *model.ModelCore {
	t.Helper()
	s, err := k.solver()
	fmMust(t, err)
	md, err := s.AcquireModel()
	fmMust(t, err)
	if md == nil {
		t.Fatal("QSAT solver omitted model")
	}
	t.Cleanup(func() { fmMust(t, md.DecRef()) })
	return md
}
func qsatKernelEvaluate(t *testing.T, f *fmFixture, md *model.ModelCore, forms ...ast.ASTHandle) {
	t.Helper()
	m := f.r.manager
	e, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation, Trace: f.r.config.Trace})
	fmMust(t, err)
	defer func() { fmMust(t, e.Close()) }()
	out := predicateRefs(t, f, ast.ASTHandle{})
	for _, h := range forms {
		fmMust(t, e.Evaluate(h, (*out)[0]))
		if !m.IsTrue(predicateValue(t, f, (*out)[0])) {
			t.Fatal("solver model violates formula")
		}
	}
}
func TestQSATKernelModelCoreResetAndParameters(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	modelKey, relevancyKey, splitKey := "model", "relevancy_lvl", "case_split_strategy"
	b, err := f.r.config.Context.GetBoolName(&k.params, &modelKey, false)
	fmMust(t, err)
	r, err := f.r.config.Context.GetUIntName(&k.params, &relevancyKey, 99)
	fmMust(t, err)
	split, err := f.r.config.Context.GetUIntName(&k.params, &splitKey, 99)
	fmMust(t, err)
	if !b || r != 0 || split != 2 {
		t.Fatal("kernel source overrides", b, r, split)
	}
	a := f.constant("kernelA", f.boolean)
	na := f.own(m.MkNot(a))
	fmMust(t, k.assertExpression(a))
	s, err := k.solver()
	fmMust(t, err)
	status, err := s.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	md := qsatKernelModel(t, f, k)
	qsatKernelEvaluate(t, f, md, a)
	status, err = s.CheckSat([]ast.ASTHandle{na})
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	core := predicateRefs(t, f, a)
	fmMust(t, k.getCore(core))
	projectionExpect(t, f, *core, []ast.ASTHandle{na})
	var st statistics.Statistics
	fmMust(t, s.CollectStatistics(&st))
	if st.Size() == 0 {
		t.Fatal("kernel solver statistics missing")
	}
	fmMust(t, k.reset())
	if _, err = s.NumAssertions(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("old solver remained open", err)
	}
	s, err = k.solver()
	fmMust(t, err)
	n, err := s.NumAssertions()
	fmMust(t, err)
	if n != 0 {
		t.Fatal("reset retained assertions")
	}
	fmMust(t, k.assertExpression(na))
	status, err = s.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	qsatKernelEvaluate(t, f, qsatKernelModel(t, f, k), na)
	fmMust(t, k.getCore(core))
	if len(*core) != 0 {
		t.Fatal("SAT core retained old values")
	}
	fmMust(t, k.Close())
	qsatKernelEvaluate(t, f, md, a)
}
func TestQSATKernelIndependentSolversArithmeticAndGuards(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	ex, fa := qsatKernelOwner(t, f), qsatKernelOwner(t, f)
	x := f.constant("kernelX", f.integer)
	lower := f.own(m.MkArithmeticGE(x, f.number("3")))
	upper := f.own(m.MkArithmeticLE(x, f.number("2")))
	fmMust(t, ex.assertExpression(lower))
	fmMust(t, fa.assertExpression(upper))
	for _, k := range []*qsatKernel{ex, fa} {
		s, err := k.solver()
		fmMust(t, err)
		status, err := s.CheckSat(nil)
		fmMust(t, err)
		if status != lbool.True {
			t.Fatal("independent arithmetic kernel", status)
		}
		h := lower
		if k == fa {
			h = upper
		}
		qsatKernelEvaluate(t, f, qsatKernelModel(t, f, k), h)
	}
	s, err := ex.solver()
	fmMust(t, err)
	tag := f.constant("upperAssumption", f.boolean)
	fmMust(t, ex.assertExpression(f.own(m.MkImplies(tag, upper))))
	status, err := s.CheckSat([]ast.ASTHandle{tag})
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("arithmetic assumption conflict", status)
	}
	core := predicateRefs(t, f)
	fmMust(t, ex.getCore(core))
	projectionExpect(t, f, *core, []ast.ASTHandle{tag})
	copy := *ex
	if _, err = copy.solver(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if err = ex.getCore(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	other := newFMFixture(t)
	foreign := other.own(other.r.manager.MkTrue())
	if err = ex.assertExpression(foreign); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	fmMust(t, ex.Close())
	if err = ex.reset(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}

func TestQSATKernelCPPArithmeticAssumptionCore(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	x := f.constant("kernelX", f.integer)
	tag := f.constant("upperAssumption", f.boolean)
	fmMust(t, k.assertExpression(f.own(m.MkArithmeticGE(x, f.number("3")))))
	fmMust(t, k.assertExpression(f.own(m.MkImplies(tag, f.own(m.MkArithmeticLE(x, f.number("2")))))))
	s, err := k.solver()
	fmMust(t, err)
	status, err := s.CheckSat([]ast.ASTHandle{tag})
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	core := predicateRefs(t, f)
	fmMust(t, k.getCore(core))
	projectionExpect(t, f, *core, []ast.ASTHandle{tag})
	script := "(declare-const kernelX Int)\n(declare-const upperAssumption Bool)\n(assert (>= kernelX 3))\n(assert (=> upperAssumption (<= kernelX 2)))\n(check-sat-assuming (upperAssumption))\n"
	if got := projectionCPP(t, script); got != "unsat" {
		t.Fatal(got)
	}
}

func TestQSATPreparationFeedsDualKernelDefinitions(t *testing.T) {
	c := qsatPrepareFixture(t, "integer-dependent")
	f := c.f
	m := f.r.manager
	fmMust(t, m.RegisterDeclarationPlugins())
	p := qsatPrepareOwner(t, f)
	fmMust(t, p.hoist(qsatSAT, c.formula))
	defs := predicateRefs(t, f)
	level := EmptyMaxLevel()
	fmMust(t, p.abstraction.AbstractAtoms(predicateValue(t, f, c.formula), &level, defs))
	out := predicateRefs(t, f, ast.ASTHandle{})
	fmMust(t, p.abstraction.MakeAbstract(predicateValue(t, f, c.formula), (*out)[0]))
	matrix := predicateValue(t, f, (*out)[0])
	ex, fa := qsatKernelOwner(t, f), qsatKernelOwner(t, f)
	var hs []ast.ASTHandle
	for _, r := range *defs {
		hs = append(hs, predicateValue(t, f, r))
	}
	definition := f.own(m.MkAndArray(hs))
	negative := f.own(m.MkNot(matrix))
	for _, k := range []*qsatKernel{ex, fa} {
		fmMust(t, k.assertExpression(definition))
		h := matrix
		if k == fa {
			h = negative
		}
		fmMust(t, k.assertExpression(h))
		s, err := k.solver()
		fmMust(t, err)
		status, err := s.CheckSat(nil)
		fmMust(t, err)
		if status != lbool.True {
			t.Fatal("prepared kernel was not satisfiable", status)
		}
		md := qsatKernelModel(t, f, k)
		qsatKernelEvaluate(t, f, md, definition, h)
		valid, err := p.abstraction.ValidateDefinitions(md)
		fmMust(t, err)
		if !valid {
			t.Fatal("kernel model violates predicate definitions")
		}
	}
}
func TestQSATKernelAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "reset", "assert", "core", "trace", "closed-trace"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 8192 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f := qsatKernelFixture(t)
					m := f.r.manager
					var trace bytes.Buffer
					if strings.Contains(phase, "trace") {
						var w io.Writer = &trace
						if phase == "closed-trace" {
							w = closedPredicateTrace{}
						}
						tags := diagnostic.NewTraceTagsWithWriter(w)
						tags.Enable("qe")
						t.Cleanup(tags.Finalize)
						f.r.config.Trace = tags
					}
					a := f.constant("stopKernel", f.boolean)
					na := f.own(m.MkNot(a))
					core := predicateRefs(t, f, a)
					var k *qsatKernel
					var err error
					if phase != "constructor" {
						k = qsatKernelOwner(t, f)
					}
					if phase == "core" || strings.Contains(phase, "trace") {
						fmMust(t, k.assertExpression(a))
						s, e := k.solver()
						fmMust(t, e)
						status, e := s.CheckSat([]ast.ASTHandle{na})
						fmMust(t, e)
						if status != lbool.False {
							t.Fatal(status)
						}
					}
					m.Budget().FailAfter(stop)
					switch phase {
					case "constructor":
						k, err = newQSATKernel(m, f.r.config)
					case "reset":
						err = k.reset()
					case "assert":
						err = k.assertExpression(a)
					default:
						err = k.getCore(core)
					}
					m.Budget().FailAfter(-1)
					var failure memory.ConstructionFailure
					if errors.As(err, &failure) {
						fmMust(t, failure.Cleanup())
					}
					if phase == "constructor" && k != nil {
						fmMust(t, k.Close())
					}
					if err == nil {
						if phase == "trace" && f.r.config.Trace.ShouldTrace("qe") && !strings.Contains(trace.String(), "get_core src/qe/qsat.cpp:563") {
							t.Fatal(trace.String())
						}
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						var memout memory.Failure
						if !errors.As(err, &memout) || memout.Kind != memory.FailureMemout {
							t.Fatalf("%v\n%s", err, trace.String())
						}
					}
				})
				if t.Failed() {
					t.Fatal("QSAT kernel allocation frontier", stop)
				}
			}
			if !success {
				t.Fatal("no successful frontier")
			}
		})
	}
}
