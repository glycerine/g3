package tab

// Source: src/muz/tab/tab_context.cpp selection::score_argument and
// prepare_weight_select. These oracles observe predicate choices rather than
// assigning internal score, frequency or cache state.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestTabSelectionConstructorCapIsPerArgument(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		tabDiagnosticsMust(t, err)
		ref, err := m.NewRef(h)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, m.DelRef(&ref)) })
		return h
	}
	name := func(text string) symbol.Symbol {
		s, err := m.Symbols.Intern(text)
		tabDiagnosticsMust(t, err)
		return s
	}
	_, err := m.RegisterDatatypePlugin()
	tabDiagnosticsMust(t, err)
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: name("selection-depth"), Constructors: []ast.DatatypeConstructorDefinition{
		{Name: name("selection-zero"), Recognizer: name("is-selection-zero")},
		{Name: name("selection-next"), Recognizer: name("is-selection-next"), Accessors: []ast.DatatypeAccessorDefinition{{Name: name("selection-tail"), Recursive: true}}},
	}}})
	tabDiagnosticsMust(t, err)
	sort := own(sorts[0], nil)
	constructors, err := m.DatatypeConstructors(sort)
	tabDiagnosticsMust(t, err)
	terms := make([]ast.ASTHandle, 41)
	terms[1] = own(m.MkApp(constructors[0], nil))
	for i := 2; i < len(terms); i++ {
		terms[i] = own(m.MkApp(constructors[1], []ast.ASTHandle{terms[i-1]}))
	}
	boolean := own(m.MkBoolSort())
	p := own(m.MkFuncDecl(name("selection-unary"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{}))
	q := own(m.MkFuncDecl(name("selection-binary"), []ast.ASTHandle{sort, sort}, boolean, ast.ASTFuncDeclInfo{}))
	apply := func(declaration ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		return own(m.MkApp(declaration, args))
	}
	truth, falsehood := own(m.MkTrue()), own(m.MkFalse())
	for _, tc := range []struct {
		name       string
		predicates []ast.ASTHandle
		want       uint32
	}{
		{"below-cap", []ast.ASTHandle{apply(p, terms[19]), apply(p, terms[20])}, 1},
		{"cap-tie-keeps-first", []ast.ASTHandle{apply(p, terms[20]), apply(p, terms[21])}, 0},
		{"each-argument-has-own-cap", []ast.ASTHandle{apply(p, terms[40]), apply(q, terms[20], terms[20])}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goal, err := NewClause(m, f.rw, falsehood, tc.predicates, truth)
			tabDiagnosticsMust(t, err)
			defer func() { tabDiagnosticsMust(t, goal.Close()) }()
			for _, strategy := range []string{"basic-weight", "weight"} {
				selection, err := newPredicateSelection(m, strategy)
				tabDiagnosticsMust(t, err)
				selected, err := selection.Select(goal)
				tabDiagnosticsMust(t, err)
				tabDiagnosticsMust(t, selection.Close())
				if selected != tc.want {
					t.Fatal("source constructor score chose the wrong predicate", strategy, selected, tc.want)
				}
			}
		})
	}
}

func TestTabSelectionNaturalDecayAndRolloverPreserveCachedChoices(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		tabDiagnosticsMust(t, err)
		ref, err := m.NewRef(h)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, m.DelRef(&ref)) })
		return h
	}
	boolean := own(m.MkBoolSort())
	atom := func(text string) ast.ASTHandle {
		name, err := m.Symbols.Intern(text)
		tabDiagnosticsMust(t, err)
		declaration := own(m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{}))
		return own(m.MkApp(declaration, nil))
	}
	old := atom("selection-old")
	fresh20, fresh41, fresh42 := atom("selection-fresh-20"), atom("selection-fresh-41"), atom("selection-fresh-42")
	beforeRollover, afterRollover := atom("selection-before-rollover"), atom("selection-after-rollover")
	truth, falsehood := own(m.MkTrue()), own(m.MkFalse())
	clause := func(predicates ...ast.ASTHandle) *Clause {
		goal, err := NewClause(m, f.rw, falsehood, predicates, truth)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, goal.Close()) })
		return goal
	}
	driver := clause(old)
	firstDecay := clause(fresh20, old)
	beforeSecondDecay := clause(fresh41)
	secondDecay := clause(fresh42, fresh41)
	beforeReset := clause(beforeRollover)
	resetChoice := clause(beforeRollover, afterRollover)
	preservedChoice := clause(afterRollover, old)
	baseline := m.Stats()
	selection, err := newPredicateSelection(m, "weight")
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, selection.Close()) }()
	rules := &RuleIndex{manager: m, byHead: make(map[ast.ASTHandle][]int)}
	tabDiagnosticsMust(t, selection.Init(rules))
	var calls uint32
	choose := func(goal *Clause, want uint32) {
		t.Helper()
		selected, err := selection.Select(goal)
		tabDiagnosticsMust(t, err)
		calls++
		if selected != want {
			t.Fatal("source aging chose the wrong predicate", calls, selected, want)
		}
	}
	advance := func(through uint32) {
		t.Helper()
		for calls < through {
			choose(driver, 0)
		}
	}
	// All atoms have equal structural score. At the source update boundaries
	// a newly seen atom loses to an earlier cached atom, even when listed first.
	advance(19)
	choose(firstDecay, 1)
	advance(40)
	choose(beforeSecondDecay, 0)
	choose(secondDecay, 1)
	// Rule initialization between queries resets structural averages while
	// the source aging schedule and predicate scores continue unchanged.
	tabDiagnosticsMust(t, selection.Init(rules))
	// The source integer schedule first reaches a frequency of 70528 at
	// selection 705470, then resets at selection 775998. Walk the live API to
	// that boundary; do not assign its internal counters or cached scores.
	advance(775996)
	choose(beforeReset, 0)
	choose(resetChoice, 1)
	// Rollover refreshes the multiplier, but must not erase old cached scores.
	choose(preservedChoice, 1)
	tabDiagnosticsMust(t, selection.Close())
	after := m.Stats()
	if after.LiveNodes != baseline.LiveNodes || after.LiveReferences != baseline.LiveReferences || after.LiveInfos != baseline.LiveInfos || after.LiveParameters != baseline.LiveParameters {
		t.Fatal("selection cache disposal changed AST ownership", baseline, after)
	}
}
