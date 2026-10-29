//go:build g3trace

package bv

import (
	"bytes"
	"github.com/glycerine/g3/pkg/ast"
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
	_, e := f.tactic.rewriter.rewrite(f.app(t, ast.ASTArrayOpSelect, f.a, f.i))
	arrayMust(t, e)
	text := output.String()
	for _, want := range []string{"for array-a", "new func_decl is", "f_t"} {
		if !strings.Contains(text, want) {
			t.Fatal(want, text)
		}
	}
}
