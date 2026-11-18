//go:build g3trace

package bv

import (
	"bytes"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"strings"
	"testing"
)

func TestBVArray2UFSourceCreationTrace(t *testing.T) {
	f := newArrayFixture(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("bvarray2uf_rw")
	arrayMust(t, f.tactic.BindTrace(tags))
	_, _, e := f.tactic.rewriter.rewrite(f.app(t, ast.ASTArrayOpSelect, f.a, f.i))
	arrayMust(t, e)
	text := output.String()
	for _, want := range []string{"for array-a", "new func_decl is", "f_t"} {
		if !strings.Contains(text, want) {
			t.Fatal(want, text)
		}
	}
}

// bvarray2uf_tactic::imp::operator() traces the goal before and after the
// rewrite under the bvarray2uf tag.
func TestBVArray2UFTacticBeforeAfterTrace(t *testing.T) {
	f := newArrayFixture(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("bvarray2uf")
	arrayMust(t, f.tactic.BindTrace(tags))
	g := f.goal(t, false, false, false, f.app(t, ast.ASTArrayOpSelect, f.a, f.i))
	var out tactic.TacticResult
	t.Cleanup(func() { arrayMust(t, out.Close()) })
	arrayMust(t, f.tactic.Apply(g, &out))
	text := output.String()
	before := strings.Index(text, "[bvarray2uf] operator() src/tactic/bv/bvarray2uf_tactic.cpp")
	if before < 0 || !strings.Contains(text, "Before: \n(goal") || !strings.Contains(text, "After: \n(goal") || strings.Index(text, "After: ") < strings.Index(text, "Before: ") {
		t.Fatal("missing Before:/After: goal traces", text)
	}
}
