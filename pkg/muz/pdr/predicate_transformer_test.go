package pdr_test

// Source: src/muz/pdr/pdr_context.cpp predicate-transformer signature,
// frame-property, reachability-cache and propagation operations.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPredicateTransformerSignatureFramesAndReachability(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 2)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "transformer", []ast.ASTHandle{boolean, boolean}, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableHashCache)
	pdrMust(t, err)
	signature, err := transformer.Signature()
	pdrMust(t, err)
	if len(signature) != 2 {
		t.Fatal("signature size", len(signature))
	}
	for index, declaration := range signature {
		old, x := owner.IsOldPredicate(declaration, 0)
		pdrMust(t, x)
		if !old {
			t.Fatal("signature is not old-state zero", index)
		}
		rangeSort, x := manager.GetRange(declaration)
		pdrMust(t, x)
		if rangeSort != boolean {
			t.Fatal("signature range", index, rangeSort)
		}
	}

	p := pdrBool(t, manager, "transformer_p")
	q := pdrBool(t, manager, "transformer_q")
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	pdrMust(t, transformer.InitializeSystem(p, truth))
	if initial, x := transformer.InitialState(); x != nil || initial != p {
		t.Fatal("initial state", initial, x)
	}
	if transition, x := transformer.Transition(); x != nil || transition != truth {
		t.Fatal("transition", transition, x)
	}

	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	result, err := transformer.CheckReachable(0, notP, true, false)
	pdrMust(t, err)
	if result.Status != lbool.False {
		t.Fatal("initial frame did not enforce initial state", result.Status)
	}
	result, err = transformer.CheckReachable(1, notP, false, true)
	pdrMust(t, err)
	if result.Status != lbool.True || result.Model == nil {
		t.Fatal("initial state leaked into later frame", result.Status, result.Model)
	}
	pdrMust(t, result.Model.DecRef())

	changed, err := transformer.AddProperty(q, 1)
	pdrMust(t, err)
	if !changed {
		t.Fatal("new frame property was ignored")
	}
	changed, err = transformer.AddProperty(q, 1)
	pdrMust(t, err)
	if changed {
		t.Fatal("duplicate frame property was added")
	}
	notQ, err := manager.MkNot(q)
	pdrMust(t, err)
	// notQ is reused across checks, so the test owns a reference to it.
	notQRef, err := manager.NewRef(notQ)
	pdrMust(t, err)
	result, err = transformer.CheckReachable(1, notQ, false, false)
	pdrMust(t, err)
	if result.Status != lbool.False {
		t.Fatal("frame property was not active", result.Status)
	}
	result, err = transformer.CheckReachable(2, notQ, false, false)
	pdrMust(t, err)
	if result.Status != lbool.True {
		t.Fatal("frame property was active above its level", result.Status)
	}
	changed, err = transformer.AddProperty(q, 2)
	pdrMust(t, err)
	if !changed {
		t.Fatal("property was not moved to stronger frame")
	}
	result, err = transformer.CheckReachable(2, notQ, false, false)
	pdrMust(t, err)
	if result.Status != lbool.False {
		t.Fatal("moved frame property was not active", result.Status)
	}

	pdrMust(t, transformer.CloseState(q))
	if found, x := transformer.IsCachedReachable(q); x != nil || !found {
		t.Fatal("closed state was not cached", found, x)
	}
	stats, err := transformer.Statistics()
	pdrMust(t, err)
	if stats.Properties != 2 {
		t.Fatal("frame property accounting", stats)
	}

	pdrMust(t, manager.DelRef(&notQRef))
	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}

func TestPredicateTransformerPropagatesGlobalLemmaToInfinity(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 2)
	q := pdrBool(t, manager, "propagate_q")
	pdrMust(t, owner.SetBackground(q))
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "propagate", nil, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableConstraintCache)
	pdrMust(t, err)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	pdrMust(t, transformer.InitializeSystem(truth, truth))
	changed, err := transformer.AddProperty(q, 0)
	pdrMust(t, err)
	if !changed {
		t.Fatal("level-zero lemma was ignored")
	}
	empty, err := transformer.PropagateToNextLevel(0)
	pdrMust(t, err)
	if !empty {
		t.Fatal("globally implied lemma was not propagated")
	}
	notQ, err := manager.MkNot(q)
	pdrMust(t, err)
	for _, level := range []uint32{0, 2} {
		result, x := transformer.CheckReachable(level, notQ, false, false)
		pdrMust(t, x)
		if result.Status != lbool.False {
			t.Fatal("invariant was not active", level, result.Status)
		}
	}
	stats, err := transformer.Statistics()
	pdrMust(t, err)
	if stats.Propagations != 1 || stats.Properties != 1 {
		t.Fatal("propagation accounting", stats)
	}
	if found, x := transformer.IsCachedReachable(q); x != nil || !found {
		t.Fatal("initial reachable constraint was not installed", found, x)
	}
	if found, x := transformer.IsCachedReachable(notQ); x != nil || found {
		t.Fatal("inconsistent reachable constraint accepted", found, x)
	}

	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}

func TestPredicateTransformerCheckInductiveReturnsSubsetCore(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "inductive_core", nil, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableHashCache)
	pdrMust(t, err)
	p := pdrBool(t, manager, "inductive_p")
	q := pdrBool(t, manager, "inductive_q")
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	pdrMust(t, transformer.InitializeSystem(notP, truth))
	inductive, assumesLevel, core, err := transformer.CheckInductive(0, []ast.ASTHandle{p, q})
	pdrMust(t, err)
	if !inductive || !assumesLevel || len(core) != 1 || core[0] != p {
		t.Fatal("inductive subset core", inductive, assumesLevel, core)
	}
	if _, err = manager.GetKind(core[0]); err != nil {
		t.Fatal("returned borrowed core lost its input lifetime", err)
	}
	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}

func TestPredicateTransformerInductiveCoreKeepsUnrootedInputUntilNextCheck(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "inductive_unrooted_core", nil, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableNoCache)
	pdrMust(t, err)
	p := pdrBool(t, manager, "inductive_unrooted_p")
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	pdrMust(t, transformer.InitializeSystem(p, truth))
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	inductive, _, core, err := transformer.CheckInductive(0, []ast.ASTHandle{notP})
	pdrMust(t, err)
	if !inductive || len(core) != 1 || core[0] != notP {
		t.Fatal("unexpected core", core)
	}
	isNot, inner, err := manager.IsNot(core[0])
	pdrMust(t, err)
	if !isNot || inner != p {
		t.Fatal("core input was not retained", core[0])
	}
	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
