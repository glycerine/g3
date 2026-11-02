package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazySparseEqualProjectAndIdenticalStayDeferred(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	equal, err := engine.New(lazySignature(t, 2, 3), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{0, 0}, {1, 1}, {1, 2}} {
		if _, err = equal.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err = equal.FilterEqual(1, 0); err != nil {
		t.Fatal(err)
	}
	projected, err := equal.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	if equal.Forced() || projected.Forced() {
		t.Fatal("equal/project filter was eager")
	}
	facts, err := projected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{2}, Fact{1})
	if projected.root.evaluated == nil || projected.root.evaluated.layout.entryBytes == 0 {
		t.Fatal("equal/project did not produce packed sparse rows")
	}
	stats := engine.Statistics()
	if stats.SelectEqualProject != 1 || stats.EqualFilters != 1 || !equal.Forced() {
		t.Fatal("equal/project fusion", stats, equal.Forced())
	}

	identical, err := engine.New(lazySignature(t, 2, 2, 3), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{0, 0, 1}, {0, 1, 2}, {1, 1, 0}} {
		if _, err = identical.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err = identical.FilterIdentical([]int{0, 1}); err != nil || identical.Forced() {
		t.Fatal("identical filter was eager", err)
	}
	facts, err = identical.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0, 0, 1}, Fact{1, 1, 0})
	if identical.root.evaluated == nil || identical.root.evaluated.layout.entryBytes == 0 {
		t.Fatal("identical filter did not produce packed sparse rows")
	}
}

func TestLazySparseInterpretedProjectAndNegatedJoinFusion(t *testing.T) {
	f := newTableRelationFixture(t)
	theory := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazySparseEngine()
	defer engine.Close()
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	table, err := engine.New(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{0, 0}, {1, 1}, {1, 2}} {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := f.ast.MkEq(variable, f.truth)
	if err != nil {
		t.Fatal(err)
	}
	if err = table.FilterInterpreted(f.relations, sorts, theory, condition); err != nil {
		t.Fatal(err)
	}
	projected, err := table.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	if table.Forced() || projected.Forced() {
		t.Fatal("interpreted/project filter was eager")
	}
	facts, err := projected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{2}, Fact{1})
	stats := engine.Statistics()
	if stats.InterpretedProjects != 1 || stats.InterpretedFilters != 1 || !table.Forced() {
		t.Fatal("interpreted/project fusion", stats, table.Forced())
	}

	target, _ := engine.New(lazySignature(t, 2), 0)
	first, _ := engine.New(lazySignature(t, 2), 0)
	second, _ := engine.New(lazySignature(t, 2), 0)
	for _, fact := range []Fact{{0}, {1}} {
		_, _ = target.Add(fact)
	}
	_, _ = first.Add(Fact{1})
	_, _ = second.Add(Fact{1})
	negative, err := first.Join(second, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	if err = target.FilterByNegation(negative, []int{0}, []int{0}); err != nil {
		t.Fatal(err)
	}
	if target.Forced() || negative.Forced() {
		t.Fatal("negated join was eager")
	}
	facts, err = target.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0})
	stats = engine.Statistics()
	if stats.NegationFilters != 1 || negative.Forced() {
		t.Fatal("negated join fusion", stats, negative.Forced())
	}
}

func TestLazySparseNullaryFiltersAndInterpretedReferenceOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	theory := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazySparseEngine()
	seed, err := engine.New(lazySignature(t, 2), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = seed.Add(Fact{1})
	nullary, err := seed.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	if err = nullary.FilterIdentical(nil); err != nil || nullary.Forced() {
		t.Fatal("nullary identical was eager", err)
	}
	baseline := f.ast.Stats().LiveReferences
	if err = nullary.FilterInterpreted(f.relations, nil, theory, f.falsehood); err != nil {
		t.Fatal(err)
	}
	if got := f.ast.Stats().LiveReferences; got != baseline+1 {
		t.Fatal("deferred condition reference", got, baseline)
	}
	facts, err := nullary.Facts()
	if err != nil || len(facts) != 0 {
		t.Fatal("ground-false nullary filter", facts, err)
	}
	negativeSeed, _ := engine.New(lazySignature(t, 2), 0)
	_, _ = negativeSeed.Add(Fact{0})
	negative, _ := negativeSeed.Project([]int{0})
	truthSeed, _ := engine.New(lazySignature(t, 2), 0)
	_, _ = truthSeed.Add(Fact{1})
	truth, _ := truthSeed.Project([]int{0})
	if err = truth.FilterByNegation(negative, nil, nil); err != nil || truth.Forced() {
		t.Fatal("nullary negation was eager", err)
	}
	facts, err = truth.Facts()
	if err != nil || len(facts) != 0 {
		t.Fatal("nullary true minus true", facts, err)
	}
	if err = engine.Close(); err != nil {
		t.Fatal(err)
	}
	if got := f.ast.Stats().LiveReferences; got != baseline {
		t.Fatal("condition reference survived engine close", got, baseline)
	}
}
