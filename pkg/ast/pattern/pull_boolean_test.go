package pattern

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPatternPullNoProofIdentityAndSourcePreconditionGuards(t *testing.T) {
	m := patternManager(t)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	b, err := m.MkBoolSort()
	patternMust(t, err)
	truth, err := m.MkTrue()
	patternMust(t, err)
	pin(truth)
	c, err := newPatternCandidates(m, 0, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })

	identity, err := c.pullNoProof(truth)
	patternMust(t, err)
	got, err := m.RefGet(identity)
	patternMust(t, err)
	if got != truth {
		t.Fatal("quantifier-free expression lost source identity", got, truth)
	}
	patternMust(t, m.DelRef(&identity))

	name := symbol.Numerical(970)
	forall, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{name}, Body: truth})
	patternMust(t, err)
	pin(forall)
	exists, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{name}, Body: truth})
	patternMust(t, err)
	pin(exists)
	mixed, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpAnd, nil, []ast.ASTHandle{forall, exists})
	patternMust(t, err)
	pin(mixed)
	if _, err = c.pullNoProof(mixed); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("mixed quantifier polarities must reject the violated SNF precondition", err)
	}

	outerExists, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{name}, Body: forall})
	patternMust(t, err)
	pin(outerExists)
	if _, err = c.pullNoProof(outerExists); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("nested non-universal quantifier must reject the source universal precondition", err)
	}
	if len(c.pendingPatternRefs) != 0 {
		t.Fatal("failed pulls retained temporary references", len(c.pendingPatternRefs))
	}
}

func TestPatternPullBooleanApplicationsShiftBindersAndFreshenNames(t *testing.T) {
	m := patternManager(t)
	ctx, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.BudgetField})
	patternMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.BudgetField})
	patternMust(t, err)
	global, err := uparams.NewGlobalParamsService(m.Symbols, ctx, uparams.GlobalParamsConfig{Budget: m.BudgetField, Catalog: catalog})
	patternMust(t, err)
	patternMust(t, global.InitializeSource())
	previous, err := uparams.BindProcessGlobalParamsService(global)
	patternMust(t, err)
	t.Cleanup(func() {
		_, e := uparams.BindProcessGlobalParamsService(previous)
		patternMust(t, e)
		patternMust(t, global.Close())
		patternMust(t, catalog.Close())
		patternMust(t, ctx.Close())
	})
	b, err := m.MkBoolSort()
	patternMust(t, err)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	v0, err := m.MkVar(0, b)
	patternMust(t, err)
	pin(v0)
	makeTerm := func(id uint32) ast.ASTHandle {
		t.Helper()
		decl, e := m.MkFuncDecl(symbol.Numerical(id), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		patternMust(t, e)
		pin(decl)
		term, e := m.MkApp(decl, []ast.ASTHandle{v0})
		patternMust(t, e)
		return pin(term)
	}
	f0, g0 := makeTerm(950), makeTerm(951)
	name, err := m.Symbols.Intern("repeated")
	patternMust(t, err)
	firstID, err := m.Symbols.Intern("first-pulled")
	patternMust(t, err)
	makeQ := func(body ast.ASTHandle, weight int32, qid symbol.Symbol) ast.ASTHandle {
		t.Helper()
		q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b},
			Names: []symbol.Symbol{name}, Body: body, Weight: weight, QID: qid})
		patternMust(t, e)
		return pin(q)
	}
	q1, q2 := makeQ(f0, 7, firstID), makeQ(g0, 3, symbol.Numerical(952))
	and, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpAnd, nil, []ast.ASTHandle{q1, q2, v0})
	patternMust(t, err)
	pin(and)
	c, err := newPatternCandidates(m, 0, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })
	var traceOutput bytes.Buffer
	trace := diagnostic.NewTraceTagsWithWriter(&traceOutput)
	trace.Enable("pull_quant")
	c.traceTags = trace
	ref, err := c.pullNoProof(and)
	patternMust(t, err)
	if trace.ShouldTrace("pull_quant") {
		want := "-------- [pull_quant] pull_quant1_core src/ast/normal_forms/pull_quant.cpp:102 ---------\nResult num decls:2\n------------------------------------------------\n" +
			"-------- [pull_quant] pull_quant1_core src/ast/normal_forms/pull_quant.cpp:131 ---------\n" +
			"shifted  bound: 1 shift1: 0 shift2: 1\n(k!950 (:var 0))\n---->\n(k!950 (:var 0))\n------------------------------------------------\n" +
			"-------- [pull_quant] pull_quant1_core src/ast/normal_forms/pull_quant.cpp:131 ---------\n" +
			"shifted  bound: 1 shift1: 1 shift2: 1\n(k!951 (:var 0))\n---->\n(k!951 (:var 1))\n------------------------------------------------\n" +
			"-------- [pull_quant] pull_quant1_core src/ast/normal_forms/pull_quant.cpp:109 ---------\n" +
			"shifted by: 2\n(:var 0)\n---->\n(:var 2)\n------------------------------------------------\n"
		if traceOutput.String() != want {
			t.Fatal("source declaration and shifted-child TRACE records", traceOutput.String())
		}
	} else if traceOutput.Len() != 0 {
		t.Fatal("erased TRACE record wrote output", traceOutput.String())
	}
	defer func() { patternMust(t, m.DelRef(&ref)) }()
	got, err := m.RefGet(ref)
	patternMust(t, err)
	forall, err := m.IsForall(got)
	patternMust(t, err)
	n, err := m.GetNumDeclarations(got)
	patternMust(t, err)
	weight, err := m.GetQuantifierWeight(got)
	patternMust(t, err)
	qid, _, err := m.GetQuantifierIDs(got)
	patternMust(t, err)
	newName, err := m.GetDeclarationName(got, 0)
	patternMust(t, err)
	oldName, err := m.GetDeclarationName(got, 1)
	patternMust(t, err)
	spelling, err := m.Symbols.String(newName)
	patternMust(t, err)
	if !forall || n != 2 || weight != 3 || qid != firstID || oldName != name ||
		newName == name || !strings.HasPrefix(spelling, "repeated!") {
		t.Fatal("source pulled declarations, QID, minimum weight and fresh duplicate name", forall, n, weight, qid, newName, oldName, spelling)
	}
	body, err := m.GetQuantifierBody(got)
	patternMust(t, err)
	for i, want := range []uint32{0, 1, 2} {
		child, e := m.GetArg(body, uint32(i))
		patternMust(t, e)
		if i < 2 {
			child, e = m.GetArg(child, 0)
			patternMust(t, e)
		}
		index, e := m.GetVarIndex(child)
		patternMust(t, e)
		if index != want {
			t.Fatal("source binder/free-variable shift", i, index, want)
		}
	}
	or, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpOr, nil, []ast.ASTHandle{q1, q2})
	patternMust(t, err)
	pin(or)
	orRef, err := c.pullNoProof(or)
	patternMust(t, err)
	defer func() { patternMust(t, m.DelRef(&orRef)) }()
	orQ, err := m.RefGet(orRef)
	patternMust(t, err)
	orBody, err := m.GetQuantifierBody(orQ)
	patternMust(t, err)
	orKind, err := m.GetDeclKind(orBody)
	patternMust(t, err)
	if orKind != ast.ASTOpOr {
		t.Fatal("source OR pull lost its Boolean declaration", orKind)
	}
	not, err := m.MkNot(q1)
	patternMust(t, err)
	pin(not)
	negRef, err := c.pullNoProof(not)
	patternMust(t, err)
	defer func() { patternMust(t, m.DelRef(&negRef)) }()
	neg, err := m.RefGet(negRef)
	patternMust(t, err)
	negForall, err := m.IsForall(neg)
	patternMust(t, err)
	negBody, err := m.GetQuantifierBody(neg)
	patternMust(t, err)
	isNot, innerBody, err := m.IsNot(negBody)
	patternMust(t, err)
	if negForall || !isNot || innerBody != f0 {
		t.Fatal("source NOT changes quantifier polarity around its body", negForall, isNot, innerBody, f0)
	}
}

func TestPatternInferencePullsBooleanApplicationThenFindsPattern(t *testing.T) {
	m := patternManager(t)
	b, err := m.MkBoolSort()
	patternMust(t, err)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	x, err := m.MkVar(1, b)
	patternMust(t, err)
	pin(x)
	y, err := m.MkVar(0, b)
	patternMust(t, err)
	pin(y)
	decl, err := m.MkFuncDecl(symbol.Numerical(960), []ast.ASTHandle{b, b}, b, ast.ASTFuncDeclInfo{})
	patternMust(t, err)
	pin(decl)
	term, err := m.MkApp(decl, []ast.ASTHandle{x, y})
	patternMust(t, err)
	pin(term)
	inner, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(961)}, Body: term, Weight: 4})
	patternMust(t, err)
	pin(inner)
	truth, err := m.MkTrue()
	patternMust(t, err)
	body, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpAnd, nil, []ast.ASTHandle{inner, truth})
	patternMust(t, err)
	pin(body)
	qid, err := m.Symbols.Intern("app-pull-qid")
	patternMust(t, err)
	outer, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(962)}, Body: body, Weight: 9, QID: qid})
	patternMust(t, err)
	pin(outer)
	c, err := newPatternCandidates(m, 0, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })
	var output bytes.Buffer
	opts := quantifierInferenceOptions{arithmeticMode: arithmeticPatternsNo, noPatternWeight: -1,
		pullQuantifiers: true, warnings: true, warningService: diagnostic.NewWarningService(&output)}
	ref, changed, err := c.reduceQuantifierNoDatabase(outer, body, nil, opts)
	patternMust(t, err)
	if !changed {
		t.Fatal("Boolean application pull did not expose a pattern")
	}
	defer func() { patternMust(t, m.DelRef(&ref)) }()
	got, err := m.RefGet(ref)
	patternMust(t, err)
	n, err := m.GetNumDeclarations(got)
	patternMust(t, err)
	weight, err := m.GetQuantifierWeight(got)
	patternMust(t, err)
	count, err := m.GetNumPatterns(got)
	patternMust(t, err)
	if n != 2 || weight != 4 || count != 1 || !strings.Contains(output.String(), "pulled nested quantifier") {
		t.Fatal("source application pull result and warning", n, weight, count, output.String())
	}
	pat, err := m.GetPattern(got, 0)
	patternMust(t, err)
	patTerm, err := m.GetArg(pat, 0)
	patternMust(t, err)
	if patTerm != term {
		t.Fatal("source pulled pattern term", patTerm, term)
	}
}

// AUDIT.md ast_sub_02-F3: pull_quant is a rewriter_tpl, so a shared
// subterm is pulled once and its fresh binder names are drawn once.
func TestPatternPullNoProofSharedSubtermPulledOnce(t *testing.T) {
	m := patternManager(t)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	b, err := m.MkBoolSort()
	patternMust(t, err)
	intern := func(s string) symbol.Symbol {
		t.Helper()
		v, e := m.Symbols.Intern(s)
		patternMust(t, e)
		return v
	}
	decl := func(name string) ast.ASTHandle {
		t.Helper()
		d, e := m.MkFuncDecl(intern(name), []ast.ASTHandle{b, b}, b, ast.ASTFuncDeclInfo{})
		patternMust(t, e)
		return pin(d)
	}
	p, q := decl("pull-share-p"), decl("pull-share-q")
	x, err := m.MkVar(0, b)
	patternMust(t, err)
	y, err := m.MkVar(1, b)
	patternMust(t, err)
	inner := func(f ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		body, e := m.MkApp(f, []ast.ASTHandle{x, y})
		patternMust(t, e)
		forall, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{intern("x")}, Body: body})
		patternMust(t, e)
		return pin(forall)
	}
	shared, err := m.MkOr(inner(p), inner(q))
	patternMust(t, err)
	pin(shared)
	r1, err := m.MkConst(intern("pull-share-r1"), b)
	patternMust(t, err)
	r2, err := m.MkConst(intern("pull-share-r2"), b)
	patternMust(t, err)
	left, err := m.MkAnd(shared, r1)
	patternMust(t, err)
	right, err := m.MkAnd(shared, r2)
	patternMust(t, err)
	disjunction, err := m.MkOr(left, right)
	patternMust(t, err)
	root, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{intern("y")}, Body: disjunction})
	patternMust(t, err)
	pin(root)
	c, err := newPatternCandidates(m, 0, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })
	pulled, err := c.pullNoProof(root)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, m.DelRef(&pulled)) })
	got, err := m.RefGet(pulled)
	patternMust(t, err)
	n, err := m.GetNumDeclarations(got)
	patternMust(t, err)
	var names []string
	for i := uint32(0); i < n; i++ {
		name, e := m.GetDeclarationName(got, i)
		patternMust(t, e)
		text, e := m.Symbols.Display(name)
		patternMust(t, e)
		names = append(names, text)
	}
	// The C++ trace prints (y x!0!2 x!1 x!0 x) for this shape.
	if want := "y x!0!2 x!1 x!0 x"; strings.Join(names, " ") != want {
		t.Fatalf("pulled binder names %q, want %q", strings.Join(names, " "), want)
	}
}
