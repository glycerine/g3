package ufbv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestUFBVFormulaProcessorReschedulesEarlierFormulaAndPublishesResults(t *testing.T) {
	m := cacheFixture(t)
	b, e := m.MkBoolSort()
	cacheMust(t, e)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		cacheMust(t, m.IncRef(h))
		t.Cleanup(func() { cacheMust(t, m.DecRef(h)) })
		return h
	}
	x, e := m.MkVar(0, b)
	cacheMust(t, e)
	pin(x)
	f, e := m.MkFuncDecl(symbol.Numerical(301), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	cacheMust(t, e)
	pin(f)
	aDecl, e := m.MkFuncDecl(symbol.Numerical(302), nil, b, ast.ASTFuncDeclInfo{})
	cacheMust(t, e)
	pin(aDecl)
	a, e := m.MkApp(aDecl, nil)
	cacheMust(t, e)
	pin(a)
	fx, e := m.MkApp(f, []ast.ASTHandle{x})
	cacheMust(t, e)
	pin(fx)
	fa, e := m.MkApp(f, []ast.ASTHandle{a})
	cacheMust(t, e)
	pin(fa)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{symbol.Numerical(303)}, Body: fx})
	cacheMust(t, e)
	pin(q)
	truth, e := m.MkTrue()
	cacheMust(t, e)
	and, e := m.MkAnd(truth, fa)
	cacheMust(t, e)
	pin(and)
	boolean := ufbvBooleanFixture(t, m)
	processor, e := newFormulaProcessor(m, boolean)
	cacheMust(t, e)
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("demodulator")
	tags.Enable("demodulator_s")
	tags.Enable("demodulator_fwd")
	processor.trace = tags
	t.Cleanup(func() { cacheMust(t, processor.close()) })
	beforeQRefs, e := m.GetRefCount(q)
	cacheMust(t, e)
	// The initial LIFO queue processes `and` before it discovers q's rule.
	// That rule reschedules the already processed expression for rewriting.
	output, e := processor.run([]ast.ASTHandle{q, and})
	cacheMust(t, e)
	if len(output) != 2 || processor.back.processed.Len() != 1 || len(processor.index.rules) != 1 || len(processor.back.todo) != 0 {
		t.Fatal("source processed/rule partition and empty main todo", len(output), processor.back.processed.Len(), len(processor.index.rules), processor.back.todo)
	}
	seen := map[ast.ASTHandle]bool{}
	for _, h := range output {
		seen[h] = true
	}
	if !seen[truth] || !seen[q] || len(seen) != 2 {
		t.Fatal("rescheduled formula did not simplify or rule not published", seen)
	}
	if tags.ShouldTrace("demodulator") && (!strings.Contains(trace.String(), "ufbv_rewriter.cpp:702 ---------\n") || !strings.Contains(trace.String(), "ufbv_rewriter.cpp:706 ---------\n") || !strings.Contains(trace.String(), "ufbv_rewriter.cpp:748 ---------\n") || !strings.Contains(trace.String(), "after demodulator:\n")) {
		t.Fatal("source demodulator publication and final trace", trace.String())
	}
	if tags.ShouldTrace("demodulator_fwd") && !strings.Contains(trace.String(), "insert_fwd_idx src/tactic/ufbv/ufbv_rewriter.cpp:220 ---------\nINSERT: ") {
		t.Fatal("source forward-index insertion trace", trace.String())
	}
	if e := processor.close(); e != nil {
		t.Fatal(e)
	}
	afterCloseRefs, e := m.GetRefCount(q)
	cacheMust(t, e)
	if afterCloseRefs != beforeQRefs+1 {
		t.Fatal("published quantifier lost its independent output pin", beforeQRefs, afterCloseRefs)
	}
	for _, h := range output {
		if _, e := m.GetKind(h); e != nil {
			t.Fatal("published result must outlive processor", e)
		}
	}
	for _, h := range output {
		cacheMust(t, m.DecRef(h))
	}
	afterOutputRefs, e := m.GetRefCount(q)
	cacheMust(t, e)
	if afterOutputRefs != beforeQRefs {
		t.Fatal("output slice retained a quantifier pin", beforeQRefs, afterOutputRefs)
	}
}
