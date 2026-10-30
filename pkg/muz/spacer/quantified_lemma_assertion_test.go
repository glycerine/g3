package spacer

// Source: src/muz/spacer/spacer_context.h lemma::is_ground and
// spacer_context.cpp pred_transformer::add_lemma_core/add_lemma_from_child.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerQuantifiedLemmaAssertionPolicies(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	must(err)
	defer func() { must(owner.Close()) }()
	var pins []ast.ASTRef
	defer func() { must(releaseNormalizeRefs(m, pins)) }()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		ref, err := m.NewRef(h)
		must(err)
		pins = append(pins, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		symbol, err := m.Symbols.Intern(name)
		must(err)
		return own(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
	}
	childDecl := decl("quantified-child", []ast.ASTHandle{boolean})
	parentDecl := decl("quantified-parent", nil)
	child, err := NewRulePredicateTransformer(owner, childDecl, PropSolverConfig{})
	must(err)
	defer func() { must(child.Close()) }()
	parent, err := NewRulePredicateTransformer(owner, parentDecl, PropSolverConfig{})
	must(err)
	defer func() { must(parent.Close()) }()
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	must(rules.RegisterPredicate(childDecl))
	must(rules.RegisterPredicate(parentDecl))
	truth := own(m.MkTrue())
	head := own(m.MkApp(parentDecl, nil))
	tail := own(m.MkApp(childDecl, []ast.ASTHandle{truth}))
	rule, err := rules.NewRule(head, []ast.ASTHandle{tail}, nil, "quantified-parent-rule")
	must(err)
	defer func() { must(rule.Close()) }()
	must(parent.InitializeRules(map[ast.ASTHandle]*PredicateTransformer{parentDecl: parent, childDecl: child}, []*base.Rule{rule}))
	property := decl("quantified-property", []ast.ASTHandle{boolean})
	variable := own(m.MkVar(0, boolean))
	body := own(m.MkApp(property, []ast.ASTHandle{variable}))
	quantifier := own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body}))
	lemma, err := NewLemma(owner, quantifier, 1)
	must(err)
	defer func() { must(lemma.Close()) }()
	must(lemma.AddBindings([]ast.ASTHandle{truth}))
	for _, tc := range []struct {
		name                 string
		instantiate, qlemmas bool
		assertions           uint32
	}{
		{"instances-only", true, false, 1},
		{"quantifier-only", false, true, 1},
		{"both", true, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must(parent.SetQuantifiedLemmaPolicy(tc.instantiate, tc.qlemmas))
			childBefore, err := child.solver.ownedBackends[0].NumAssertions()
			must(err)
			parentBefore, err := parent.solver.ownedBackends[0].NumAssertions()
			must(err)
			must(child.AddLemmaCore(lemma))
			childAfter, err := child.solver.ownedBackends[0].NumAssertions()
			must(err)
			parentAfter, err := parent.solver.ownedBackends[0].NumAssertions()
			must(err)
			if childAfter != childBefore {
				t.Error("a quantified lemma must not be asserted into its own predicate solver")
			}
			if parentAfter-parentBefore != tc.assertions {
				t.Errorf("parent received %d assertions, want %d", parentAfter-parentBefore, tc.assertions)
			}
		})
	}
	// An ordinary ground lemma is asserted both locally and through the user
	// even when transmission of quantified lemmas is disabled.
	must(parent.SetQuantifiedLemmaPolicy(false, false))
	ground, err := NewLemma(owner, own(m.MkApp(property, []ast.ASTHandle{truth})), 1)
	must(err)
	defer func() { must(ground.Close()) }()
	if isGround, err := ground.IsGround(); err != nil || !isGround {
		t.Fatal("ordinary clause must be a ground lemma", isGround, err)
	}
	if isGround, err := lemma.IsGround(); err != nil || isGround {
		t.Fatal("top-level quantified clause must be a non-ground lemma", isGround, err)
	}
	childBefore, err := child.solver.ownedBackends[0].NumAssertions()
	must(err)
	parentBefore, err := parent.solver.ownedBackends[0].NumAssertions()
	must(err)
	must(child.AddLemmaCore(ground))
	childAfter, err := child.solver.ownedBackends[0].NumAssertions()
	must(err)
	parentAfter, err := parent.solver.ownedBackends[0].NumAssertions()
	must(err)
	if childAfter-childBefore != 1 || parentAfter-parentBefore != 1 {
		t.Fatal("ground lemma assertion policy changed")
	}

}
