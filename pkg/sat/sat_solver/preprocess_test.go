package sat_solver_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/sat"
	incsat "github.com/glycerine/g3/pkg/sat/sat_solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func incMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type encoderFixture struct {
	context  *cmd_context.Context
	manager  *ast.ASTManager
	services model.ModelEvaluatorConfig
	params   uparams.ParamsRef
	core     *sat.Solver
	encoder  *incsat.IncrementalEncoder
}

func newEncoderFixture(t *testing.T, construct bool) *encoderFixture {
	t.Helper()
	c, err := cmd_context.NewStandaloneContext()
	incMust(t, err)
	f := &encoderFixture{context: c}
	t.Cleanup(func() {
		incMust(t, c.Close())
		if f.manager != nil && f.manager.Budget().Used() != 0 {
			t.Errorf("incremental encoder retained %d bytes", f.manager.Budget().Used())
		}
	})
	incMust(t, smt2.ParseCommands(c, strings.NewReader(`(set-logic ALL)(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))(declare-const p Bool)(declare-const q Bool)(declare-const r Bool)(declare-const s Bool)(declare-const v Bool)`), io.Discard))
	f.manager, err = c.Manager()
	incMust(t, err)
	g, err := c.GlobalParameters()
	incMust(t, err)
	previous, err := uparams.BindProcessGlobalParamsService(g)
	incMust(t, err)
	t.Cleanup(func() { _, err := uparams.BindProcessGlobalParamsService(previous); incMust(t, err) })
	coop, err := cooperate.NewCooperation(f.manager.Budget())
	incMust(t, err)
	t.Cleanup(func() { incMust(t, coop.Close()) })
	f.services = model.ModelEvaluatorConfig{Context: g.Context, Globals: g, Memory: f.manager.NumeralService().RuntimeOwner(), Cooperation: coop}
	f.params, err = g.Context.NewValue()
	incMust(t, err)
	t.Cleanup(func() { incMust(t, g.Context.Del(&f.params)) })
	key := "elim_vars"
	incMust(t, g.Context.SetBoolName(&f.params, &key, false))
	runtime, err := sat.NewRuntime(sat.RuntimeConfig{Budget: f.manager.Budget(), ParamsContext: g.Context})
	incMust(t, err)
	t.Cleanup(func() { incMust(t, runtime.Close()) })
	f.core, err = sat.NewSATSolver(sat.SolverConfig{ParamsContext: g.Context, Params: f.params, RLimit: f.manager.ResourceLimit(), Budget: f.manager.Budget(), Runtime: runtime})
	incMust(t, err)
	t.Cleanup(func() { incMust(t, f.core.Close()) })
	if construct {
		f.encoder, err = incsat.NewIncrementalEncoder(f.manager, f.core, &f.params, f.services)
		if f.encoder != nil {
			t.Cleanup(func() { incMust(t, f.encoder.Close()) })
		}
		incMust(t, err)
	}
	return f
}
func (f *encoderFixture) goal(t *testing.T, forms string) *tactic.Goal {
	t.Helper()
	incMust(t, f.context.Push(1))
	incMust(t, smt2.ParseCommands(f.context, strings.NewReader(forms), io.Discard))
	g, err := tactic.NewGoal(f.manager, false, true, false)
	incMust(t, err)
	t.Cleanup(func() { incMust(t, g.Close()) })
	incMust(t, f.context.AssertGoal(g))
	incMust(t, f.context.Pop(1))
	return g
}
func (f *encoderFixture) compile(t *testing.T, forms string) {
	t.Helper()
	incMust(t, f.core.PopToBaseLevel())
	status, err := f.encoder.Compile(f.goal(t, forms))
	incMust(t, err)
	if status != lbool.True {
		t.Fatal("formula was not encoded", status)
	}
}

func TestIncrementalEncoderPersistentBitBindingsAndUserScopesCPP(t *testing.T) {
	f := newEncoderFixture(t, true)
	var script strings.Builder
	script.WriteString(`(set-logic ALL)(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))`)
	var answers []string
	check := func(want lbool.LBool) {
		t.Helper()
		status, err := f.core.Check(nil)
		incMust(t, err)
		if status != want {
			t.Fatal("incremental SAT status", status, want)
		}
		answers = append(answers, map[lbool.LBool]string{lbool.True: "sat", lbool.False: "unsat"}[status])
		script.WriteString(`(check-sat-using (then card2bv (using-params simplify :som true :pull_cheap_ite true :push_ite_bv false :local_ctx true :local_ctx_limit 10000000 :flat true :hoist_mul false :elim_and true :blast_distinct true) max-bv-sharing bit-blast (using-params simplify :som true :pull_cheap_ite true :push_ite_bv false :local_ctx true :local_ctx_limit 10000000 :flat true :hoist_mul false :elim_and true :blast_distinct true) sat))`)
	}
	assert := func(form string) { t.Helper(); f.compile(t, form); script.WriteString(form) }
	assert(`(assert (= x #x1))`)
	check(lbool.True)
	x, err := f.context.MakeApp("x", nil, nil, ast.ASTHandle{})
	incMust(t, err)
	xd, err := f.manager.GetDecl(x)
	incMust(t, err)
	bits, _, err := f.encoder.FindBitVectorBinding(xd)
	incMust(t, err)
	if bits.IsNull() {
		t.Fatal("missing persistent x bit binding")
	}
	incMust(t, f.encoder.Push())
	script.WriteString(`(push 1)`)
	assert(`(assert (= x #x2))`)
	check(lbool.False)
	incMust(t, f.encoder.Pop(1))
	script.WriteString(`(pop 1)`)
	incMust(t, f.encoder.Push())
	script.WriteString(`(push 1)`)
	assert(`(assert (= y #x4))`)
	check(lbool.True)
	binding := func(d ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		h, _, err := f.encoder.FindBitVectorBinding(d)
		incMust(t, err)
		return h
	}
	y, err := f.context.MakeApp("y", nil, nil, ast.ASTHandle{})
	incMust(t, err)
	yd, err := f.manager.GetDecl(y)
	incMust(t, err)
	if binding(xd) != bits || binding(yd).IsNull() {
		t.Fatal("base mapping was rebuilt or scoped mapping missing")
	}
	incMust(t, f.encoder.Pop(10)) // Source clamps over-pops to the current level.
	script.WriteString(`(pop 1)`)
	if binding(xd) != bits || !binding(yd).IsNull() {
		t.Fatal("pop discarded base bits or retained scoped bits")
	}
	assert(`(assert (= y #x3))`)
	check(lbool.True)
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = strings.NewReader(script.String())
		out, err := cmd.CombinedOutput()
		incMust(t, err)
		if strings.Join(strings.Fields(string(out)), " ") != strings.Join(answers, " ") {
			t.Fatal("C++ scoped preprocessing mismatch", string(out), answers)
		}
	}
}

func TestIncrementalEncoderPBAndUnsupportedInterpretedAtoms(t *testing.T) {
	f := newEncoderFixture(t, true)
	f.compile(t, `(assert ((_ at-most 2) p q r s v))(assert ((_ at-least 2) p q r s v))`)
	status, err := f.core.Check(nil)
	incMust(t, err)
	if status != lbool.True {
		t.Fatal("PB network", status)
	}
	incMust(t, f.encoder.Push())
	f.compile(t, `(assert p)(assert q)(assert r)`)
	status, err = f.core.Check(nil)
	incMust(t, err)
	if status != lbool.False {
		t.Fatal("PB network did not retain its earlier clauses", status)
	}
	incMust(t, f.encoder.Pop(1))
	incMust(t, f.core.PopToBaseLevel())
	// Bit-vector UF applications survive preprocessing as interpreted atoms;
	// source internalize_goal must refuse to claim a propositional encoding.
	incMust(t, smt2.ParseCommands(f.context, strings.NewReader(`(declare-fun h ((_ BitVec 4)) Bool)`), io.Discard))
	status, err = f.encoder.Compile(f.goal(t, `(assert (h x))`))
	incMust(t, err)
	if status != lbool.Undef {
		t.Fatal("unhandled interpreted atom was accepted as pure SAT", status)
	}
}

func TestIncrementalEncoderAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "compile", "recompile"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 4096 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f := newEncoderFixture(t, phase != "constructor")
					if phase == "recompile" {
						f.compile(t, `(assert (= x #x1))`)
					}
					g := f.goal(t, `(assert (= (bvadd x y) #x3))`)
					f.manager.Budget().FailAfter(stop)
					var err error
					if phase == "constructor" {
						f.encoder, err = incsat.NewIncrementalEncoder(f.manager, f.core, &f.params, f.services)
						f.manager.Budget().FailAfter(-1)
						if f.encoder != nil {
							incMust(t, f.encoder.Close())
						}
					} else {
						var status lbool.LBool
						status, err = f.encoder.Compile(g)
						f.manager.Budget().FailAfter(-1)
						if err == nil && status != lbool.True {
							t.Fatal("successful frontier did not finish compilation", status)
						}
					}
					if err == nil {
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
				})
				if t.Failed() {
					t.Fatal("reservation frontier", phase, stop)
				}
			}
			if !success {
				t.Fatal("no successful reservation frontier", phase)
			}
		})
	}
}

func TestIncrementalEncoderProofRefusalAndTacticExceptionScopeRecovery(t *testing.T) {
	t.Run("proof-refusal", func(t *testing.T) {
		f := newEncoderFixture(t, true)
		incMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
		g, err := tactic.NewGoal(f.manager, true, true, false)
		incMust(t, err)
		t.Cleanup(func() { incMust(t, g.Close()) })
		_, err = f.encoder.Compile(g)
		var refusal z3exception.DefaultException
		if !errors.As(err, &refusal) || err.Error() != "generation of proof objects is not supported in this mode" {
			t.Fatal("source proof refusal", err)
		}
	})
	// Reach the source tactic-exception handler after cardinality rewriting,
	// rather than replacing it with cancellation at an earlier phase.
	reached := false
	for fuel := uint32(1); fuel <= 120 && !reached; fuel++ {
		t.Run(fmt.Sprint("fuel/", fuel), func(t *testing.T) {
			f := newEncoderFixture(t, true)
			incMust(t, f.encoder.Push())
			g := f.goal(t, `(assert (= (bvadd x y) #x3))`)
			incMust(t, f.manager.ResourceLimit().Push(fuel))
			status, err := f.encoder.Compile(g)
			f.manager.ResourceLimit().Pop()
			if err != nil || status != lbool.Undef {
				return
			}
			bindings, err := f.encoder.BitVectorBindings()
			incMust(t, err)
			if bindings != nil {
				return
			}
			reached = true
			// Reinitialization must push the new bit blaster to the existing
			// SAT/atom scope depth before processing the next goal.
			f.compile(t, `(assert (= x #x1))`)
			incMust(t, f.encoder.Pop(1))
			level, err := f.encoder.ScopeLevel()
			incMust(t, err)
			if level != 0 {
				t.Fatal("scope recovery", level)
			}
			f.compile(t, `(assert (= x #x2))`)
			status, err = f.core.Check(nil)
			incMust(t, err)
			if status != lbool.True {
				t.Fatal("refused scoped assertion escaped after rebuild/pop", status)
			}
		})
	}
	if !reached {
		t.Fatal("did not reach source preprocessing tactic-exception recovery")
	}
}
