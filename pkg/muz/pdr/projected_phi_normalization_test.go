package pdr

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRProjectedPhiNormalizesWithNoVariablesOrLite(t *testing.T) {
	m, c, _ := classifierContextFixture(t)
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	symbol, err := m.Symbols.Intern("projected-phi-p")
	closureMust(t, err)
	decl, err := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	p, err := m.MkApp(decl, nil)
	closureMust(t, err)
	root, err := m.NewRef(p)
	closureMust(t, err)
	notP, err := m.MkNot(p)
	closureMust(t, err)
	double, err := m.MkNot(notP)
	closureMust(t, err)
	doubleRoot, err := m.NewRef(double)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&doubleRoot)); closureMust(t, m.DelRef(&root)) }()
	for _, skipLite := range []bool{false, true} {
		prior := c.lite
		if skipLite {
			c.lite = nil
		}
		vars := []ast.ASTRef{}
		result, x := c.normalizeProjectedPhi([]ast.ASTHandle{double, double}, &vars, nil)
		c.lite = prior
		closureMust(t, x)
		got, x := m.RefGet(result)
		closureMust(t, x)
		if got != p {
			t.Fatalf("skipLite=%v: normalized %v, want %v", skipLite, got, p)
		}
		if len(vars) != 0 {
			t.Fatal("empty variable vector changed")
		}
		closureMust(t, m.DelRef(&result))
	}
	if c.projectionTheory == nil || !c.projectionTheoryReady {
		t.Fatal("theory rewrite was not reached")
	}
}

func TestPDRProjectedPhiWithoutGlobalServicesStillNormalizes(t *testing.T) {
	m, configured, _ := classifierContextFixture(t)
	config := configured.owner.contexts.config
	c, err := NewContext(m, nil, 1, ReachableNoCache, smtcore.SMTSolverConfig{Params: config.Params})
	closureMust(t, err)
	defer func() { closureMust(t, c.Close()) }()
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	symbol, err := m.Symbols.Intern("projected-phi-no-globals")
	closureMust(t, err)
	decl, err := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	p, err := m.MkApp(decl, nil)
	closureMust(t, err)
	root, err := m.NewRef(p)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&root)) }()
	vars := []ast.ASTRef{}
	result, err := c.normalizeProjectedPhi([]ast.ASTHandle{p, p}, &vars, nil)
	closureMust(t, err)
	got, err := m.RefGet(result)
	closureMust(t, err)
	if got != p {
		t.Fatal("duplicate conjunction was not normalized without globals", got)
	}
	closureMust(t, m.DelRef(&result))
	if c.projectionTheory != nil {
		t.Fatal("nil global services created a theory rewriter")
	}
}

func TestPDRProjectedPhiRewriterConstructorFailureRetries(t *testing.T) {
	m, c, _ := classifierContextFixture(t)
	truth, err := m.MkTrue()
	closureMust(t, err)
	// Reconstruct the eagerly owned rewriter to exercise partial construction.
	closureMust(t, c.projectionTheory.Close())
	c.projectionTheory = nil
	c.projectionTheoryReady = false
	// Prevent QE from consuming the injected allocation before the rewriter.
	lite := c.lite
	c.lite = nil
	defer func() { c.lite = lite }()
	vars := []ast.ASTRef{}
	foundPartial := false
	for failurePoint := int64(0); failurePoint < 200; failurePoint++ {
		m.Budget().FailAfter(failurePoint)
		result, failure := c.normalizeProjectedPhi([]ast.ASTHandle{truth}, &vars, nil)
		m.Budget().FailAfter(-1)
		if failure != nil && c.projectionTheory != nil && !c.projectionTheoryReady {
			if result != (ast.ASTRef{}) {
				t.Fatal("failure published phi")
			}
			foundPartial = true
			break
		}
		if result != (ast.ASTRef{}) {
			closureMust(t, m.DelRef(&result))
		}
		if c.projectionTheory != nil {
			closureMust(t, c.projectionTheory.Close())
			c.projectionTheory = nil
			c.projectionTheoryReady = false
		}
	}
	if !foundPartial {
		t.Fatal("no partial theory-constructor failure was reached")
	}
	result, err := c.normalizeProjectedPhi([]ast.ASTHandle{truth}, &vars, nil)
	closureMust(t, err)
	got, err := m.RefGet(result)
	closureMust(t, err)
	if !m.IsTrue(got) || !c.projectionTheoryReady {
		t.Fatal("retry did not complete theory construction", got)
	}
	closureMust(t, m.DelRef(&result))
}

func TestPDRProjectionWithZeroEliminatedVariablesCreatesChildren(t *testing.T) {
	for _, withoutLite := range []bool{false, true} {
		t.Run(map[bool]string{false: "with-lite", true: "without-lite"}[withoutLite], func(t *testing.T) {
			m, c, rules := classifierContextFixture(t)
			boolean, err := m.MkBoolSort()
			closureMust(t, err)
			declare := func(name string) ast.ASTHandle {
				sym, x := m.Symbols.Intern(name)
				closureMust(t, x)
				decl, x := m.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
				closureMust(t, x)
				closureMust(t, rules.RegisterPredicate(decl))
				return decl
			}
			pred, goal := declare("projected-zero-pred"), declare("projected-zero-goal")
			predApp, err := m.MkApp(pred, nil)
			closureMust(t, err)
			goalApp, err := m.MkApp(goal, nil)
			closureMust(t, err)
			set, err := base.NewRuleSet(rules)
			closureMust(t, err)
			defer func() { closureMust(t, set.Close()) }()
			fact, err := rules.NewRule(predApp, nil, nil, "projected-zero-fact")
			closureMust(t, err)
			closureMust(t, set.Add(fact))
			closureMust(t, fact.Close())
			step, err := rules.NewRule(goalApp, []ast.ASTHandle{predApp}, nil, "projected-zero-step")
			closureMust(t, err)
			closureMust(t, set.Add(step))
			closureMust(t, step.Close())
			closureMust(t, c.Initialize(set))
			lite := c.lite
			if withoutLite {
				c.lite = nil
			}
			status, solveErr := c.Solve(goal)
			c.lite = lite
			closureMust(t, solveErr)
			if status != lbool.True {
				t.Fatal("zero-variable derivation failed", status)
			}
			root, found, err := c.search.Root()
			closureMust(t, err)
			if !found || len(root.children) != 1 {
				t.Fatal("projection was not reached")
			}
			if c.projectionTheory == nil || !c.projectionTheoryReady {
				t.Fatal("context did not retain its source theory rewriter")
			}
		})
	}
}

func TestPDRProjectedPhiReducesDisequalitiesBeforeTheory(t *testing.T) {
	m, c, _ := classifierContextFixture(t)
	var refs []ast.ASTRef
	defer func() { closureMust(t, closeASTRefs(m, refs)) }()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		closureMust(t, err)
		ref, x := m.NewRef(h)
		closureMust(t, x)
		refs = append(refs, ref)
		return h
	}
	seven := own(m.ArithmeticRealNumeralText("7"))
	sort, err := m.GetSort(seven)
	closureMust(t, err)
	x := own(m.MkFreshConst("projected-phi-x", sort))
	boolean := own(m.MkBoolSort())
	q := own(m.MkFreshConst("projected-phi-q", boolean))
	notQ := own(m.MkNot(q))
	doubleQ := own(m.MkNot(notQ))
	forms := []ast.ASTHandle{doubleQ}
	for _, n := range []string{"0", "1", "2"} {
		value := own(m.ArithmeticRealNumeralText(n))
		eq := own(m.MkEq(x, value))
		forms = append(forms, own(m.MkNot(eq)))
	}
	md, err := model.NewModel(m)
	closureMust(t, err)
	defer func() { closureMust(t, md.Close()) }()
	xDecl, err := m.GetDecl(x)
	closureMust(t, err)
	qDecl, err := m.GetDecl(q)
	closureMust(t, err)
	truth := own(m.MkTrue())
	closureMust(t, md.Core().RegisterConstant(xDecl, seven))
	closureMust(t, md.Core().RegisterConstant(qDecl, truth))
	vars := []ast.ASTRef{}
	result, err := c.normalizeProjectedPhi(forms, &vars, md)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&result)) }()
	got, err := m.RefGet(result)
	closureMust(t, err)
	terms, err := m.FlattenAnd([]ast.ASTHandle{got})
	closureMust(t, err)
	expectedEq := own(m.MkEq(x, seven))
	if len(terms) != 2 || !((terms[0] == q && terms[1] == expectedEq) || (terms[1] == q && terms[0] == expectedEq)) {
		t.Fatal("expected rewritten guard and model equality after reduction", terms)
	}
}
