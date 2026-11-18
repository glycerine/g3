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
	// compute_dominators' m_doms dump, then simplify_rec's entry and exit
	// records around simplify_arg's.
	for _, want := range []string{
		"[simplify] compute_dominators src/tactic/core/dom_simplify_tactic.cpp:94 ---------\n(and dom-a (or (not dom-a) dom-b)) |-> (and dom-a (or (not dom-a) dom-b))\ndom-a |-> ",
		"[simplify] simplify_rec src/tactic/core/dom_simplify_tactic.cpp:267 ---------\ndepth: 0 (or (not dom-a) dom-b)\n",
		"[simplify] simplify_arg src/tactic/core/dom_simplify_tactic.cpp:256 ---------\ndepth: 1 (not dom-a) -> (not dom-a)\n",
		"[simplify] simplify_rec src/tactic/core/dom_simplify_tactic.cpp:306 ---------\ndepth: 1 dom-a -> dom-a\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing source trace %q\n%s", want, text)
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
