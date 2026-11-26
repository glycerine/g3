package smtcore

import (
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTAssertedOrWitnessAcrossScopes(t *testing.T) {
	m := astTestManager(t)
	ctx, p, s := smtInternalizerFixture(t, m, nil)
	terms, literals := benchmarkSMTBooleansForTest(t, m, s, "asserted-or", 5)
	root, err := m.MkOrArray(terms)
	astTestMust(t, err)
	astTestMust(t, s.Assert(root, ast.ASTHandle{}, 0))
	astTestMust(t, s.PushScope())
	astTestMust(t, s.SetAssignment(literals[4].Var(), lbool.True))
	astTestMust(t, p.AssignmentEvent(terms[4], true))
	astTestMust(t, s.SetAssignment(literals[0].Var(), lbool.True))
	astTestMust(t, p.AssignmentEvent(terms[0], true))
	if got := p.RelevantExpressions(); !slices.Equal(got, []ast.ASTHandle{root, terms[4]}) {
		t.Fatalf("an existing true witness should suffice: %v", got)
	}
	astTestMust(t, s.PopScopes(1))
	astTestMust(t, s.SetAssignment(literals[4].Var(), lbool.True))
	astTestMust(t, s.SetAssignment(literals[0].Var(), lbool.True))
	astTestMust(t, p.AssignmentEvent(terms[4], true))
	if got := p.RelevantExpressions(); !slices.Equal(got, []ast.ASTHandle{root, terms[0]}) {
		t.Fatalf("after pop the first true child should be selected: %v", got)
	}
	astTestMust(t, ctx.SetAssignment(root, lbool.False))
	astTestMust(t, p.AssignmentEvent(terms[4], true))
	want := []ast.ASTHandle{root, terms[0], terms[4], terms[3], terms[2], terms[1]}
	if got := p.RelevantExpressions(); !slices.Equal(got, want) {
		t.Fatalf("a false parent marks its remaining children in reverse order: %v", got)
	}
}

func TestSMTAssertedOrUsesCurrentBinding(t *testing.T) {
	for _, mode := range []string{"unbound", "bound", "detached", "rebound"} {
		t.Run(mode, func(t *testing.T) {
			m := astTestManager(t)
			ctx, p, s := smtInternalizerFixture(t, m, nil)
			terms, literals := benchmarkSMTBooleansForTest(t, m, s, "signed-root", 2)
			negated, err := m.MkNot(terms[0])
			astTestMust(t, err)
			root, err := m.MkOr(negated, terms[1])
			astTestMust(t, err)
			astTestMust(t, s.Assert(root, ast.ASTHandle{}, 0))
			if mode == "bound" {
				astTestMust(t, s.Internalize(root, true))
			}
			if mode == "detached" || mode == "rebound" {
				astTestMust(t, s.SetRelevancy(nil))
				t.Cleanup(func() { astTestMust(t, s.SetRelevancy(p)) })
				if mode == "rebound" {
					astTestMust(t, s.SetRelevancy(p))
				}
			}
			astTestMust(t, s.SetAssignment(literals[0].Var(), lbool.True))
			astTestMust(t, ctx.SetAssignment(terms[0], lbool.False))
			astTestMust(t, s.SetAssignment(literals[1].Var(), lbool.True))
			astTestMust(t, p.AssignmentEvent(terms[1], true))
			// A bound OR reads its signed SAT literals. An asserted OR
			// without a variable reads the NOT operand's context assignment.
			wantNegative := mode != "bound"
			if p.IsRelevant(negated) != wantNegative || p.IsRelevant(terms[1]) == wantNegative {
				t.Fatalf("signed witness for %s: %v", mode, p.RelevantExpressions())
			}
		})
	}
}

func TestSMTAssertedOrNestedNot(t *testing.T) {
	m := astTestManager(t)
	ctx, p, s := smtInternalizerFixture(t, m, nil)
	terms, literals := benchmarkSMTBooleansForTest(t, m, s, "nested-root", 2)
	inner, err := m.MkNot(terms[0])
	astTestMust(t, err)
	astTestMust(t, s.Internalize(inner, false))
	outer, err := m.MkNot(inner)
	astTestMust(t, err)
	root, err := m.MkOr(outer, terms[1])
	astTestMust(t, err)
	astTestMust(t, s.Assert(root, ast.ASTHandle{}, 0))
	astTestMust(t, ctx.SetAssignment(inner, lbool.False))
	astTestMust(t, s.SetAssignment(literals[1].Var(), lbool.True))
	astTestMust(t, p.AssignmentEvent(terms[1], true))
	if !p.IsRelevant(outer) || p.IsRelevant(terms[1]) {
		t.Fatalf("the immediate NOT operand determines the first witness: %v", p.RelevantExpressions())
	}
}

func TestSMTRelevancyWatchesNestedNotLiteral(t *testing.T) {
	for _, mode := range []string{"asserted", "bound"} {
		t.Run(mode, func(t *testing.T) {
			m := astTestManager(t)
			_, p, s := smtInternalizerFixture(t, m, nil)
			a := smtRelevancyBool(t, m, "watch-nested-not-a")
			b := smtRelevancyBool(t, m, "watch-nested-not-b")
			seed := smtRelevancyBool(t, m, "watch-nested-not-seed")
			marker := smtRelevancyBool(t, m, "watch-nested-not-marker")
			inner, err := m.MkNot(a)
			astTestMust(t, err)
			outer, err := m.MkNot(inner)
			astTestMust(t, err)
			parent, err := m.MkOr(outer, b)
			astTestMust(t, err)
			astTestMust(t, s.Internalize(inner, false))
			astTestMust(t, s.Internalize(seed, true))
			if mode == "asserted" {
				astTestMust(t, s.Assert(parent, ast.ASTHandle{}, 0))
			} else {
				astTestMust(t, s.Internalize(parent, true))
				astTestMust(t, s.markRelevant(parent))
			}
			astTestMust(t, p.AddWatchTarget(seed, true, marker))
			if p.IsRelevant(outer) || p.IsRelevant(marker) {
				t.Fatal("watch fired before an assignment")
			}
			outerLit, err := s.Literal(outer)
			astTestMust(t, err)
			seedLit, err := s.Literal(seed)
			astTestMust(t, err)
			astTestMust(t, s.AssignLiteral(outerLit, NewSMTBooleanAxiom()))
			astTestMust(t, s.AssignLiteral(seedLit, NewSMTBooleanAxiom()))
			ok, err := s.PropagateCore()
			astTestMust(t, err)
			if !ok {
				t.Fatal("unexpected Boolean conflict")
			}
			// The signed literal for outer is on inner's variable. Boolean
			// propagation assigns a later, after the independent seed.
			marks := p.RelevantExpressions()
			outerIndex, markerIndex := slices.Index(marks, outer), slices.Index(marks, marker)
			if outerIndex < 0 || markerIndex < 0 || outerIndex >= markerIndex {
				t.Fatalf("the outer NOT watch must fire before the seed watch, got indices %d and %d", outerIndex, markerIndex)
			}
			if !p.IsRelevant(a) || p.IsRelevant(b) {
				t.Fatal("relevance did not follow the true disjunct")
			}
		})
	}
}

func TestSMTAssertedOrAfterParentVariableReuse(t *testing.T) {
	m := astTestManager(t)
	ctx, p, s := smtInternalizerFixture(t, m, nil)
	terms, literals := benchmarkSMTBooleansForTest(t, m, s, "reuse-root", 3)
	root, err := m.MkOrArray(terms)
	astTestMust(t, err)
	astTestMust(t, s.Assert(root, ast.ASTHandle{}, 0))
	astTestMust(t, s.PushScope())
	astTestMust(t, s.Internalize(root, true))
	old, ok := s.BoolVar(root)
	if !ok {
		t.Fatal("the scoped root needs a Boolean variable")
	}
	astTestMust(t, s.PopScopes(1))
	fresh := smtRelevancyBool(t, m, "reused-parent-variable")
	astTestMust(t, s.Internalize(fresh, true))
	if v, ok := s.BoolVar(fresh); !ok || v != old {
		t.Fatalf("variable reuse control: got %d, want %d", v, old)
	}
	astTestMust(t, s.SetAssignment(old, lbool.False))
	astTestMust(t, s.SetAssignment(literals[2].Var(), lbool.True))
	astTestMust(t, p.AssignmentEvent(terms[2], true))
	if got := p.RelevantExpressions(); !slices.Equal(got, []ast.ASTHandle{root, terms[2]}) {
		t.Fatalf("false on the replacement variable affected the old disjunction: %v", got)
	}
	astTestMust(t, ctx.SetRelevant(terms[2], false))
	astTestMust(t, s.SetAssignment(literals[0].Var(), lbool.True))
	astTestMust(t, p.AssignmentEvent(terms[0], true))
	if !p.IsRelevant(terms[0]) || p.IsRelevant(terms[2]) {
		t.Fatal("a cleared witness must be reselected in argument order")
	}
}

func TestSMTAssertedOrWatchRetainsPremarkedParent(t *testing.T) {
	for _, mode := range []string{"compact", "reserved-literal"} {
		t.Run(mode, func(t *testing.T) {
			m := astTestManager(t)
			ctx, p, s := smtInternalizerFixture(t, m, nil)
			terms, literals := benchmarkSMTBooleansForTest(t, m, s, "premarked-or", 2)
			args := []ast.ASTHandle{terms[0], terms[1]}
			if mode == "reserved-literal" {
				falsehood, err := m.MkFalse()
				astTestMust(t, err)
				args = append(args, falsehood)
			}
			root, err := m.MkOrArray(args)
			astTestMust(t, err)
			astTestMust(t, m.IncRef(root))
			astTestMust(t, s.PushScope())
			// The checker mark has no owning propagation entry. Assert must
			// leave a live parent for both deferred watches.
			astTestMust(t, ctx.SetRelevant(root, true))
			astTestMust(t, s.Assert(root, ast.ASTHandle{}, 0))
			astTestMust(t, m.DecRef(root))
			if !m.IsLiveHandle(root) {
				t.Error("the deferred watch lost its parent when the caller released it")
			}
			args[0], args[1] = args[1], args[0]
			replacement, err := m.MkOrArray(args)
			astTestMust(t, err)
			astTestPin(t, m, replacement)
			astTestMust(t, ctx.SetRelevant(replacement, true))
			astTestMust(t, s.AssignLiteral(literals[1], NewSMTBooleanAxiom()))
			ok, err := s.PropagateCore()
			astTestMust(t, err)
			if !ok || !p.IsRelevant(terms[1]) || p.IsRelevant(terms[0]) {
				t.Fatalf("expected only the true disjunct to become relevant: %v", p.RelevantExpressions())
			}
			astTestMust(t, s.PopScopes(1))
			if m.IsLiveHandle(root) || p.IsRelevant(terms[1]) || !p.IsRelevant(replacement) {
				t.Fatal("pop did not retire the assertion while preserving the independent mark")
			}
		})
	}
}

func TestSMTAssertedOrPopRetriesParentAndSourceRelease(t *testing.T) {
	var p *SMTRelevancyPropagator
	var refused error
	m, _, _, trigger := smtQuantifierRefusalFixture(t, func() { refused = p.PopScopes(1) })
	_, p, s := smtInternalizerFixture(t, m, nil)
	terms, literals := benchmarkSMTBooleansForTest(t, m, s, "refused-or", 2)
	root, err := m.MkOrArray(terms)
	astTestMust(t, err)
	astTestMust(t, p.PushScope())
	// One child is already false, leaving one deferred watch that owns
	// both the parent and the source. The busy manager refuses the source
	// decrement after that watch has been unlinked.
	astTestMust(t, s.SetAssignment(literals[0].Var(), lbool.False))
	astTestMust(t, p.addAssertedOrWatches(root, terms, literals))
	before, err := m.GetRefCount(terms[1])
	astTestMust(t, err)
	trigger()
	if refused != ast.ErrASTPending || p.positiveWatch.head(terms[1]) != 0 || !m.IsLiveHandle(root) {
		t.Fatalf("refused pop must unlink the watch and keep its owners: %v", refused)
	}
	astTestMust(t, p.PopScopes(1))
	after, err := m.GetRefCount(terms[1])
	astTestMust(t, err)
	// The source loses the watch's reference and the parent's child edge.
	if m.IsLiveHandle(root) || after+2 != before || p.positiveWatch.head(terms[1]) != 0 {
		t.Fatalf("retry did not retire both owners: source references %d → %d", before, after)
	}
}
