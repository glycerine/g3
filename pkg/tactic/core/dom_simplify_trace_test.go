//go:build g3trace

package core

import (
	"bytes"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"strings"
	"testing"
)

func TestDomSimplifySourceTraceTreeAndRewrite(t *testing.T) {
	f := newDomFixture(t)
	na, e := f.m.MkNot(f.a)
	domMust(t, e)
	tail, e := f.m.MkOr(na, f.b)
	domMust(t, e)
	formula, e := f.m.MkAnd(f.a, tail)
	domMust(t, e)
	g := f.goal(t, false, formula)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("simplify")
	r, e := NewDomSimplifyTactic(f.m, nil)
	domMust(t, e)
	t.Cleanup(func() { domMust(t, r.Close()) })
	domMust(t, r.BindTrace(tags))
	var out tactic.TacticResult
	t.Cleanup(func() { domMust(t, out.Close()) })
	domMust(t, r.Apply(g, &out))
	text := output.String()
	for _, want := range []string{"dom-a", "dom-b", "(or (not dom-a) dom-b)"} {
		if !strings.Contains(text, want) {
			t.Fatal("missing source trace", want, text)
		}
	}
}

// expr_dominators::display prints a node, then its children on the same
// line, and only then the newline: root A with leaf child B is "A B\n\n".
func TestDomSimplifyDominatorTreeTraceLayout(t *testing.T) {
	f := newDomFixture(t)
	formula, e := f.m.MkAnd(f.a, f.b)
	domMust(t, e)
	g := f.goal(t, false, formula)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("simplify")
	r, e := NewDomSimplifyTactic(f.m, nil)
	domMust(t, e)
	t.Cleanup(func() { domMust(t, r.Close()) })
	domMust(t, r.BindTrace(tags))
	var out tactic.TacticResult
	t.Cleanup(func() { domMust(t, out.Close()) })
	domMust(t, r.Apply(g, &out))
	text := output.String()
	if !tags.ShouldTrace("simplify") {
		return
	}
	header := "[simplify] compile src/tactic/core/dom_simplify_tactic.cpp:"
	i := strings.Index(text, header)
	if i < 0 {
		t.Fatal("missing dominator tree trace", text)
	}
	body := text[strings.Index(text[i:], "\n")+i+1:]
	if !strings.HasPrefix(body, "(and dom-a dom-b) dom-") {
		t.Fatalf("dominator tree children must follow the parent on its line: %q", body)
	}
}
