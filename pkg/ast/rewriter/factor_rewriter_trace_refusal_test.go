package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp:56,77,104.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestFactorRewriterSourceTraceRefusalRelations(t *testing.T) {
	for _, tc := range []struct {
		op                 int32
		function, relation string
		line               int
	}{
		{-1, "mk_eq", " = ", 56},
		{factorOpLE, "mk_le", " <= ", 77},
		{factorOpLT, "mk_lt", " < ", 104},
	} {
		t.Run(tc.function, func(t *testing.T) {
			f := newFactorFixture(t)
			bindBVTraceGlobals(t, f.m)
			x, y, z := f.variable("refuse-x"), f.variable("refuse-y"), f.variable("refuse-z")
			left, right := f.app(factorOpAdd, x, y), f.app(factorOpAdd, x, z)
			_, decl := f.relation(tc.op, left, right)
			rw, err := NewFactorRewriter(f.m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, rw.Close()) })
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("factor_rewriter")
			astTestMust(t, rw.BindTrace(tags))
			sentinel, err := f.m.MkTrue()
			astTestMust(t, err)
			out := astTestPin(t, f.m, sentinel)
			status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{left, right}), out)
			astTestMust(t, err)
			got, err := f.m.RefGet(out)
			astTestMust(t, err)
			if status != RewriteFailed || got != sentinel {
				t.Fatal("refusal publication", status, got)
			}
			if tags.ShouldTrace("factor_rewriter") {
				header := tc.function + " src/ast/rewriter/factor_rewriter.cpp:" + fmt.Sprint(tc.line)
				if !strings.Contains(output.String(), header) || !strings.Contains(output.String(), tc.relation) {
					t.Fatal("refusal trace", output.String())
				}
			}
		})
	}
}
