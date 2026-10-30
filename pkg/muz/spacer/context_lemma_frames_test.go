package spacer_test

// Source: src/muz/spacer/spacer_context.cpp lemma and
// pred_transformer::frames.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type frameTransformerFixture struct {
	checked []ast.ASTHandle
	ensured []uint32
	added   []*spacer.Lemma
	levels  map[ast.ASTHandle]uint32
}

func (f *frameTransformerFixture) EnsureLevel(level uint32) error {
	f.ensured = append(f.ensured, level)
	return nil
}

func (f *frameTransformerFixture) AddLemmaCore(lemma *spacer.Lemma) error {
	f.added = append(f.added, lemma)
	return nil
}

func (f *frameTransformerFixture) IsInvariant(_ uint32, formula ast.ASTHandle) (bool, uint32, error) {
	f.checked = append(f.checked, formula)
	level, found := f.levels[formula]
	return found, level, nil
}

func lemmaOwnerFixture(t *testing.T) (*cmd_context.Context, *ast.ASTManager, *spacer.Manager) {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	owner, err := spacer.NewManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	spacerMust(t, err)
	return command, manager, owner
}

func TestSpacerLemmaNormalizesBodyAndBuildsCube(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	p := spacerBool(t, manager, "lemma-cube-p")
	q := spacerBool(t, manager, "lemma-cube-q")
	conjunction, err := manager.MkAnd(p, q)
	spacerMust(t, err)
	clause, err := manager.PushNot(conjunction)
	spacerMust(t, err)
	lemma, err := spacer.NewLemma(owner, clause, 3)
	spacerMust(t, err)
	cube, err := lemma.Cube()
	spacerMust(t, err)
	if len(cube) != 2 || cube[0] == cube[1] {
		t.Fatal("lemma cube", cube)
	}
	foundP, foundQ := false, false
	for _, literal := range cube {
		foundP = foundP || literal == p
		foundQ = foundQ || literal == q
	}
	if !foundP || !foundQ {
		t.Fatal("lemma cube literals", foundP, foundQ)
	}
	if level, err := lemma.Level(); err != nil || level != 3 {
		t.Fatal("lemma level", level, err)
	}
	spacerMust(t, lemma.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerFramesMergePromoteAndPropagate(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	p := spacerBool(t, manager, "frames-p")
	q := spacerBool(t, manager, "frames-q")
	fixture := &frameTransformerFixture{levels: make(map[ast.ASTHandle]uint32)}
	frames, err := spacer.NewFrames(manager, fixture)
	spacerMust(t, err)
	first, err := spacer.NewLemma(owner, p, 1)
	spacerMust(t, err)
	added, err := frames.AddLemma(first)
	spacerMust(t, err)
	if !added || len(fixture.added) != 1 {
		t.Fatal("first lemma", added, len(fixture.added))
	}
	duplicate, err := spacer.NewLemma(owner, p, 3)
	spacerMust(t, err)
	added, err = frames.AddLemma(duplicate)
	spacerMust(t, err)
	if !added || len(fixture.added) != 2 {
		t.Fatal("promoted lemma", added, len(fixture.added))
	}
	if got, err := frames.FrameLemmas(3); err != nil || len(got) != 1 || got[0] != p {
		t.Fatal("promoted frame", got, err)
	}
	second, err := spacer.NewLemma(owner, q, 3)
	spacerMust(t, err)
	added, err = frames.AddLemma(second)
	spacerMust(t, err)
	if !added {
		t.Fatal("second lemma not added")
	}
	fixture.levels[p] = 5
	all, err := frames.PropagateToNextLevel(3)
	spacerMust(t, err)
	if all || len(fixture.ensured) != 1 || fixture.ensured[0] != 4 {
		t.Fatal("partial propagation", all, fixture.ensured)
	}
	if got, err := frames.FrameLemmas(5); err != nil || len(got) != 1 || got[0] != p {
		t.Fatal("propagated frame", got, err)
	}
	spacerMust(t, frames.PropagateToInfinity(3))
	if got, err := frames.FrameLemmas(spacer.InfinityLevel); err != nil || len(got) != 2 {
		t.Fatal("infinity frame", got, err)
	}
	spacerMust(t, frames.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerFramesSimplifySameLevelLemmasByUnitPropagation(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	a := spacerBool(t, manager, "frames-simplify-a")
	b := spacerBool(t, manager, "frames-simplify-b")
	weak, err := manager.MkOr(a, b)
	spacerMust(t, err)
	fixture := &frameTransformerFixture{levels: make(map[ast.ASTHandle]uint32)}
	frames, err := spacer.NewFrames(manager, fixture)
	spacerMust(t, err)
	weakLemma, err := spacer.NewLemma(owner, weak, 2)
	spacerMust(t, err)
	strongLemma, err := spacer.NewLemma(owner, a, 2)
	spacerMust(t, err)
	otherLevel, err := spacer.NewLemma(owner, weak, 3)
	spacerMust(t, err)
	_, err = frames.AddLemma(weakLemma)
	spacerMust(t, err)
	_, err = frames.AddLemma(strongLemma)
	spacerMust(t, err)
	_, err = frames.AddLemma(otherLevel)
	spacerMust(t, err)
	spacerMust(t, frames.Simplify(owner))
	if got, err := frames.FrameLemmas(2); err != nil || len(got) != 1 || got[0] != a {
		t.Fatal("simplified frame", got, err)
	}
	if got, err := frames.FrameLemmas(3); err != nil || len(got) != 1 || got[0] != weak {
		t.Fatal("independent level", got, err)
	}
	spacerMust(t, frames.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerLemmaInstantiatesBindingsInSourceOrder(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	v0, err := manager.MkVar(0, boolean)
	spacerMust(t, err)
	v1, err := manager.MkVar(1, boolean)
	spacerMust(t, err)
	notV1, err := manager.MkNot(v1)
	spacerMust(t, err)
	body, err := manager.MkAnd(v0, notV1)
	spacerMust(t, err)
	quantifier, err := manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{boolean, boolean},
		Names:  []usymbol.Symbol{usymbol.Numerical(0), usymbol.Numerical(1)},
		Body:   body,
	})
	spacerMust(t, err)
	lemma, err := spacer.NewLemma(owner, quantifier, 1)
	spacerMust(t, err)
	p := spacerBool(t, manager, "lemma-binding-p")
	q := spacerBool(t, manager, "lemma-binding-q")
	spacerMust(t, lemma.AddBindings([]ast.ASTHandle{p, q}))
	instances, err := lemma.Instances()
	spacerMust(t, err)
	if len(instances) != 1 {
		t.Fatal("lemma instance count", len(instances))
	}
	instance, err := manager.RefGet(instances[0])
	spacerMust(t, err)
	notQ, err := manager.MkNot(q)
	spacerMust(t, err)
	want, err := manager.MkAnd(p, notQ)
	spacerMust(t, err)
	if instance != want {
		t.Fatal("lemma source-order instance", instance, want)
	}
	spacerMust(t, manager.DelRef(&instances[0]))
	spacerMust(t, lemma.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerPredicateTransformerConnectsFramesAndSolver(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	p := spacerBool(t, manager, "predicate-transformer-p")
	initial, err := manager.MkFalse()
	spacerMust(t, err)
	transformer, err := spacer.NewPredicateTransformer(owner, "predicate-transformer", initial, p, spacer.PropSolverConfig{})
	spacerMust(t, err)
	added, err := transformer.AddLemma(p, 0)
	spacerMust(t, err)
	if !added {
		t.Fatal("predicate-transformer lemma not added")
	}
	all, err := transformer.PropagateToNextLevel(0)
	spacerMust(t, err)
	if !all {
		t.Fatal("transition-implied lemma did not propagate")
	}
	formula, err := transformer.Formulas(spacer.InfinityLevel, false)
	spacerMust(t, err)
	if formula != p {
		t.Fatal("infinity formula", formula, p)
	}
	collected := statistics.New()
	spacerMust(t, transformer.CollectStatistics(collected))
	if collected.Size() == 0 {
		t.Fatal("predicate-transformer statistics are empty")
	}
	spacerMust(t, transformer.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerRulePredicateTransformerBuildsSourceStateSignature(t *testing.T) {
	command, manager, owner := lemmaOwnerFixture(t)
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	name, err := manager.Symbols.Intern("rule-predicate-transformer")
	spacerMust(t, err)
	head, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	transformer, err := spacer.NewRulePredicateTransformer(owner, head, spacer.PropSolverConfig{})
	spacerMust(t, err)
	if got, err := transformer.Head(); err != nil || got != head {
		t.Fatal("rule predicate head", got, head, err)
	}
	signature, err := transformer.Signature()
	spacerMust(t, err)
	if len(signature) != 2 || signature[0] == signature[1] {
		t.Fatal("rule predicate signature", signature)
	}
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	falsehood, err := manager.MkFalse()
	spacerMust(t, err)
	spacerMust(t, transformer.InitializeSystem(falsehood, truth))
	if _, err = transformer.Formulas(0, true); err != nil {
		t.Fatal("initialized rule predicate formulas", err)
	}
	first, err := manager.MkVar(0, boolean)
	spacerMust(t, err)
	second, err := manager.MkVar(1, boolean)
	spacerMust(t, err)
	notSecond, err := manager.MkNot(second)
	spacerMust(t, err)
	property, err := manager.MkAnd(first, notSecond)
	spacerMust(t, err)
	spacerMust(t, transformer.AddCover(-1, property))
	cover, err := transformer.CoverDelta(-1)
	spacerMust(t, err)
	coverHandle, err := manager.RefGet(cover)
	spacerMust(t, err)
	parts, err := manager.FlattenAnd([]ast.ASTHandle{coverHandle})
	spacerMust(t, err)
	if len(parts) != 2 {
		t.Fatal("bound-variable cover", parts)
	}
	seenFirst, seenSecond := false, false
	for _, part := range parts {
		if kind, x := manager.GetKind(part); x == nil && kind == ast.ASTVar {
			index, x := manager.GetVarIndex(part)
			spacerMust(t, x)
			seenFirst = seenFirst || index == 0
			continue
		}
		negated, argument, x := manager.IsNot(part)
		spacerMust(t, x)
		if negated {
			index, x := manager.GetVarIndex(argument)
			spacerMust(t, x)
			seenSecond = seenSecond || index == 1
		}
	}
	if !seenFirst || !seenSecond {
		t.Fatal("cover signature round trip", parts)
	}
	spacerMust(t, manager.DelRef(&cover))
	spacerMust(t, transformer.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}
