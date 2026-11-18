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
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func qsatSearchOwner(t *testing.T, f *fmFixture, mode qsatMode) *qsatSearch {
	t.Helper()
	s, err := newQSATSearch(f.r.manager, mode, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, s.Close()) })
	return s
}

func TestQSATSearchQEAnswerEquivalence(t *testing.T) {
	for _, mode := range qsatPreparationModes {
		if mode == "blocked" {
			continue
		}
		t.Run(mode, func(t *testing.T) {
			c := qsatPrepareFixture(t, mode)
			f := c.f
			m := f.r.manager
			fmMust(t, m.RegisterDeclarationPlugins())
			s := qsatSearchOwner(t, f, qsatQE)
			fmMust(t, s.initialize(c.formula))
			status, err := s.check()
			fmMust(t, err)
			if status != lbool.False {
				t.Fatal("QE search incomplete", status)
			}
			var hs []ast.ASTHandle
			for _, r := range s.answers {
				hs = append(hs, predicateValue(t, f, r))
			}
			answer := f.own(m.MkAndArray(hs))
			format := func(h ast.ASTHandle) string {
				var b bytes.Buffer
				fmMust(t, writeExpression(m, f.r.config, &b, h))
				return b.String()
			}
			script := "(set-logic ALL)\n(declare-const free Bool)\n(assert (not (= " + format(c.original) + " " + format(answer) + ")))\n"
			got := projectionCPP(t, script+"(check-sat)\n")
			// Old full QE mishandles this positive-label equivalence, while
			// ordinary SMT and qe2 both prove it. Use full QE only if ordinary
			// SMT cannot decide a quantified arithmetic counterexample.
			if got == "unknown" {
				got = projectionCPP(t, script+"(check-sat-using (then simplify qe smt))\n")
			}
			if got != "unsat" {
				t.Fatal("QE answer equivalence", got, script)
			}
			if len(s.freeVariables) != 0 {
				t.Fatal("forced QE retained bound variables")
			}
		})
	}
}

func TestQSATSearchAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "reset", "prepare", "sat", "backtrack", "qe", "trace", "closed-trace", "maximize", "maximize-strict"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 16384 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					mode := "ground"
					if phase == "backtrack" || strings.Contains(phase, "trace") {
						mode = "dependent"
					}
					c := qsatPrepareFixture(t, mode)
					f := c.f
					m := f.r.manager
					fmMust(t, m.RegisterDeclarationPlugins())
					if mode == "dependent" {
						h := f.own(m.MkAndArray([]ast.ASTHandle{f.own(m.MkNot(c.free)), c.original}))
						fmMust(t, m.SetRef(c.formula, h))
					}
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
					var s *qsatSearch
					var err error
					var status lbool.LBool
					kind := qsatSAT
					if phase == "qe" {
						kind = qsatQE
					}
					maximizing := strings.HasPrefix(phase, "maximize")
					var objective ast.ASTHandle
					var out tactic.ModelRef
					t.Cleanup(func() { fmMust(t, out.Close()) })
					value := simplex.NewOptimum()
					if maximizing {
						kind = qsatMaximize
						objective = f.constant("stopObjective", f.real)
						three := f.own(m.ArithmeticRealNumeralText("3"))
						op := int32(2)
						if phase == "maximize-strict" {
							op = 4
						}
						h := f.own(m.MkFamilyApp(f.r.arithmetic, op, nil, []ast.ASTHandle{objective, three}))
						fmMust(t, m.SetRef(c.formula, h))
					}
					if phase != "constructor" {
						s = qsatSearchOwner(t, f, kind)
					}
					if phase != "constructor" && phase != "reset" && phase != "prepare" && !maximizing {
						fmMust(t, s.initialize(c.formula))
					}
					m.Budget().FailAfter(stop)
					switch phase {
					case "constructor":
						s, err = newQSATSearch(m, kind, f.r.config)
					case "reset":
						err = s.reset()
					case "prepare":
						err = s.initialize(c.formula)
					case "maximize", "maximize-strict":
						status, err = s.maximize([]ast.ASTRef{c.formula}, objective, &out, &value)
					default:
						status, err = s.check()
					}
					m.Budget().FailAfter(-1)
					var failure memory.ConstructionFailure
					if errors.As(err, &failure) {
						fmMust(t, failure.Cleanup())
					}
					if phase == "constructor" && s != nil {
						fmMust(t, s.Close())
					}
					if err == nil {
						if phase != "constructor" && phase != "reset" && phase != "prepare" && status == lbool.Undef {
							t.Fatal("inconclusive allocation frontier was not a completed search")
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
					t.Fatal("QSAT search frontier", stop)
				}
			}
			if !success {
				t.Fatal("no successful frontier")
			}
		})
	}
}

func TestQSATSearchCancelResetReuseAndGuards(t *testing.T) {
	c := qsatPrepareFixture(t, "dependent")
	f := c.f
	m := f.r.manager
	fmMust(t, m.RegisterDeclarationPlugins())
	s := qsatSearchOwner(t, f, qsatSAT)
	fmMust(t, s.initialize(c.formula))
	m.ResourceLimit().Cancel()
	_, err := s.check()
	m.ResourceLimit().ResetCancel()
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatal("cancel did not precede solving", err)
	}
	if s.rounds != 1 || s.level != 0 {
		t.Fatal("cancellation source state", s.rounds, s.level)
	}
	status, err := s.check()
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	before := s.rounds
	falsehood := f.own(m.MkFalse())
	fmMust(t, m.SetRef(c.formula, falsehood))
	fmMust(t, s.initialize(c.formula))
	status, err = s.check()
	fmMust(t, err)
	if status != lbool.False || s.rounds != before+1 {
		t.Fatal("reset altered round statistics", status, s.rounds, before)
	}
	copy := *s
	if _, err = copy.check(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	other := newFMFixture(t)
	foreign := predicateRefs(t, other, other.own(other.r.manager.MkTrue()))
	if err = s.initialize((*foreign)[0]); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	fmMust(t, s.Close())
	if _, err = s.check(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}

func TestQSATSearchMaximizeModelsAndBounds(t *testing.T) {
	for _, mode := range []string{"finite", "strict", "unbounded", "unsat"} {
		t.Run(mode, func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			s := qsatSearchOwner(t, f, qsatMaximize)
			x := f.constant("optX", f.real)
			three := f.own(m.ArithmeticRealNumeralText("3"))
			one := f.own(m.ArithmeticRealNumeralText("1"))
			form := f.own(m.MkArithmeticLE(x, three))
			if mode == "strict" {
				form = f.own(m.MkFamilyApp(f.r.arithmetic, 4, nil, []ast.ASTHandle{x, three}))
			}
			if mode == "unbounded" {
				form = f.own(m.MkArithmeticGE(x, one))
			}
			if mode == "unsat" {
				form = f.own(m.MkAndArray([]ast.ASTHandle{form, f.own(m.MkArithmeticGE(x, f.own(m.ArithmeticRealNumeralText("4"))))}))
			}
			forms := predicateRefs(t, f, form)
			var out tactic.ModelRef
			t.Cleanup(func() { fmMust(t, out.Close()) })
			value := simplex.NewOptimum()
			status, err := s.maximize(*forms, x, &out, &value)
			fmMust(t, err)
			if mode == "unsat" {
				if status != lbool.False {
					t.Fatal(status)
				}
				return
			}
			if status != lbool.True {
				t.Fatal(status)
			}
			if mode == "unbounded" {
				if !value.Infinite {
					t.Fatal(value)
				}
			} else if value.Infinite || optimumText(m, &value.Value) != "3" || (optimumSign(m, &value.Epsilon) < 0) != (mode == "strict") {
				t.Fatal(value)
			}
			md, err := out.Get()
			fmMust(t, err)
			if md == nil {
				t.Fatal("missing optimization model")
			}
			qsatKernelEvaluate(t, f, md, form)
			fmMust(t, s.Close())
			qsatKernelEvaluate(t, f, md, form)
		})
	}
}

func TestQSATSearchResetPreservesUnsignedPredicateStatistics(t *testing.T) {
	c := qsatPrepareFixture(t, "integer-dependent")
	f := c.f
	fmMust(t, f.r.manager.RegisterDeclarationPlugins())
	s := qsatSearchOwner(t, f, qsatSAT)
	fmMust(t, s.initialize(c.formula))
	fmMust(t, s.reset())
	found := false
	for i := 0; i < s.stats.Size(); i++ {
		if s.stats.Key(i) == "qsat num predicates" {
			found = true
			if !s.stats.IsUint(i) || s.stats.UintValue(i) == 0 {
				t.Fatal("source unsigned predicate counter", i)
			}
		}
	}
	if !found {
		t.Fatal("reset discarded predicate statistics")
	}
}

func TestQSATSearchCPPMaximumBounds(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(fmt.Sprint(strict), func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			s := qsatSearchOwner(t, f, qsatMaximize)
			x := f.constant("optX", f.real)
			three := f.own(m.ArithmeticRealNumeralText("3"))
			op := int32(2)
			if strict {
				op = 4
			}
			form := f.own(m.MkFamilyApp(f.r.arithmetic, op, nil, []ast.ASTHandle{x, three}))
			forms := predicateRefs(t, f, form)
			var md tactic.ModelRef
			t.Cleanup(func() { fmMust(t, md.Close()) })
			value := simplex.NewOptimum()
			status, err := s.maximize(*forms, x, &md, &value)
			fmMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			core, err := md.Get()
			fmMust(t, err)
			decl, err := m.GetDecl(x)
			fmMust(t, err)
			v, err := core.ConstantInterpretation(decl)
			fmMust(t, err)
			attainable := f.own(m.MkArithmeticGE(x, v))
			format := func(h ast.ASTHandle) string {
				var b bytes.Buffer
				fmMust(t, writeExpression(m, f.r.config, &b, h))
				return b.String()
			}
			script := "(set-logic ALL)\n(declare-const optX Real)\n(assert " + format(form) + ")\n(push 1)\n(assert " + format(predicateValue(t, f, s.improvement)) + ")\n(check-sat)\n(pop 1)\n(assert " + format(attainable) + ")\n(check-sat)\n"
			if got := projectionCPP(t, script); got != "unsat\nsat" {
				t.Fatal("maximum improvement/attainability", got, script)
			}
		})
	}
}
func TestQSATSearchAlternationAndCPPStatus(t *testing.T) {
	for _, mode := range qsatPreparationModes {
		if mode == "blocked" {
			continue
		} // Source atom abstraction requires applications throughout.
		for _, fixed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", mode, fixed), func(t *testing.T) {
				c := qsatPrepareFixture(t, mode)
				f := c.f
				m := f.r.manager
				fmMust(t, m.RegisterDeclarationPlugins())
				constraint := c.free
				if !fixed {
					constraint = f.own(m.MkNot(c.free))
				}
				original := f.own(m.MkAndArray([]ast.ASTHandle{constraint, c.original}))
				forms := predicateRefs(t, f, original)
				s := qsatSearchOwner(t, f, qsatSAT)
				fmMust(t, s.initialize((*forms)[0]))
				status, err := s.check()
				fmMust(t, err)
				var out bytes.Buffer
				fmMust(t, writeExpression(m, f.r.config, &out, original))
				cpp := projectionCPP(t, "(set-logic ALL)\n(declare-const free Bool)\n(assert "+out.String()+")\n(check-sat-using qsat)\n")
				want := lbool.True
				if cpp == "unsat" {
					want = lbool.False
				} else if cpp != "sat" {
					t.Fatal("reference status", cpp)
				}
				if status != want {
					t.Fatal("native/reference status", status, cpp, "level", s.level, "rounds", s.rounds)
				}
				if status == lbool.True {
					md, e := s.saved.Get()
					fmMust(t, e)
					if md == nil {
						t.Fatal("missing saved model")
					}
				}
				t.Log("rounds", s.rounds, "level", s.level)
			})
		}
	}
}
