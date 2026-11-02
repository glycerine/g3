package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestContextResetDiscardsFramesAndAllowsReinitialization(t *testing.T) {
	f := newValidationBatchFixture(t, "reset-program")
	prior, found, err := f.context.Transformer(f.declaration)
	pdrMust(t, err)
	if !found {
		t.Fatal("fixture predicate missing")
	}
	status, err := f.context.Solve(f.declaration)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("fixture fact is unreachable", status)
	}
	answer, err := f.context.Answer(f.declaration)
	pdrMust(t, err)
	defer func() { pdrMust(t, f.manager.DelRef(&answer)) }()
	falsity, err := f.manager.MkFalse()
	pdrMust(t, err)
	pdrMust(t, f.context.AddCover(2, f.declaration, falsity))
	pdrMust(t, f.context.Reset())
	pdrMust(t, f.context.Reset())
	if _, err := prior.Head(); err == nil {
		t.Fatal("reset left the original transformer open")
	}
	status, err = f.context.LastResult()
	pdrMust(t, err)
	if status != lbool.Undef {
		t.Fatal("reset preserved a stale result", status)
	}
	if _, err := f.context.Answer(f.declaration); err == nil {
		t.Fatal("reset exposed the previous answer")
	}
	if _, err := f.manager.RefGet(answer); err != nil {
		t.Fatal("reset invalidated the caller's owning answer", err)
	}
	if _, err := f.context.Model(f.declaration); err == nil {
		t.Fatal("reset exposed the previous model")
	}
	if _, err := f.context.Solve(f.declaration); err == nil {
		t.Fatal("reset allowed solving without an installed program")
	}
	levels, err := f.context.NumLevels(f.declaration)
	pdrMust(t, err)
	if levels != 0 {
		t.Fatal("reset preserved old frames", levels)
	}
	cover, err := f.context.CoverDelta(2, f.declaration)
	pdrMust(t, err)
	handle, err := f.manager.RefGet(cover)
	pdrMust(t, err)
	if !f.manager.IsTrue(handle) {
		t.Fatal("reset preserved an old cover")
	}
	pdrMust(t, f.manager.DelRef(&cover))
	pdrMust(t, f.context.SetBackground(falsity))
	pdrMust(t, f.context.Initialize(f.rules))
	status, err = f.context.Solve(f.declaration)
	pdrMust(t, err)
	if status != lbool.False {
		t.Fatal("reinitialization ignored its new background", status)
	}
	pdrMust(t, f.context.Reset())
	truth, err := f.manager.MkTrue()
	pdrMust(t, err)
	pdrMust(t, f.context.SetBackground(truth))
	pdrMust(t, f.context.Initialize(f.rules))
	status, err = f.context.Solve(f.declaration)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("reinitialization reused a stale false background", status)
	}
}

func TestContextResetReselectsArithmeticAndProofMode(t *testing.T) {
	f := newValidationBatchFixture(t, "reset-arithmetic")
	pdrMust(t, f.context.Reset())
	arithmetic, err := f.manager.RegisterArithmeticPlugin()
	pdrMust(t, err)
	real, err := f.manager.MkFamilySort(arithmetic, 0, nil)
	pdrMust(t, err)
	boolean, err := f.manager.MkBoolSort()
	pdrMust(t, err)
	declaration := pdrFunction(t, f.manager, "reset-real-predicate", []ast.ASTHandle{real}, boolean)
	pdrMust(t, f.ruleManager.RegisterPredicate(declaration))
	zero, err := f.manager.ArithmeticRealNumeralText("0")
	pdrMust(t, err)
	head, err := f.manager.MkApp(declaration, []ast.ASTHandle{zero})
	pdrMust(t, err)
	set, err := base.NewRuleSet(f.ruleManager)
	pdrMust(t, err)
	defer func() { pdrMust(t, set.Close()) }()
	rule, err := f.ruleManager.NewRule(head, nil, nil, "reset-real-fact")
	pdrMust(t, err)
	pdrMust(t, set.Add(rule))
	pdrMust(t, rule.Close())
	proofsBefore, err := f.manager.ProofsEnabled()
	pdrMust(t, err)
	pdrMust(t, f.context.Initialize(set))
	difference, err := f.context.IsDifferenceLogic()
	pdrMust(t, err)
	if !difference {
		t.Fatal("new arithmetic program did not select difference logic")
	}
	status, err := f.context.Solve(declaration)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("reinitialized arithmetic fact is unreachable", status)
	}
	pdrMust(t, f.context.Reset())
	pdrMust(t, f.context.Initialize(f.rules))
	difference, err = f.context.IsDifferenceLogic()
	pdrMust(t, err)
	if difference {
		t.Fatal("Boolean program inherited obsolete arithmetic selection")
	}
	status, err = f.context.Solve(f.declaration)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("reinitialized Boolean fact is unreachable", status)
	}
	proofsAfter, err := f.manager.ProofsEnabled()
	pdrMust(t, err)
	if proofsAfter != proofsBefore {
		t.Fatal("reset/reinitialization changed caller proof mode")
	}
}
