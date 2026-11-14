package rewriter

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestPB2BVParametersSurviveCleanupReplacement(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.RegisterDeclarationPlugins())
	c, p := enum2BVParameters(t, m)
	key := astTestSymbol(t, m, "pb-constructor-parameter")
	astTestMust(t, c.SetUInt(p, key, 17))
	r, err := NewPB2BVRewriter(m, c, p)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	if r.parameters != *p || c.Stats().References != 2 {
		t.Fatal("constructor did not acquire shared parameter copy", c.Stats())
	}
	astTestMust(t, c.SetUInt(p, key, 29))
	astTestMust(t, r.UpdateParams(p))
	astTestMust(t, c.Del(p))
	value, err := c.GetUInt(&r.parameters, key, 0)
	astTestMust(t, err)
	if value != 29 {
		t.Fatal("source parameter append omitted update", value)
	}
	m.Budget().FailAfter(0)
	err = r.Cleanup()
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || r.header != 0 || r.parameters != (uparams.ParamsRef{}) || c.Stats().References != 1 {
		t.Fatal("replacement lost local parameter owner", err, c.Stats())
	}
	value, err = c.GetUInt(&r.cleanupParameters, key, 0)
	astTestMust(t, err)
	if value != 29 {
		t.Fatal("cleanup local snapshot", value)
	}
	astTestMust(t, r.Cleanup())
	value, err = c.GetUInt(&r.parameters, key, 0)
	astTestMust(t, err)
	if value != 29 || r.cleanupParameters != (uparams.ParamsRef{}) || c.Stats().References != 1 {
		t.Fatal("reconstruction did not retire temporary", value, c.Stats())
	}
	astTestMust(t, r.Close())
	if c.Stats().References != 0 || c.Stats().LiveBlocks != 0 {
		t.Fatal("parameter member remained owned", c.Stats())
	}
}
func TestPB2BVTraversalAlwaysPreservesCallerProof(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.RegisterDeclarationPlugins())
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	c, p := enum2BVParameters(t, m)
	r, err := NewPB2BVRewriter(m, c, p)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	a := goalTestAtom(t, m, "pb-source-a")
	b := goalTestAtom(t, m, "pb-source-b")
	d := goalTestAtom(t, m, "pb-source-c")
	h, err := m.MkPseudoBooleanAtMost([]ast.ASTHandle{a, b, d}, 1)
	astTestMust(t, err)
	astTestPin(t, m, h)
	pr, err := m.MkProofAsserted(a)
	astTestMust(t, err)
	proof := astTestPin(t, m, pr)
	out := astTestPin(t, m, a)
	for pass := 0; pass < 2; pass++ {
		astTestMust(t, r.Rewrite(h, out, proof))
		actual, err := m.RefGet(proof)
		astTestMust(t, err)
		result, err := m.RefGet(out)
		astTestMust(t, err)
		if actual != pr || result == h {
			t.Fatal("non-proof traversal changed caller proof or omitted rewrite", actual, result)
		}
		astTestMust(t, r.Cleanup())
	}
}

func TestPB2BVFreshDeclarationRetirementFollowsSourceRange(t *testing.T) {
	base := astTestManager(t)
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	astTestMust(t, m.RegisterDeclarationPlugins())
	c, p := enum2BVParameters(t, m)
	r, err := NewPB2BVRewriter(m, c, p)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	var vars []ast.ASTHandle
	for i := 0; i < 7; i++ {
		vars = append(vars, goalTestAtom(t, m, fmt.Sprint("pb-retire-", i)))
	}
	h, err := m.MkPseudoBooleanAtMost(vars, 2)
	astTestMust(t, err)
	astTestPin(t, m, h)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	astTestMust(t, m.DelRef(&out))
	fresh, err := r.FreshConstants()
	astTestMust(t, err)
	if len(fresh) < 2 {
		t.Fatal("fixture needs multiple fresh declarations")
	}
	var ids []uint32
	for _, d := range fresh {
		id, e := m.GetASTID(d)
		astTestMust(t, e)
		ids = append(ids, id)
	}
	tags.Enable("ast")
	trace.Reset()
	astTestMust(t, r.Close())
	for _, d := range fresh {
		if m.IsLiveHandle(d) {
			t.Fatal("fresh declaration remained owned")
		}
	}
	if tags.ShouldTrace("ast") {
		previous := -1
		for _, id := range ids {
			pos := strings.Index(trace.String(), fmt.Sprintf("Deleting object %d ", id))
			if pos <= previous {
				t.Fatal("fresh vector did not retire in source forward order", ids, trace.String())
			}
			previous = pos
		}
	}
}
