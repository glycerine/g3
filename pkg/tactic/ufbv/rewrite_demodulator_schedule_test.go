package ufbv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRewriteDemodulatorRescheduleMatchesAnnotationsAndTransfersRoot(t *testing.T) {
	m := cacheFixture(t)
	b, e := m.MkBoolSort()
	cacheMust(t, e)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		cacheMust(t, m.IncRef(h))
		t.Cleanup(func() { cacheMust(t, m.DecRef(h)) })
		return h
	}
	decl := func(name uint32, arity int) ast.ASTHandle {
		t.Helper()
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = b
		}
		h, e := m.MkFuncDecl(symbol.Numerical(name), domain, b, ast.ASTFuncDeclInfo{})
		cacheMust(t, e)
		return pin(h)
	}
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		h, e := m.MkApp(d, args)
		cacheMust(t, e)
		return pin(h)
	}
	x, e := m.MkVar(0, b)
	cacheMust(t, e)
	pin(x)
	f, g, ad, bd := decl(71, 1), decl(72, 1), decl(73, 0), decl(74, 0)
	a, c := app(ad), app(bd)
	fa, fc, gx := app(f, a), app(f, c), app(g, x)
	quant := func(name uint32, body ast.ASTHandle, noPattern ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		spec := ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{b},
			Names: []symbol.Symbol{symbol.Numerical(name)}, Body: body,
		}
		if !noPattern.IsNull() {
			spec.NoPatterns = []ast.ASTHandle{noPattern}
		}
		h, e := m.MkQuantifier(spec)
		cacheMust(t, e)
		return pin(h)
	}
	qAnnotation := quant(75, gx, fa)
	qDifferent := quant(76, fc, ast.ASTHandle{})
	qProcessed := quant(77, fa, ast.ASTHandle{})
	baseQ, e := m.GetRefCount(qAnnotation)
	cacheMust(t, e)
	forward, e := newRewriteRuleIndex(m)
	cacheMust(t, e)
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("demodulator")
	forward.trace = tags
	t.Cleanup(func() { cacheMust(t, forward.close()) })
	cacheMust(t, forward.insert(qAnnotation, gx, fa))
	cacheMust(t, forward.insert(qDifferent, gx, fc))
	back, e := newRewriteBackIndex(m)
	cacheMust(t, e)
	t.Cleanup(func() { cacheMust(t, back.close()) })
	cacheMust(t, back.addDemodulator(forward, qAnnotation))
	cacheMust(t, back.addDemodulator(forward, qDifferent))
	cacheMust(t, back.addProcessed(qProcessed))
	cacheMust(t, back.addProcessed(fa))
	if back.byDecl[f].values.Len() != 4 {
		t.Fatal("back index missed quantified annotation or processed occurrences", back.byDecl[f].values.Len())
	}
	cacheMust(t, back.rescheduleDemodulators(forward, f, fa))
	if tags.ShouldTrace("demodulator") && !strings.Contains(trace.String(), "reschedule_demodulators src/tactic/ufbv/ufbv_rewriter.cpp:637 ---------\nRescheduling: \n") {
		t.Fatal("source matched-quantifier rescheduling trace", trace.String())
	}
	if _, found := forward.rules[qAnnotation]; found {
		t.Fatal("matching annotation demodulator remained in forward index")
	}
	if _, found := forward.rules[qDifferent]; !found {
		t.Fatal("nonmatching demodulator was removed")
	}
	if len(back.todo) != 1 || back.todo[0] != qAnnotation || back.byDecl[f].values.Len() != 3 {
		t.Fatal("rescheduling must move only the matched quantified occurrence", back.todo, back.byDecl[f].values.Len())
	}
	if !ruleIndexConsistent(forward) {
		t.Fatal("forward index inconsistent after rescheduling")
	}
	refs, e := m.GetRefCount(qAnnotation)
	cacheMust(t, e)
	if refs != baseQ+1 {
		t.Fatal("removed rule root must transfer to main todo", refs, baseQ)
	}
	cacheMust(t, back.close())
	refs, e = m.GetRefCount(qAnnotation)
	cacheMust(t, e)
	if refs != baseQ {
		t.Fatal("main todo leaked rescheduled quantifier", refs, baseQ)
	}
	cacheMust(t, forward.close())
}
