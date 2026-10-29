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
