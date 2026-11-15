package pattern

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPatternInferencePullDirectNestedForallFindsPattern(t *testing.T) {
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
	decl, err := m.MkFuncDecl(symbol.Numerical(940), []ast.ASTHandle{b, b}, b, ast.ASTFuncDeclInfo{})
	patternMust(t, err)
	pin(decl)
	body, err := m.MkApp(decl, []ast.ASTHandle{x, y})
	patternMust(t, err)
	pin(body)
	inner, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(942)}, Body: body, Weight: 3})
	patternMust(t, err)
	pin(inner)
	qid, err := m.Symbols.Intern("outer-pull-qid")
	patternMust(t, err)
	outer, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(941)}, Body: inner, Weight: 7, QID: qid})
	patternMust(t, err)
	pin(outer)
	c, err := newPatternCandidates(m, 0, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })
	var output bytes.Buffer
	opts := quantifierInferenceOptions{arithmeticMode: arithmeticPatternsNo, noPatternWeight: -1,
		pullQuantifiers: true, warnings: true, warningService: diagnostic.NewWarningService(&output)}
	ref, changed, err := c.reduceQuantifierNoDatabase(outer, inner, nil, opts)
	patternMust(t, err)
	if !changed {
		t.Fatal("direct nested universal quantifier was not pulled")
	}
	defer func() { patternMust(t, m.DelRef(&ref)) }()
	got, err := m.RefGet(ref)
	patternMust(t, err)
	n, err := m.GetNumDeclarations(got)
	patternMust(t, err)
	gotBody, err := m.GetQuantifierBody(got)
	patternMust(t, err)
	weight, err := m.GetQuantifierWeight(got)
	patternMust(t, err)
	gotQID, _, err := m.GetQuantifierIDs(got)
	patternMust(t, err)
	outerName, err := m.GetDeclarationName(got, 0)
	patternMust(t, err)
	innerName, err := m.GetDeclarationName(got, 1)
	patternMust(t, err)
	count, err := m.GetNumPatterns(got)
	patternMust(t, err)
	if n != 2 || gotBody != body || weight != 3 || gotQID != qid ||
		outerName != symbol.Numerical(941) || innerName != symbol.Numerical(942) || count != 1 {
		t.Fatal("source direct-pull binder, body, weight, ID or pattern", n, gotBody, weight, gotQID, outerName, innerName, count)
	}
	pat, err := m.GetPattern(got, 0)
	patternMust(t, err)
	term, err := m.GetArg(pat, 0)
	patternMust(t, err)
	if want := "WARNING: pulled nested quantifier to be able to find an useable pattern (quantifier id: outer-pull-qid)\n"; term != body || output.String() != want {
		t.Fatalf("source inferred pattern and warning: term=%v got %q, want %q", term, output.String(), want)
	}

	innerEmpty, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(942)}, Body: y, Weight: 3})
	patternMust(t, err)
	pin(innerEmpty)
	outerEmpty, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(941)}, Body: innerEmpty, Weight: 7, QID: qid})
	patternMust(t, err)
	pin(outerEmpty)
	output.Reset()
	noRef, noChange, err := c.reduceQuantifierNoDatabase(outerEmpty, innerEmpty, nil, opts)
	patternMust(t, err)
	if noChange || noRef != (ast.ASTRef{}) || !strings.Contains(output.String(), "failed to find a pattern") ||
		strings.Contains(output.String(), "pulled nested quantifier") {
		t.Fatal("unsuccessful source pull must keep the ordinary no-pattern result", noRef, noChange, output.String())
	}
	patternMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := m.NewRef(ast.ASTHandle{})
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, m.DelRef(&proof)) })
	proofRef, proofChanged, err := c.reduceQuantifierNoDatabaseWithProof(outer, inner, nil, opts, proof)
	patternMust(t, err)
	if !proofChanged {
		t.Fatal("proof direct pull was not published")
	}
	defer func() { patternMust(t, m.DelRef(&proofRef)) }()
	proofResult, err := m.RefGet(proofRef)
	patternMust(t, err)
	p, err := m.RefGet(proof)
	patternMust(t, err)
	fact, err := m.GetProofFact(p)
	patternMust(t, err)
	wantFact, err := m.MkIff(outer, proofResult)
	patternMust(t, err)
	if fact != wantFact {
		t.Fatal("pulled quantifier proof fact", fact, wantFact)
	}
	// AUDIT.md ast_sub_02-F2: result_pr = mk_transitivity(new_pr, ...)
	// where new_pr comes from the proof-producing pull_quant, not rewrite.
	if trans, e := m.IsProofKind(p, ast.ASTPrTransitivity); e != nil || !trans {
		t.Fatal("pulled quantifier proof is not a transitivity", trans, e)
	}
	first, err := m.GetProofParent(p, 0)
	patternMust(t, err)
	if pull, e := m.IsProofKind(first, ast.ASTPrPullQuant); e != nil || !pull {
		t.Fatal("pull step is not a pull-quant proof", pull, e)
	}
}
