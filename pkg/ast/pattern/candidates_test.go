package pattern

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPatternInferenceLoopFilterConcreteCandidateOwnership(t *testing.T) {
	m := patternManager(t)
	b, err := m.MkBoolSort()
	patternMust(t, err)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	x, err := m.MkVar(0, b)
	patternMust(t, err)
	pin(x)
	y, err := m.MkVar(1, b)
	patternMust(t, err)
	pin(y)
	declare := func(name uint32) ast.ASTHandle {
		t.Helper()
		h, e := m.MkFuncDecl(symbol.Numerical(name), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		patternMust(t, e)
		return pin(h)
	}
	app := func(decl, arg ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		h, e := m.MkApp(decl, []ast.ASTHandle{arg})
		patternMust(t, e)
		return pin(h)
	}
	f, g := declare(711), declare(712)
	general := app(f, x)
	specific := app(f, app(g, x))
	excluded := app(f, y)
	otherVars := app(g, y)
	base, err := m.GetRefCount(general)
	patternMust(t, err)
	filter, err := newPatternCandidates(m, 2, []ast.ASTHandle{excluded}, true)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, filter.close()) })
	patternMust(t, filter.addCandidate(general, []uint32{0}, 1))
	patternMust(t, filter.addCandidate(general, []uint32{0}, 99)) // Duplicate keeps first metadata and one pin.
	patternMust(t, filter.addCandidate(specific, []uint32{0}, 2))
	patternMust(t, filter.addCandidate(excluded, []uint32{1}, 1))
	patternMust(t, filter.addCandidate(otherVars, []uint32{1}, 1))
	if len(filter.candidates) != 3 || filter.info[general].size != 1 {
		t.Fatal("candidate insertion order, exclusion or duplicate metadata", filter.candidates, filter.info)
	}
	if refs, e := m.GetRefCount(general); e != nil || refs != base+1 {
		t.Fatal("candidate vector must own one AST reference", refs, base, e)
	}
	got, err := filter.filterLooping()
	patternMust(t, err)
	if len(got) != 2 || got[0] != specific || got[1] != otherVars {
		t.Fatal("strict same-free-variable filtering and source order", got)
	}
	if _, found := filter.info[general]; found || len(filter.candidates) != 3 {
		t.Fatal("source erases metadata but retains vector ownership", filter.info, filter.candidates)
	}
	patternMust(t, filter.close())
	if refs, e := m.GetRefCount(general); e != nil || refs != base {
		t.Fatal("candidate vector close must release its reference", refs, base, e)
	}

	pass, err := newPatternCandidates(m, 2, nil, false)
	patternMust(t, err)
	patternMust(t, pass.addCandidate(general, []uint32{0}, 1))
	patternMust(t, pass.addCandidate(specific, []uint32{0}, 2))
	got, err = pass.filterLooping()
	patternMust(t, err)
	if len(got) != 2 || got[0] != general || got[1] != specific {
		t.Fatal("disabled loop filtering must preserve candidate order", got)
	}
	patternMust(t, pass.close())
}

func TestPatternInferenceContainsSubpatternAndBiggerFilter(t *testing.T) {
	m := patternManager(t)
	b, err := m.MkBoolSort()
	patternMust(t, err)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	x, err := m.MkVar(0, b)
	patternMust(t, err)
	pin(x)
	y, err := m.MkVar(1, b)
	patternMust(t, err)
	pin(y)
	decl := func(name uint32, arity int) ast.ASTHandle {
		t.Helper()
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = b
		}
		h, e := m.MkFuncDecl(symbol.Numerical(name), domain, b, ast.ASTFuncDeclInfo{})
		patternMust(t, e)
		return pin(h)
	}
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		h, e := m.MkApp(d, args)
		patternMust(t, e)
		return pin(h)
	}
	g, h, f := decl(721, 1), decl(722, 1), decl(723, 2)
	first, last := app(g, x), app(h, x)
	root := app(f, first, last)
	wider := app(f, first, y)
	c, err := newPatternCandidates(m, 2, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })
	for _, item := range []struct {
		h    ast.ASTHandle
		vars []uint32
	}{
		{first, []uint32{0}}, {last, []uint32{0}},
		{root, []uint32{0}}, {wider, []uint32{0, 1}},
	} {
		patternMust(t, c.addCandidate(item.h, item.vars, 1))
	}
	if yes, e := c.containsSubpattern(root); e != nil || !yes {
		t.Fatal("root must contain its last-child candidate", yes, e)
	}
	if len(c.todo) != 1 || c.todo[0] != first {
		t.Fatal("source traversal must leave first child pending after last-child match", c.todo)
	}
	if yes, e := c.containsSubpattern(wider); e != nil || yes {
		t.Fatal("proper child with fewer free variables must not remove parent", yes, e)
	}
	if len(c.todo) != 0 {
		t.Fatal("full traversal must drain worklist", c.todo)
	}
	looped, err := c.filterLooping()
	patternMust(t, err)
	got, err := c.filterBigger(looped)
	patternMust(t, err)
	if len(got) != 3 || got[0] != first || got[1] != last || got[2] != wider {
		t.Fatal("bigger filter must keep source order and same-variable rule", got)
	}
	if _, found := c.info[root]; !found {
		t.Fatal("larger-pattern filter must not erase candidate metadata")
	}
	unknown := app(decl(724, 1), x)
	if _, e := c.containsSubpattern(unknown); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("source requires root metadata", e)
	}
	patternMust(t, c.close())
}

func (c *patternCandidates) addCandidate(n ast.ASTHandle, indices []uint32, size uint32) error {
	var vars patternVarSet
	for _, v := range indices {
		vars.insert(v)
	}
	return c.recordCandidate(n, vars, size)
}
