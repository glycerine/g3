package smtlogics_test

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
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func ackrConfig(t *testing.T, c *cmd_context.Context) smtcore.SMTSolverConfig {
	t.Helper()
	m, err := c.Manager()
	qfaufliaMust(t, err)
	g, err := c.GlobalParameters()
	qfaufliaMust(t, err)
	old, err := uparams.BindProcessGlobalParamsService(g)
	qfaufliaMust(t, err)
	t.Cleanup(func() { _, err := uparams.BindProcessGlobalParamsService(old); qfaufliaMust(t, err) })
	coop, err := cooperate.NewCooperation(m.Budget())
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, coop.Close()) })
	services := model.ModelEvaluatorConfig{Context: g.Context, Globals: g, Memory: m.NumeralService().RuntimeOwner(), Cooperation: coop}
	return smtcore.SMTSolverConfig{Params: g.Context, Globals: g, ModelEvaluator: services}
}
func ackrSettings(t *testing.T, config smtcore.SMTSolverConfig, backend string, eager bool) *uparams.ParamsRef {
	t.Helper()
	p, err := config.Params.NewValue()
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, config.Params.Del(&p)) })
	for key, value := range map[string]bool{"sat_backend": backend != "smt", "inc_sat_backend": backend == "incsat", "eager": eager} {
		key := key
		qfaufliaMust(t, config.Params.SetBoolName(&p, &key, value))
	}
	return &p
}

func TestQFUFBVAckrRawBackendModelsSurviveClose(t *testing.T) {
	for _, backend := range []string{"smt", "sat", "incsat"} {
		for _, eager := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/%v", backend, eager), func(t *testing.T) {
				c := qfliaContext(t, `(set-logic QF_UFBV)(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))(declare-fun f ((_ BitVec 4)) (_ BitVec 4))(assert (bvult x y))(assert (bvult (f x) (f y)))`)
				m, err := c.Manager()
				qfaufliaMust(t, err)
				config := ackrConfig(t, c)
				p := ackrSettings(t, config, backend, eager)
				r, err := smtlogics.NewQFUFBVAckrCoreTactic(m, p, config, io.Discard)
				qfaufliaMust(t, err)
				t.Cleanup(func() { qfaufliaMust(t, r.Close()) })
				qfaufliaMust(t, r.UpdateParams(p))
				goal, err := tactic.NewGoal(m, false, true, false)
				qfaufliaMust(t, err)
				t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
				qfaufliaMust(t, c.AssertGoal(goal))
				original, err := tactic.NewGoalCopy(goal)
				qfaufliaMust(t, err)
				t.Cleanup(func() { qfaufliaMust(t, original.Close()) })
				var result tactic.TacticResult
				t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
				qfaufliaMust(t, r.Apply(goal, &result))
				if !result.DecidedSat() || result.ModelConverter == nil {
					t.Fatal("missing raw Ackermannization model")
				}
				if n, err := goal.Size(); err != nil || n != 2 {
					t.Fatal("raw source mutated input", n, err)
				}
				before := statistics.Sums(r.CollectStatistics)
				qfaufliaMust(t, r.Cleanup())
				if before["lackr-its"] != statistics.Sums(r.CollectStatistics)["lackr-its"] {
					t.Fatal("source cleanup reset stats")
				}
				if !eager && before["lackr-its"] == 0 {
					t.Fatal("lazy iterations not reported")
				}
				qfaufliaMust(t, r.Close())
				var out tactic.ModelRef
				t.Cleanup(func() { qfaufliaMust(t, out.Close()) })
				qfaufliaMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &out))
				md, err := out.Get()
				qfaufliaMust(t, err)
				ev, err := model.NewModelEvaluator(md, config.ModelEvaluator)
				qfaufliaMust(t, err)
				t.Cleanup(func() { qfaufliaMust(t, ev.Close()) })
				qfaufliaMust(t, ev.SetModelCompletion(true))
				n, err := original.Size()
				qfaufliaMust(t, err)
				if n == 0 {
					t.Fatal("empty original model check")
				}
				for i := uint32(0); i < n; i++ {
					h, err := original.Form(i)
					qfaufliaMust(t, err)
					value, err := ev.EvaluateValue(h)
					qfaufliaMust(t, err)
					v, err := m.RefGet(value)
					qfaufliaMust(t, err)
					valid := m.IsTrue(v)
					qfaufliaMust(t, m.DelRef(&value))
					if !valid {
						t.Fatal("fixed UF model violates original formula", i)
					}
				}
				declarations := append([]ast.ASTHandle(nil), md.ConstantDeclarations...)
				for i := uint32(0); i < md.NumFunctionDeclarations(); i++ {
					d, err := md.Function(i)
					qfaufliaMust(t, err)
					declarations = append(declarations, d)
				}
				for _, d := range declarations {
					name, err := m.GetASTName(d)
					qfaufliaMust(t, err)
					text, err := m.Symbols.String(name)
					qfaufliaMust(t, err)
					if text != "x" && text != "y" && text != "f" {
						t.Fatal("abstraction/backend helper escaped fixed model", text)
					}
				}
			})
		}
	}
}

func TestQFUFBVAckrConstructorAndApplyAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "smt", "sat", "incsat", "strategy"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 16384 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					c := qfliaContext(t, `(set-logic QF_UFBV)(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))(declare-fun f ((_ BitVec 4)) (_ BitVec 4))(assert (= (bvmul x x) #x1))(assert (bvult x y))(assert (bvult (f x) (f y)))`)
					m, err := c.Manager()
					qfaufliaMust(t, err)
					var r tactic.Tactic
					var closeRaw *smtlogics.QFUFBVAckrCoreTactic
					var strategy tactic.Tactic
					var goal *tactic.Goal
					var result tactic.TacticResult
					if phase != "constructor" {
						config := ackrConfig(t, c)
						backend := phase
						if phase == "strategy" {
							backend = "incsat"
							qfaufliaMust(t, config.Globals.SourceSetName("ackermannization.eager", "false"))
						}
						p := ackrSettings(t, config, backend, false)
						if phase == "strategy" {
							strategy, err = c.NewBuiltinTactic("qfufbv_ackr")
							qfaufliaMust(t, err)
							qfaufliaMust(t, strategy.IncRef())
							t.Cleanup(func() { qfaufliaMust(t, strategy.DecRef()) })
							qfaufliaMust(t, strategy.UpdateParams(p))
						} else {
							closeRaw, err = smtlogics.NewQFUFBVAckrCoreTactic(m, p, config, io.Discard)
							qfaufliaMust(t, err)
							qfaufliaMust(t, closeRaw.UpdateParams(p))
							t.Cleanup(func() { qfaufliaMust(t, closeRaw.Close()) })
						}
						goal, err = tactic.NewGoal(m, false, true, false)
						qfaufliaMust(t, err)
						t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
						qfaufliaMust(t, c.AssertGoal(goal))
						t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
					}
					m.Budget().FailAfter(stop)
					if phase == "constructor" {
						r, err = c.NewBuiltinTactic("qfufbv_ackr")
					} else if phase == "strategy" {
						err = strategy.Apply(goal, &result)
					} else {
						err = closeRaw.Apply(goal, &result)
					}
					m.Budget().FailAfter(-1)
					if r != nil {
						qfaufliaMust(t, r.IncRef())
						qfaufliaMust(t, r.DecRef())
					}
					var failure memory.ConstructionFailure
					if errors.As(err, &failure) {
						qfaufliaMust(t, failure.Cleanup())
					}
					if err == nil {
						success = true
						if phase != "constructor" && !result.DecidedSat() {
							t.Fatal("successful application incomplete")
						}
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						var memout memory.Failure
						if !errors.As(err, &memout) || memout.Kind != memory.FailureMemout {
							t.Fatal(err)
						}
					}
				})
				if t.Failed() {
					t.Fatal("Ackermannization allocation frontier", phase, stop)
				}
			}
			if !success {
				t.Fatal("no successful Ackermannization frontier", phase)
			}
		})
	}
}

func TestQFUFBVAckrConstructorSnapshotAndTranslationBackendReset(t *testing.T) {
	c := qfliaContext(t, `(set-logic QF_UFBV)(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))(declare-fun f ((_ BitVec 4)) (_ BitVec 4))(assert (bvult x y))(assert (bvult (f x) (f y)))`)
	m, err := c.Manager()
	qfaufliaMust(t, err)
	config := ackrConfig(t, c)
	p := ackrSettings(t, config, "incsat", false)
	r, err := smtlogics.NewQFUFBVAckrCoreTactic(m, p, config, io.Discard)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, r.Close()) })
	goal, err := tactic.NewGoal(m, false, true, false)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
	qfaufliaMust(t, c.AssertGoal(goal))
	verbose, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, verbose.Close()) })
	previous, err := diagnostic.BorrowProcessVerboseStream()
	qfaufliaMust(t, err)
	level, err := diagnostic.ProcessVerbosityLevel()
	qfaufliaMust(t, err)
	qfaufliaMust(t, diagnostic.SetProcessVerboseStream(verbose))
	qfaufliaMust(t, diagnostic.SetProcessVerbosityLevel(10))
	t.Cleanup(func() {
		qfaufliaMust(t, diagnostic.SetProcessVerboseStream(previous))
		qfaufliaMust(t, diagnostic.SetProcessVerbosityLevel(level))
	})
	var result tactic.TacticResult
	t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
	qfaufliaMust(t, r.Apply(goal, &result))
	text, err := verbose.String()
	qfaufliaMust(t, err)
	if !strings.Contains(text, "(smt.tactic start)") || strings.Contains(text, "(card2bv ") {
		t.Fatal("constructor eagerly applied backend options", text)
	}
	qfaufliaMust(t, result.Prepare(m))
	qfaufliaMust(t, verbose.ResetText())
	key := "eager"
	qfaufliaMust(t, config.Params.SetBoolName(p, &key, true))
	qfaufliaMust(t, r.UpdateParams(p))
	qfaufliaMust(t, r.Apply(goal, &result))
	text, err = verbose.String()
	qfaufliaMust(t, err)
	if !strings.Contains(text, "(card2bv ") || strings.Contains(text, "(smt.tactic start)") {
		t.Fatal("backend update not applied", text)
	}
	if statistics.Sums(r.CollectStatistics)["lackr-its"] < 2 {
		t.Fatal("backend update replaced constructor eager snapshot", statistics.Sums(r.CollectStatistics))
	}
	qfaufliaMust(t, r.ResetStatistics())
	if statistics.Sums(r.CollectStatistics)["lackr-its"] != 0 || statistics.Sums(r.CollectStatistics)["ackr-constraints"] != 0 {
		t.Fatal("statistics reset")
	}
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, target.Close()) })
	qfaufliaMust(t, target.RegisterDeclarationPlugins())
	translated, err := r.Translate(target)
	qfaufliaMust(t, err)
	qfaufliaMust(t, translated.IncRef())
	t.Cleanup(func() { qfaufliaMust(t, translated.DecRef()) })
	tr, err := ast.NewASTTranslation(m, target, false)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, tr.Close()) })
	translatedGoal, err := goal.Translate(tr)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, translatedGoal.Close()) })
	qfaufliaMust(t, r.Close())
	qfaufliaMust(t, verbose.ResetText())
	var translatedResult tactic.TacticResult
	t.Cleanup(func() { qfaufliaMust(t, translatedResult.Close()) })
	qfaufliaMust(t, translated.Apply(translatedGoal, &translatedResult))
	text, err = verbose.String()
	qfaufliaMust(t, err)
	if !translatedResult.DecidedSat() || !strings.Contains(text, "(smt.tactic start)") || strings.Contains(text, "(card2bv ") {
		t.Fatal("translation copied updated backend flags instead of constructor defaults", text)
	}
}

func TestQFUFBVAckrRefusalOrderAndMetadata(t *testing.T) {
	for _, flags := range [][2]bool{{false, true}, {true, false}, {true, true}} {
		t.Run(fmt.Sprint(flags), func(t *testing.T) {
			c := qfliaContext(t, `(set-logic QF_UFBV)(declare-const p Bool)`)
			m, err := c.Manager()
			qfaufliaMust(t, err)
			config := ackrConfig(t, c)
			if flags[0] {
				qfaufliaMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			r, err := smtlogics.NewQFUFBVAckrCoreTactic(m, nil, config, io.Discard)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, r.Close()) })
			goal, err := tactic.NewGoal(m, flags[0], true, flags[1])
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
			prefix, err := tactic.NewGoal(m, flags[0], true, false)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, prefix.Close()) })
			var result tactic.TacticResult
			t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
			qfaufliaMust(t, result.BindManager(m))
			qfaufliaMust(t, result.AddGoal(prefix))
			filter, err := tactic.NewFilterModel(m)
			qfaufliaMust(t, err)
			qfaufliaMust(t, result.SetModelConverter(filter))
			p, err := c.MakeApp("p", nil, nil, ast.ASTHandle{})
			qfaufliaMust(t, err)
			leaf, err := m.DependencyLeaf(p)
			qfaufliaMust(t, err)
			qfaufliaMust(t, result.SetCore(leaf))
			err = r.Apply(goal, &result)
			want := "qfufbv_ackr does not support proof production"
			if flags[1] {
				want = "qfufbv_ackr does not support unsat core production"
			}
			if err == nil || err.Error() != want {
				t.Fatal("source refusal order", err, want)
			}
			if result.ModelConverter != nil || result.Core != leaf || len(result.Goals) != 1 || result.Goals[0] != prefix {
				t.Fatal("raw tactic changed unrelated result metadata")
			}
		})
	}
}

func TestQFUFBVAckrUnknownLeavesNoResultGoal(t *testing.T) {
	c := qfliaContext(t, `(set-logic QF_UFBV)(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))(declare-fun f ((_ BitVec 4)) (_ BitVec 4))(assert (bvult x y))(assert (bvult (f x) (f y)))`)
	m, err := c.Manager()
	qfaufliaMust(t, err)
	config := ackrConfig(t, c)
	qfaufliaMust(t, config.Globals.SourceSetName("sat.dimacs.display", "true"))
	p := ackrSettings(t, config, "incsat", true)
	var output strings.Builder
	r, err := smtlogics.NewQFUFBVAckrCoreTactic(m, p, config, &output)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, r.Close()) })
	qfaufliaMust(t, r.UpdateParams(p))
	goal, err := tactic.NewGoal(m, false, true, false)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
	qfaufliaMust(t, c.AssertGoal(goal))
	var result tactic.TacticResult
	t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
	qfaufliaMust(t, r.Apply(goal, &result))
	if len(result.Goals) != 0 || result.ModelConverter != nil {
		t.Fatal("undefined solve published a goal/model")
	}
	if strings.Count(output.String(), "p cnf ") != 2 {
		t.Fatal("eager source unknown must still reach the second check", output.String())
	}
}

type closedAckrTraceWriter struct{}

func (closedAckrTraceWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestQFUFBVAckrTraceAndClosedStream(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(fmt.Sprint(closed), func(t *testing.T) {
			c := qfliaContext(t, `(set-logic QF_UFBV)(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))(declare-fun f ((_ BitVec 4)) (_ BitVec 4))(assert (bvult x y))(assert (bvult (f x) (f y)))`)
			m, err := c.Manager()
			qfaufliaMust(t, err)
			config := ackrConfig(t, c)
			var output bytes.Buffer
			var writer io.Writer = &output
			if closed {
				writer = closedAckrTraceWriter{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("qfufbv_ackr_tactic")
			config.ModelEvaluator.Trace = tags
			p := ackrSettings(t, config, "incsat", false)
			r, err := smtlogics.NewQFUFBVAckrCoreTactic(m, p, config, io.Discard)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, r.Close()) })
			qfaufliaMust(t, r.UpdateParams(p))
			goal, err := tactic.NewGoal(m, false, true, false)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
			qfaufliaMust(t, c.AssertGoal(goal))
			var result tactic.TacticResult
			t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
			qfaufliaMust(t, r.Apply(goal, &result))
			if !result.DecidedSat() {
				t.Fatal("trace changed tactic result")
			}
			if !closed && tags.ShouldTrace("qfufbv_ackr_tactic") {
				if !strings.Contains(output.String(), "goal:\n") || !strings.Contains(output.String(), "qfufbv_tactic.cpp:67") {
					t.Fatal("missing source raw tactic trace", output.String())
				}
			}
		})
	}
}
