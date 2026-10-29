package rewriter

// Source: src/ast/rewriter/der.cpp:100,140,344-359.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestDERExistentialSourceTracePipeline(t *testing.T) {
	f := newFactorFixture(t)
	bindBVTraceGlobals(t, f.m)
	d, err := NewDER(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	x, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	equality, err := f.m.MkEq(x, f.numeral(5))
	astTestMust(t, err)
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	p, err := f.m.MkConst(astTestSymbol(t, f.m, "der-trace-p"), boolean)
	astTestMust(t, err)
	body, err := f.m.MkAndArray([]ast.ASTHandle{equality, p})
	astTestMust(t, err)
	q := derQuantifier(t, f.m, false, []ast.ASTHandle{f.sort}, body)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("der")
	tags.Enable("top_sort")
	astTestMust(t, d.BindTrace(tags))
	derReduce(t, d, f.m, q)
	if !tags.ShouldTrace("der") {
		return
	}
	trace := output.String()
	for _, want := range []string{"der.cpp:140", "der.cpp:100", "der.cpp:344", "DEFINITIONS:", "VAR 0 = 5", "der.cpp:356", "Elimination m_order:\n0\n"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("missing trace %q in %q", want, trace)
		}
	}
	output.Reset()
	tags.Enable("der_bug")
	noCandidate := derQuantifier(t, f.m, false, []ast.ASTHandle{f.sort}, p)
	if got := derReduce(t, d, f.m, noCandidate); got != noCandidate {
		t.Fatal("candidate-free quantifier changed", got, noCandidate)
	}
	if tags.ShouldTrace("der_bug") {
		trace = output.String()
		for _, want := range []string{"der.cpp:229", "Did not find any diseq", "der-trace-p"} {
			if !strings.Contains(trace, want) {
				t.Fatalf("missing no-candidate trace %q in %q", want, trace)
			}
		}
	}
}
