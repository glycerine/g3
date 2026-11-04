package duality

import (
	"errors"
	"testing"
	"time"
)

func TestCoveringDescendantClosureAndCandidateFiltering(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("frontier-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	leafSource, err := source.CreateNode(term)
	must(err)
	rootSource, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	earlier, err := graph.CloneNode(leafSource)
	must(err)
	_, err = graph.CreateLowerBoundEdge(earlier)
	must(err)
	leaf, err := graph.CloneNode(leafSource)
	must(err)
	_, err = graph.CreateLowerBoundEdge(leaf)
	must(err)
	root, err := graph.CloneNode(rootSource)
	must(err)
	_, err = graph.CreateEdge(root, root.Annotation, []*Node{leaf})
	must(err)
	covering, err := NewCovering(graph, aux)
	must(err)
	profile := NewProfiler(nil)
	var ticks int
	profile.now = func() time.Time {
		ticks++
		return time.Unix(int64(ticks), 0)
	}
	ctx.activeProfiler = profile
	defer func() { ctx.activeProfiler = nil }()
	must(profile.TimerStart("outer"))
	closed, err := covering.CloseDescendants(root)
	must(err)
	if !closed || covering.coveredBy[leaf] != leaf || covering.coveredBy[root] != nil {
		t.Fatal("descendant-first closure")
	}
	contains, err := covering.Contains(root)
	must(err)
	if contains {
		t.Fatal("root with covered descendant remains in frontier")
	}
	candidate, err := covering.Candidate(root)
	must(err)
	if candidate {
		t.Fatal("covered root is candidate")
	}
	candidate, err = covering.Candidate(earlier)
	must(err)
	if !candidate {
		t.Fatal("uncovered node rejected")
	}
	must(covering.SetDominated(earlier))
	candidate, err = covering.Candidate(earlier)
	must(err)
	if candidate {
		t.Fatal("dominated node accepted")
	}
	contains, err = covering.Contains(earlier)
	must(err)
	if !contains {
		t.Fatal("domination changed covering membership")
	}
	must(covering.Update(earlier))
	contains, err = covering.Contains(root)
	must(err)
	if !contains {
		t.Fatal("invalidation did not reopen ancestor frontier")
	}
	contains, err = covering.Contains(nil)
	if !contains || !errors.Is(err, ErrWrapperOwner) {
		t.Fatalf("invalid membership: got %v, %v", contains, err)
	}
	candidate, err = covering.Candidate(nil)
	if candidate || !errors.Is(err, ErrWrapperOwner) {
		t.Fatalf("invalid candidate: got %v, %v", candidate, err)
	}
	outer := profile.top.children["outer"]
	if profile.current != outer {
		t.Fatal("covering timer did not restore outer scope")
	}
	for _, name := range []string{"Contains", "Candidate"} {
		child := outer.children[name]
		if child == nil || child.elapsed != 4*time.Second {
			t.Fatalf("%s timer: got %v, want four one-second samples", name, child)
		}
	}
	must(profile.TimerStop("outer"))
	profile.now = nil
	if _, err = covering.Contains(earlier); err == nil {
		t.Fatal("Contains ignored profiler start failure")
	}
	if _, err = covering.Candidate(earlier); err == nil {
		t.Fatal("Candidate ignored profiler start failure")
	}
	if profile.current != profile.top {
		t.Fatal("failed timer start changed current scope")
	}
}
func TestAnnotationUpdateInvalidatesBoundSourceCovering(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("bound-cover-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(term)
	must(err)
	original, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer original.Close()
	earlier, err := original.CloneNode(relation)
	must(err)
	_, err = original.CreateLowerBoundEdge(earlier)
	must(err)
	later, err := original.CloneNode(relation)
	must(err)
	_, err = original.CreateLowerBoundEdge(later)
	must(err)
	covering, err := NewCovering(original, aux)
	must(err)
	added, err := covering.CoverByNode(later, earlier)
	must(err)
	if !added {
		t.Fatal("initial cover")
	}
	working, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer working.Close()
	base, err := NewDerivationTree(original, working)
	must(err)
	search, err := NewBacktrackingDerivation(base)
	must(err)
	must(search.BindSourceCovering(covering))
	learned, err := base.CreateApproximatedInstance(earlier)
	must(err)
	must(learned.Annotation.SetEmpty())
	changed, err := search.RecordAnnotationUpdate(learned, aux)
	must(err)
	if !changed || covering.coveredBy[later] != nil || !covering.someUpdates {
		t.Fatal("learned fact did not invalidate source cover")
	}
	changed, err = search.RecordAnnotationUpdate(learned, aux)
	must(err)
	if changed {
		t.Fatal("redundant learned fact changed source")
	}
}
