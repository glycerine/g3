package pattern

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPatternCollectorQueuedWideApplicationsAcrossBinders(t *testing.T) {
	m := patternManager(t)
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		patternMust(t, err)
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	refCount := func(h ast.ASTHandle) uint32 {
		t.Helper()
		n, err := m.GetRefCount(h)
		patternMust(t, err)
		return n
	}
	boolean := pin(m.MkBoolSort())
	outer := pin(m.MkVar(1, boolean))
	shifted := pin(m.MkVar(0, boolean))
	repeat := func(h ast.ASTHandle) []ast.ASTHandle {
		var args [8]ast.ASTHandle
		for i := range args {
			args[i] = h
		}
		return args[:]
	}
	f := pin(m.MkFuncDecl(symbol.Numerical(937), repeat(boolean), boolean, ast.ASTFuncDeclInfo{}))
	plain := pin(m.MkApp(f, repeat(outer)))
	mapped := pin(m.MkApp(f, repeat(shifted)))
	// The conjunction itself is forbidden as a trigger, so revisiting it
	// also exercises a completed cache entry with no collected info.
	group := pin(m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpAnd, nil, repeat(plain)))
	nested := pin(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean},
		Names: []symbol.Symbol{symbol.Numerical(938)}, Body: group,
	}))
	root := pin(m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpAnd, nil,
		[]ast.ASTHandle{group, nested, group, plain, nested, plain}))
	c, err := newPatternCandidates(m, 2, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })
	plainRefs, mappedRefs, groupRefs := refCount(plain), refCount(mapped), refCount(group)
	for i := 0; i < 2; i++ {
		patternMust(t, c.collect(root))
		if len(c.candidates) != 2 || c.candidates[0] != plain || c.candidates[1] != mapped {
			t.Fatalf("unexpected candidate order or binder mapping %v", c.candidates)
		}
		if refCount(plain) != plainRefs+1 || refCount(mapped) != mappedRefs+1 || refCount(group) != groupRefs {
			t.Fatal("temporary references survived collection")
		}
		patternMust(t, c.resetCandidates())
		if refCount(plain) != plainRefs || refCount(mapped) != mappedRefs {
			t.Fatal("candidate references survived reset")
		}
	}
}

func TestPatternInferenceCollectorNestedDepthForbiddenAndOwnership(t *testing.T) {
	m := patternManager(t)
	b, err := m.MkBoolSort()
	patternMust(t, err)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	v0, err := m.MkVar(0, b)
	patternMust(t, err)
	pin(v0)
	v1, err := m.MkVar(1, b)
	patternMust(t, err)
	pin(v1)
	decl := func(name uint32, flags ast.ASTFuncFlags) ast.ASTHandle {
		t.Helper()
		var info ast.ASTFuncDeclInfo
		if flags.Skolem {
			var e error
			info, e = m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, flags, false)
			patternMust(t, e)
			t.Cleanup(func() { patternMust(t, m.DelFuncDeclInfo(&info)) })
		}
		d, e := m.MkFuncDecl(symbol.Numerical(name), []ast.ASTHandle{b}, b, info)
		patternMust(t, e)
		return pin(d)
	}
	app := func(d, arg ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		h, e := m.MkApp(d, []ast.ASTHandle{arg})
		patternMust(t, e)
		return pin(h)
	}
	f, g := decl(801, ast.ASTFuncFlags{}), decl(802, ast.ASTFuncFlags{})
	s := decl(803, ast.ASTFuncFlags{Skolem: true})
	nestedBody := app(f, v1)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{symbol.Numerical(805)}, Body: nestedBody,
	})
	patternMust(t, err)
	pin(q)
	direct, skolem := app(g, v0), app(s, v0)
	if yes, e := m.IsSkolemDecl(s); e != nil || !yes {
		t.Fatal("source skolem declaration flag", yes, e)
	}
	if yes, e := m.IsSkolemDecl(f); e != nil || yes {
		t.Fatal("ordinary declaration flag", yes, e)
	}
	root, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpAnd, nil, []ast.ASTHandle{q, direct, skolem})
	patternMust(t, err)
	pin(root)
	c, err := newPatternCandidates(m, 1, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })
	patternMust(t, c.collect(root))
	if len(c.collectRefs) != 0 {
		t.Fatal("temporary collected references must be released")
	}
	if len(c.candidates) != 2 {
		t.Fatal("normalized nested candidate and direct app", c.candidates)
	}
	normalized := c.candidates[0]
	if normalized != app(f, v0) || c.candidates[1] != direct {
		t.Fatal("source candidate order and nested binder normalization", c.candidates)
	}
	if c.info[normalized].size != 2 || c.info[normalized].freeVars.count() != 1 {
		t.Fatal("normalized candidate metadata", c.info[normalized])
	}
	if _, found := c.info[skolem]; found {
		t.Fatal("non-ground skolem must be excluded")
	}
	patterns, err := c.selectPatterns(0)
	patternMust(t, err)
	if len(patterns) != 2 {
		t.Fatal("collected candidates must feed selection", len(patterns))
	}
	patternMust(t, c.close())
	for i := range patterns {
		if _, e := m.RefGet(patterns[i]); e != nil {
			t.Fatal("pattern result must survive collector and candidate cleanup", i, e)
		}
		patternMust(t, m.DelRef(&patterns[i]))
	}

	allowSkolem, err := newPatternCandidates(m, 1, nil, false)
	patternMust(t, err)
	allowSkolem.avoidSkolems = false
	patternMust(t, allowSkolem.collect(skolem))
	if len(allowSkolem.candidates) != 1 || allowSkolem.candidates[0] != skolem {
		t.Fatal("disabled skolem avoidance must admit the source candidate", allowSkolem.candidates)
	}
	patternMust(t, allowSkolem.close())

	excluded, err := newPatternCandidates(m, 1, nil, false)
	patternMust(t, err)
	excluded.forbiddenFamilies[ast.NullFamilyID] = struct{}{}
	patternMust(t, excluded.collect(direct))
	if len(excluded.candidates) != 0 {
		t.Fatal("forbidden declaration family must suppress nonground app", excluded.candidates)
	}
	patternMust(t, excluded.close())

	boundBody := app(f, v0)
	boundQ, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{symbol.Numerical(806)}, Body: boundBody,
	})
	patternMust(t, err)
	pin(boundQ)
	bound, err := newPatternCandidates(m, 1, nil, false)
	patternMust(t, err)
	patternMust(t, bound.collect(boundQ))
	if len(bound.candidates) != 0 {
		t.Fatal("inner-bound variable must not become an outer candidate", bound.candidates)
	}
	patternMust(t, bound.close())
}

func TestPatternInferenceCollectorArithmeticNestedAndDivisionException(t *testing.T) {
	m := patternManager(t)
	family, err := m.RegisterArithmeticPlugin()
	patternMust(t, err)
	ints, err := m.MkFamilySort(family, 1, nil)
	patternMust(t, err)
	x, err := m.MkVar(0, ints)
	patternMust(t, err)
	patternMust(t, m.IncRef(x))
	t.Cleanup(func() { patternMust(t, m.DecRef(x)) })
	add, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{x, x})
	patternMust(t, err)
	patternMust(t, m.IncRef(add))
	t.Cleanup(func() { patternMust(t, m.DecRef(add)) })
	mul, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{x, x})
	patternMust(t, err)
	patternMust(t, m.IncRef(mul))
	t.Cleanup(func() { patternMust(t, m.DecRef(mul)) })
	for _, row := range []struct {
		name  string
		body  ast.ASTHandle
		allow bool
		want  int
	}{
		{"nested-add", add, true, 0},
		{"nested-mul", mul, true, 0},
		{"nonnested-add", add, false, 0},
		{"nonnested-mul", mul, false, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			c, e := newPatternCandidates(m, 1, nil, false)
			patternMust(t, e)
			c.nestedArithOnly = row.allow
			patternMust(t, c.collect(row.body))
			if len(c.candidates) != row.want {
				t.Fatal("source arithmetic candidate exception", c.candidates, row.want)
			}
			if row.want == 1 && c.candidates[0] != mul {
				t.Fatal("only multiplication exception may be selected", c.candidates)
			}
			patternMust(t, c.close())
		})
	}
}
