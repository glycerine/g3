package cmd_context

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type commandExceptionTactic struct {
	tactic.Tactic
	failure error
}

func (t *commandExceptionTactic) Apply(*tactic.Goal, *tactic.TacticResult) error { return t.failure }

func TestCommandContextTacticExceptionBoundary(t *testing.T) {
	for _, entry := range []struct {
		name    string
		failure error
		caught  bool
	}{
		{"source", fmt.Errorf("Go operation context: %w", z3exception.NewDefaultException("rewriter stopped")), true},
		{"fatal", z3exception.MustZ3Error(z3exception.ErrorInternalFatal), false},
		{"owner", arena.ErrInvalidSlot, false},
	} {
		t.Run(entry.name, func(t *testing.T) {
			c := contextFixture(t)
			selected := &commandExceptionTactic{Tactic: tactic.NewSkipTactic(), failure: entry.failure}
			status, err := c.CheckSatUsing(selected, nil)
			var command TacticCommandFailure
			if errors.As(err, &command) != entry.caught || status != lbool.Undef || !errors.Is(err, entry.failure) {
				t.Fatal("exception classification", status, err)
			}
			if entry.caught {
				if command.Error() != "rewriter stopped" {
					t.Fatal("source message gained Go context", command.Error())
				}
				reason, err := c.ReasonUnknown()
				contextMust(t, err)
				if reason != "rewriter stopped" {
					t.Fatal("caught result reason", reason)
				}
			}
		})
	}
}

func TestCommandContextRegistersClausePreprocessingTactics(t *testing.T) {
	c := contextFixture(t)
	for _, name := range []string{"occf", "split-clause"} {
		selected, err := c.NewBuiltinTactic(name)
		contextMust(t, err)
		if selected == nil {
			t.Fatal("nil registered tactic", name)
		}
		contextMust(t, selected.IncRef())
		contextMust(t, selected.DecRef())
	}
}

func TestCommandContextTacticResultsScopesAndModelLifetime(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_UF"))
	m, err := c.Manager()
	contextMust(t, err)
	b, err := m.MkBoolSort()
	contextMust(t, err)
	decl, err := c.DeclareFunction("tactic-session-a", nil, b)
	contextMust(t, err)
	a, err := c.MakeApp("tactic-session-a", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(a, ""))
	selected, err := c.NewBuiltinTactic("smt")
	contextMust(t, err)
	contextMust(t, selected.IncRef())
	t.Cleanup(func() { contextMust(t, selected.DecRef()) })
	options, err := c.config.Params.NewValue()
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, c.config.Params.Del(&options)) })
	timeoutName := "timeout"
	contextMust(t, c.config.Params.SetUIntName(&options, &timeoutName, 1000))
	status, err := c.CheckSatUsing(selected, &options)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("SMT tactic did not solve the current assertions", status)
	}
	if c.tacticResult == nil {
		t.Fatal("command did not own its independent tactic result")
	}
	md, err := c.Model()
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, md.DecRef()) })
	value, err := md.ConstantInterpretation(decl)
	contextMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("tactic result lost its model", value)
	}
	contextMust(t, c.Push(1))
	notA, err := m.MkNot(a)
	contextMust(t, err)
	contextMust(t, c.Assert(notA, ""))
	if _, err = c.Model(); err == nil {
		t.Fatal("assertion left a stale model available")
	}
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.False || c.tacticResult != nil {
		t.Fatal("ordinary check did not replace the tactic result", status)
	}
	value, err = md.ConstantInterpretation(decl)
	contextMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("ordinary check invalidated an acquired model")
	}
	contextMust(t, c.Pop(1))
	failed := tactic.NewFailTactic()
	contextMust(t, failed.IncRef())
	t.Cleanup(func() { contextMust(t, failed.DecRef()) })
	status, err = c.CheckSatUsing(failed, nil)
	contextMust(t, err)
	if status != lbool.Undef {
		t.Fatal("failed tactic did not yield UNKNOWN", status)
	}
	if _, err = c.Model(); err == nil {
		t.Fatal("failed tactic exposed the previous SAT model")
	}
	reason, err := c.tacticResult.ReasonUnknown()
	contextMust(t, err)
	if reason != "fail tactic" {
		t.Fatal("failed tactic reason was not retained", reason)
	}
	status, err = c.CheckSatUsing(selected, nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("tactic check did not recover after failure", status)
	}
	value, err = md.ConstantInterpretation(decl)
	contextMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("later tactic check invalidated an acquired model")
	}
}

func TestCommandContextTacticProofCoreFrontendGuard(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetBoolOption("produce_proofs", true))
	contextMust(t, c.SetBoolOption("produce_unsat_cores", true))
	selected, err := c.NewBuiltinTactic("smt")
	contextMust(t, err)
	contextMust(t, selected.IncRef())
	t.Cleanup(func() { contextMust(t, selected.DecRef()) })
	_, err = c.CheckSatUsing(selected, nil)
	if err == nil || err.Error() != "Frontend does not support simultaneous generation of proofs and unsat cores" {
		t.Fatal("missing native frontend proof/core refusal", err)
	}
	if c.checked || c.tacticResult != nil {
		t.Fatal("frontend refusal installed a check result")
	}
}

func TestCommandContextUnknownTacticLeavesInitializationOptionsAvailable(t *testing.T) {
	c := contextFixture(t)
	if _, err := c.NewBuiltinTactic("not-a-retained-tactic"); err == nil {
		t.Fatal("unknown tactic was accepted")
	}
	contextMust(t, c.SetBoolOption("produce_proofs", true))
	m, err := c.Manager()
	contextMust(t, err)
	enabled, err := m.ProofsEnabled()
	contextMust(t, err)
	if !enabled {
		t.Fatal("invalid tactic prematurely fixed the manager's proof mode")
	}
}

func TestCommandContextAssertGoalPreservesOrderProofsAndNames(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(map[bool]string{false: "cores", true: "proofs"}[proofs], func(t *testing.T) {
			c := contextFixture(t)
			contextMust(t, c.SetBoolOption("produce_proofs", proofs))
			contextMust(t, c.SetBoolOption("produce_unsat_cores", !proofs))
			m, err := c.Manager()
			contextMust(t, err)
			boolean, err := m.MkBoolSort()
			contextMust(t, err)
			var expressions []ast.ASTHandle
			for _, name := range []string{"goal-copy-a", "goal-copy-b"} {
				_, err = c.DeclareFunction(name, nil, boolean)
				contextMust(t, err)
				term, e := c.MakeApp(name, nil, nil, ast.ASTHandle{})
				contextMust(t, e)
				expressions = append(expressions, term)
			}
			contextMust(t, c.Push(1))
			contextMust(t, c.Assert(expressions[0], "goal-copy-tag"))
			contextMust(t, c.Assert(expressions[1], ""))
			originalTag := c.assertions[0].tag
			goal, err := tactic.NewGoal(m, proofs, false, !proofs)
			contextMust(t, err)
			t.Cleanup(func() { contextMust(t, goal.Close()) })
			contextMust(t, c.AssertGoal(goal))
			contextMust(t, c.Pop(1))
			size, err := goal.Size()
			contextMust(t, err)
			if size != 2 {
				t.Fatal("goal did not retain both copied assertions", size)
			}
			for i, want := range expressions {
				got, e := goal.Form(uint32(i))
				contextMust(t, e)
				if got != want {
					t.Fatal("assertion order changed", i, got, want)
				}
				if proofs {
					proof, e := goal.Proof(uint32(i))
					contextMust(t, e)
					fact, e := m.GetProofFact(proof)
					contextMust(t, e)
					if fact != want {
						t.Fatal("copied proof does not assert its original formula", i)
					}
				} else {
					dep, e := goal.Dependency(uint32(i))
					contextMust(t, e)
					leaves, e := m.DependencyLinearize(dep, nil)
					contextMust(t, e)
					if i == 0 && (len(leaves) != 1 || leaves[0] != originalTag) {
						t.Fatal("named assertion lost its dependency", leaves)
					}
					if i == 1 && len(leaves) != 0 {
						t.Fatal("unnamed assertion acquired a dependency", leaves)
					}
				}
			}
		})
	}
}
