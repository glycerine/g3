package qe

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/statistics"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func qsatTacticOwner(t *testing.T, f *fmFixture, mode qsatMode) *QSATTactic {
	t.Helper()
	tt, err := newQSATTactic(f.r.manager, mode, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, tt.Close()) })
	return tt
}
func TestQSATTacticGoalsModelsAndOwnerLifetime(t *testing.T) {
	for _, mode := range []qsatMode{qsatSAT, qsatQE, qsatRecursiveQE} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			c := qsatPrepareFixture(t, "forall")
			f := c.f
			m := f.r.manager
			fmMust(t, m.RegisterDeclarationPlugins())
			tt := qsatTacticOwner(t, f, mode)
			g := f.goal(false, true)
			result := f.result()
			label := f.constant("sourceLabel", f.boolean)
			dep, err := m.DependencyLeaf(label)
			fmMust(t, err)
			fmMust(t, m.DependencyInc(dep))
			t.Cleanup(func() { fmMust(t, m.DependencyDec(dep)) })
			fmMust(t, g.AssertWithProof(c.original, ast.ASTHandle{}, dep))
			fmMust(t, result.BindManager(m))
			fmMust(t, result.AddGoal(g))
			fmMust(t, result.SetCore(dep))
			fmMust(t, tt.Apply(g, result))
			if len(result.Goals) != 2 || result.Goals[0] != g || result.Goals[1] != g {
				t.Fatal("source appends input goal")
			}
			depth, err := g.Depth()
			fmMust(t, err)
			if depth != 1 {
				t.Fatal(depth)
			}
			if !result.Core.IsNull() || result.ProofConverter != nil {
				t.Fatal("source output converters/core were not reset")
			}
			if mode == qsatSAT {
				n, err := g.Size()
				fmMust(t, err)
				if n != 0 || result.ModelConverter == nil {
					t.Fatal("SAT goal/model", n, result.ModelConverter)
				}
				var md tactic.ModelRef
				t.Cleanup(func() { fmMust(t, md.Close()) })
				fmMust(t, result.ModelConverter.ApplyUnindexed(&md))
				core, err := md.Get()
				fmMust(t, err)
				qsatKernelEvaluate(t, f, core, c.free)
				fmMust(t, tt.Close())
				fmMust(t, result.ModelConverter.ApplyUnindexed(&md))
				core, err = md.Get()
				fmMust(t, err)
				qsatKernelEvaluate(t, f, core, c.free)
			} else {
				n, err := g.Size()
				fmMust(t, err)
				if n != 1 {
					t.Fatal(n)
				}
				h, err := g.Form(0)
				fmMust(t, err)
				if h != c.free {
					t.Fatal("quantifier elimination did not preserve free parameter")
				}
				if result.ModelConverter != nil {
					t.Fatal("QE should not publish a model")
				}
			}
		})
	}
}
func TestQSATTacticCancelTranslationStatisticsAndIgnoredHooks(t *testing.T) {
	c := qsatPrepareFixture(t, "exists")
	f := c.f
	m := f.r.manager
	fmMust(t, m.RegisterDeclarationPlugins())
	tt := qsatTacticOwner(t, f, qsatSAT)
	g := f.goal(false, false)
	result := f.result()
	fmMust(t, g.Assert(c.original))
	m.ResourceLimit().Cancel()
	err := tt.Apply(g, result)
	m.ResourceLimit().ResetCancel()
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatal(err)
	}
	depth, err := g.Depth()
	fmMust(t, err)
	if depth != 0 || len(result.Goals) != 0 {
		t.Fatal("cancellation changed result")
	}
	fmMust(t, tt.Apply(g, result))
	if statistics.Sums(tt.CollectStatistics)["qsat num rounds"] == 0 {
		t.Fatal("missing source round statistics")
	}
	fmMust(t, tt.UpdateParams(nil))
	fmMust(t, tt.CollectParamDescrs(nil))
	fmMust(t, tt.SetLogic(usymbol.Symbol{}))
	fmMust(t, tt.SetProgressCallback(nil))
	fmMust(t, tt.ResetStatistics())
	if tt.search.rounds != 0 {
		t.Fatal("round counter was not reset")
	}
	fmMust(t, tt.Cleanup())
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, target.Close()) })
	fmMust(t, target.RegisterDeclarationPlugins())
	translated, err := tt.Translate(target)
	fmMust(t, err)
	copy := translated.(*QSATTactic)
	t.Cleanup(func() { fmMust(t, copy.Close()) })
	tr, err := ast.NewASTTranslation(m, target, true)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, tr.Close()) })
	fresh := f.goal(false, false)
	fmMust(t, fresh.Assert(c.original))
	g2, err := fresh.Translate(tr)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, g2.Close()) })
	r2 := f.result()
	fmMust(t, copy.Apply(g2, r2))
	bad := *tt
	if err = bad.Apply(g, result); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	fmMust(t, tt.IncRef())
	fmMust(t, tt.DecRef())
	if err = tt.Apply(g, result); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}
func TestQSATTacticUnsatAndRecursiveFreeBoolean(t *testing.T) {
	for _, kind := range []qsatMode{qsatSAT, qsatQE, qsatRecursiveQE} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			c := qsatPrepareFixture(t, "dependent")
			f := c.f
			m := f.r.manager
			fmMust(t, m.RegisterDeclarationPlugins())
			tt := qsatTacticOwner(t, f, kind)
			g := f.goal(false, false)
			result := f.result()
			fmMust(t, g.Assert(c.original))
			fmMust(t, g.Assert(f.own(m.MkNot(c.free))))
			fmMust(t, tt.Apply(g, result))
			inconsistent, err := g.Inconsistent()
			fmMust(t, err)
			if !inconsistent && kind != qsatRecursiveQE {
				t.Fatal("inconsistent quantified formula was not eliminated")
			}
			if kind == qsatRecursiveQE {
				// Source recursive reconstruction adds no final theory rewrite.
				// Its unsatisfiable Boolean residue can remain syntactically open.
				forms, e := g.AppendFormulas(nil)
				fmMust(t, e)
				h := f.own(m.MkAndArray(forms))
				var b bytes.Buffer
				fmMust(t, writeExpression(m, f.r.config, &b, h))
				if got := projectionCPP(t, "(declare-const free Bool)\n(assert "+b.String()+")\n(check-sat)\n"); got != "unsat" {
					t.Fatal(got, b.String())
				}
			}
		})
	}
}
func TestQSATTacticAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "sat-model", "unsat", "qe2", "qe-rec", "trace", "closed-trace"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 16384 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					c := qsatPrepareFixture(t, "exists")
					f := c.f
					m := f.r.manager
					fmMust(t, m.RegisterDeclarationPlugins())
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
					kind := qsatSAT
					if phase == "qe2" {
						kind = qsatQE
					} else if phase == "qe-rec" {
						kind = qsatRecursiveQE
					}
					var tt *QSATTactic
					var err error
					if phase != "constructor" {
						tt = qsatTacticOwner(t, f, kind)
					}
					g := f.goal(false, false)
					result := f.result()
					form := c.original
					if phase == "unsat" {
						form = f.own(m.MkFalse())
					}
					fmMust(t, g.Assert(form))
					m.Budget().FailAfter(stop)
					if phase == "constructor" {
						tt, err = newQSATTactic(m, kind, f.r.config)
					} else {
						err = tt.Apply(g, result)
					}
					m.Budget().FailAfter(-1)
					var failure memory.ConstructionFailure
					if errors.As(err, &failure) {
						fmMust(t, failure.Cleanup())
					}
					if phase == "constructor" && tt != nil {
						fmMust(t, tt.Close())
					}
					if err == nil {
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						var memout memory.Failure
						if !errors.As(err, &memout) || memout.Kind != memory.FailureMemout {
							t.Fatalf("%v\n%s", err, trace.String())
						}
					}
				})
				if t.Failed() {
					t.Fatal("QSAT tactic frontier", stop)
				}
			}
			if !success {
				t.Fatal("no successful frontier")
			}
		})
	}
}
