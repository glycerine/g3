package smtcore

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The trans walk must find a target, refuse an absent one, and survive a cycle.
func TestTransReachesWalksWithoutAllocating(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "trans", 40)
	s := f.s
	for i := 1; i < len(f.base)-1; i++ {
		astTestMust(t, s.addEquality(s.context.enodes.get(f.base[i-1]), s.context.enodes.get(f.base[i]), NewSMTEqualityAxiom()))
	}
	first := s.context.enodes.get(f.base[0])
	loner := s.context.enodes.get(f.base[len(f.base)-1])
	if !first.TransReaches(first.root) {
		t.Fatal("no path to root")
	}
	if first.TransReaches(loner) {
		t.Fatal("reached a node in another class")
	}
	if allocs := testing.AllocsPerRun(50, func() { _ = first.TransReaches(first.root) }); allocs != 0 {
		t.Fatalf("%v allocations per walk", allocs)
	}
	// Corrupt the proof tree into cycles of several lengths.
	nodes := []*SMTEnode{}
	for _, h := range f.base[:6] {
		nodes = append(nodes, s.context.enodes.get(h))
	}
	saved := make([]SMTTransJustification, len(nodes))
	for i, node := range nodes {
		saved[i] = node.trans
	}
	for length := 1; length <= len(nodes); length++ {
		for i := 0; i < length; i++ {
			nodes[i].trans.Target = nodes[(i+1)%length]
		}
		if nodes[0].TransReaches(loner) {
			t.Fatal("cycle reached an unrelated node", length)
		}
		for i := 0; i < length; i++ {
			if !nodes[0].TransReaches(nodes[i]) {
				t.Fatal("cycle member not reached", length, i)
			}
		}
	}
	for i, node := range nodes {
		node.trans = saved[i]
	}
}

// The common ancestor must match a set-based search on random trees, and leave
// no marks behind for the next search.
func TestCommonTransAncestorMatchesSetSearch(t *testing.T) {
	for seed := int64(0); seed < 25; seed++ {
		r := rand.New(rand.NewSource(seed))
		f := newSMTEqualityUndoFixture(t, fmt.Sprintf("ancestor-%d", seed), 14)
		s := f.s
		for i := 0; i < 11; i++ {
			astTestMust(t, s.addEquality(s.context.enodes.get(f.base[r.Intn(14)]), s.context.enodes.get(f.base[r.Intn(14)]), NewSMTEqualityAxiom()))
		}
		for i := range f.base {
			for j := range f.base {
				a, b := s.context.enodes.get(f.base[i]), s.context.enodes.get(f.base[j])
				if a.root != b.root {
					if _, err := smtCommonTransAncestor(a, b); err == nil {
						t.Fatal("different classes reported an ancestor")
					}
					continue
				}
				got, err := smtCommonTransAncestor(a, b)
				astTestMust(t, err)
				seen := map[*SMTEnode]bool{}
				for current := a; current != nil; current = current.trans.Target {
					seen[current] = true
				}
				var want *SMTEnode
				for current := b; current != nil; current = current.trans.Target {
					if seen[current] {
						want = current
						break
					}
				}
				if got != want {
					t.Fatalf("seed %d: ancestor %p, want %p", seed, got, want)
				}
				for _, node := range s.enodeStack {
					if node.transMark {
						t.Fatal("search left a mark")
					}
				}
			}
		}
	}
}

// Expanding repeatedly must return correct results from reused buffers, and the
// exported form must hand back storage the next expansion cannot overwrite.
func TestExpanderReusesBuffersSafely(t *testing.T) {
	m, s, nodes, literals := smtDiseqFixture(t, "expand-reuse", 4, 3)
	a, b, c, d := nodes[0], nodes[1], nodes[2], nodes[3]
	astTestMust(t, s.addEquality(a, b, NewSMTEqualityEquation(literals[0])))
	astTestMust(t, s.addEquality(c, d, NewSMTEqualityEquation(literals[1])))
	_ = m
	expander := NewSMTAntecedentExpander()
	first, err := NewSMTEqualityPropagationJustification(a, b)
	astTestMust(t, err)
	second, err := NewSMTEqualityPropagationJustification(c, d)
	astTestMust(t, err)
	kept, err := expander.ExpandJustification(first)
	astTestMust(t, err)
	if len(kept) != 1 || kept[0] != literals[0] {
		t.Fatal("first expansion", kept)
	}
	other, err := expander.ExpandJustification(second)
	astTestMust(t, err)
	if len(other) != 1 || other[0] != literals[1] || kept[0] != literals[0] {
		t.Fatal("expansion results shared storage", kept, other)
	}
	// Marks and the processed set must reset, or repeats would come back empty.
	for i := 0; i < 3; i++ {
		again, err := expander.expandReusing(first)
		astTestMust(t, err)
		if len(again) != 1 || again[0] != literals[0] {
			t.Fatal("repeat expansion", i, again)
		}
		if first.IsMarked() {
			t.Fatal("justification left marked")
		}
	}
	if allocs := testing.AllocsPerRun(50, func() {
		_, _ = expander.expandReusing(first)
		expander.ResetUses()
	}); allocs != 0 {
		t.Fatalf("%v allocations per reused expansion", allocs)
	}
}

// A resolver reads a Boolean enode's literal live, in its current polarity.
func TestResolverEnodeLiteralFollowsAssignment(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	atom := smtRelevancyBool(t, m, "live-literal")
	resolver, err := NewSMTConflictResolver(s)
	astTestMust(t, err)
	defer func() { astTestMust(t, resolver.Close()) }()
	astTestMust(t, s.Internalize(atom, false))
	node := s.context.enodes.get(atom)
	variable, found := s.BoolVar(atom)
	if node == nil || !found {
		t.Fatal("fixture did not make a Boolean enode")
	}
	literal, err := resolver.expander.LiteralForENode(node)
	astTestMust(t, err)
	if literal != NewSMTLiteral(variable, false) {
		t.Fatal("unassigned literal", literal)
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(NewSMTLiteral(variable, true), NewSMTBooleanAxiom()))
	literal, err = resolver.expander.LiteralForENode(node)
	astTestMust(t, err)
	if literal != NewSMTLiteral(variable, true) {
		t.Fatal("false assignment must negate the literal", literal)
	}
	astTestMust(t, s.PopScopes(1))
	if _, err = resolver.expander.LiteralForENode(nil); err == nil {
		t.Fatal("nil enode must be refused")
	}
}

// minimize_lemma filters antecedents with level_approx_set, a 32-bit mask
// indexed by level & 31. Level 2 is outside the lemma's exact level set
// {1, 33, 34} but aliases level 34, so y is followed and x is removed as
// implied by the marked a.
func TestMinimizeLemmaApproxLevelSet(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	_, literals := benchmarkSMTBooleansForTest(t, m, s, "minimize", 4)
	a, y, x, d := literals[0], literals[1], literals[2], literals[3]
	resolver, err := NewSMTConflictResolver(s)
	astTestMust(t, err)
	defer func() { astTestMust(t, resolver.Close()) }()
	for level := 1; level <= 34; level++ {
		astTestMust(t, s.PushScope())
		switch level {
		case 1:
			astTestMust(t, s.AssignLiteral(a, NewSMTBooleanAxiom()))
		case 2:
			astTestMust(t, s.AssignLiteral(y, NewSMTBooleanBinaryClause(a)))
		case 33:
			astTestMust(t, s.AssignLiteral(x, NewSMTBooleanBinaryClause(y)))
		case 34:
			astTestMust(t, s.AssignLiteral(d, NewSMTBooleanAxiom()))
		}
	}
	levels := resolver.levels()
	marks := resolver.resetMarks()
	for _, l := range []SMTLiteral{a, x} {
		marks[l.Var()] = true
	}
	lemma := []SMTLiteral{d.Negated(), x.Negated(), a.Negated()}
	kept, err := resolver.minimizeLemma(lemma, marks, levels, 0)
	astTestMust(t, err)
	if len(kept) != 2 || kept[0] != d.Negated() || kept[1] != a.Negated() {
		t.Fatal("x must be removed through the aliased level", kept)
	}
	for _, l := range literals {
		if marks[l.Var()] && l != a {
			t.Fatal("minimization left a temporary mark", l)
		}
	}
	astTestMust(t, s.PopScopes(34))
}

func benchmarkSMTBooleansForTest(t *testing.T, m *ast.ASTManager, s *SMTInternalizerState, prefix string, count int) ([]ast.ASTHandle, []SMTLiteral) {
	t.Helper()
	expressions := make([]ast.ASTHandle, count)
	literals := make([]SMTLiteral, count)
	for i := range expressions {
		expressions[i] = smtRelevancyBool(t, m, fmt.Sprintf("%s-%d", prefix, i))
		astTestMust(t, s.Internalize(expressions[i], true))
		variable, found := s.BoolVar(expressions[i])
		if !found {
			t.Fatal("internalized Boolean has no variable")
		}
		literals[i] = NewSMTLiteral(variable, false)
	}
	return expressions, literals
}

// One conflict must not allocate per Boolean variable. The returned lemma is
// the only slice that has to outlive resolution.
func TestConflictResolutionAllocationsDoNotScaleWithVariables(t *testing.T) {
	measure := func(extra int) float64 {
		m := astTestManager(t)
		_, _, s := smtInternalizerFixture(t, m, nil)
		_, literals := benchmarkSMTBooleansForTest(t, m, s, "conflict-scale", 3+extra)
		astTestMust(t, s.PushScope())
		astTestMust(t, s.AssignLiteral(literals[0], NewSMTBooleanAxiom()))
		astTestMust(t, s.PushScope())
		astTestMust(t, s.AssignLiteral(literals[1], NewSMTBooleanAxiom()))
		reason, err := NewSMTClause(m, []SMTLiteral{literals[2], literals[0].Negated()}, SMTClauseLearned, nil, nil, false, nil)
		astTestMust(t, err)
		astTestMust(t, s.AssignLiteral(literals[2], NewSMTBooleanClause(reason)))
		conflict, err := NewSMTClause(m, []SMTLiteral{literals[2].Negated(), literals[1].Negated()}, SMTClauseAux, nil, nil, false, nil)
		astTestMust(t, err)
		resolver, err := NewSMTConflictResolver(s)
		astTestMust(t, err)
		t.Cleanup(func() {
			astTestMust(t, resolver.Close())
			astTestMust(t, conflict.Close())
			astTestMust(t, reason.Close())
		})
		return testing.AllocsPerRun(50, func() {
			resolution, err := resolver.Resolve(NewSMTBooleanClause(conflict), SMTNullLiteral, 0)
			astTestMust(t, err)
			if !resolution.Resolved {
				t.Fatal("not resolved")
			}
		})
	}
	few, many := measure(0), measure(2000)
	if many > few {
		t.Fatalf("allocations grew from %v to %v with variable count", few, many)
	}
}
