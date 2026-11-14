package rewriter

// Source: src/ast/rewriter/rewriter.cpp:222-416;
// src/ast/rewriter/rewriter.h:123-191.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func shiftTestVar(t *testing.T, m *ast.ASTManager, i uint32) ast.ASTHandle {
	t.Helper()
	b, e := m.MkBoolSort()
	satOK(t, e)
	v, e := m.MkVar(i, b)
	satOK(t, e)
	astTestPin(t, m, v)
	return v
}
func shiftTestIndices(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, want []uint32) {
	t.Helper()
	n, e := m.GetNumArgs(h)
	satOK(t, e)
	if n != uint32(len(want)) {
		t.Fatal("source arity", n, len(want))
	}
	for i, v := range want {
		a, e := m.GetArg(h, uint32(i))
		satOK(t, e)
		index, e := m.GetVarIndex(a)
		satOK(t, e)
		if index != v {
			t.Fatal("independent expected variable index", i, index, v)
		}
	}
}
func TestVariableShifterBoundedIndicesInverseAndUnsignedWrap(t *testing.T) {
	m := astTestManager(t)
	s, e := newVarShifter(m)
	satOK(t, e)
	defer func() { satOK(t, s.Close()) }()
	inv, e := newInvVarShifter(m)
	satOK(t, e)
	defer func() { satOK(t, inv.Close()) }()
	v0, v1, v2 := shiftTestVar(t, m, 0), shiftTestVar(t, m, 1), shiftTestVar(t, m, 2)
	original := goalTestApp(t, m, ast.ASTOpOr, v0, v1, v2)
	out := astTestPin(t, m, ast.ASTHandle{})
	back := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, s.Shift(original, 1, 4, 2, out))
	got, e := m.RefGet(out)
	satOK(t, e)
	shiftTestIndices(t, m, got, []uint32{2, 5, 6})
	satOK(t, s.ShiftBy(original, 3, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	shiftTestIndices(t, m, got, []uint32{3, 4, 5})
	satOK(t, inv.InverseShift(got, 3, back))
	got, e = m.RefGet(back)
	satOK(t, e)
	if got != original {
		t.Fatal("source round trip canonical identity")
	}
	satOK(t, s.Shift(original, 2, 0, 0, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != original {
		t.Fatal("zero shift changed canonical term")
	}
	maximum := shiftTestVar(t, m, ^uint32(0))
	satOK(t, s.ShiftBy(maximum, 1, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != v0 {
		t.Fatal("source unsigned forward wrap", got, v0)
	}
}

func TestVariableShifterNestedBindersPatternsAndAnnotations(t *testing.T) {
	m := astTestManager(t)
	s, e := newVarShifter(m)
	satOK(t, e)
	defer func() { satOK(t, s.Close()) }()
	inv, e := newInvVarShifter(m)
	satOK(t, e)
	defer func() { satOK(t, inv.Close()) }()
	b, e := m.MkBoolSort()
	satOK(t, e)
	v := []ast.ASTHandle{shiftTestVar(t, m, 0), shiftTestVar(t, m, 1), shiftTestVar(t, m, 2), shiftTestVar(t, m, 3), shiftTestVar(t, m, 4)}
	body := goalTestApp(t, m, ast.ASTOpOr, v...)
	inner, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b, b}, Names: []usymbol.Symbol{usymbol.Numerical(1), usymbol.Numerical(2)}, Body: body})
	satOK(t, e)
	astTestPin(t, m, inner)
	annotation := goalTestApp(t, m, ast.ASTOpOr, v[:3]...)
	pattern, e := m.MkFamilyApp(2, 0, nil, []ast.ASTHandle{annotation})
	satOK(t, e)
	astTestPin(t, m, pattern)
	qid, skid := astTestSymbol(t, m, "shift-qid"), astTestSymbol(t, m, "shift-skid")
	for _, patterns := range []bool{false, true} {
		t.Run(fmt.Sprint(patterns), func(t *testing.T) {
			spec := ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: inner, Weight: 17, QID: qid, SkolemID: skid}
			if patterns {
				spec.Patterns = []ast.ASTHandle{pattern}
			} else {
				spec.NoPatterns = []ast.ASTHandle{annotation}
			}
			original, e := m.MkQuantifier(spec)
			satOK(t, e)
			astTestPin(t, m, original)
			out := astTestPin(t, m, ast.ASTHandle{})
			satOK(t, s.Shift(original, 1, 4, 2, out))
			got, e := m.RefGet(out)
			satOK(t, e)
			nested, e := m.GetQuantifierBody(got)
			satOK(t, e)
			shiftedBody, e := m.GetQuantifierBody(nested)
			satOK(t, e)
			shiftTestIndices(t, m, shiftedBody, []uint32{0, 1, 2, 5, 8})
			var shiftedAnnotation ast.ASTHandle
			if patterns {
				pat, e := m.GetPattern(got, 0)
				satOK(t, e)
				shiftedAnnotation, e = m.GetArg(pat, 0)
				satOK(t, e)
			} else {
				shiftedAnnotation, e = m.GetNoPattern(got, 0)
				satOK(t, e)
			}
			shiftTestIndices(t, m, shiftedAnnotation, []uint32{0, 3, 6})
			weight, e := m.GetQuantifierWeight(got)
			satOK(t, e)
			forall, e := m.IsForall(got)
			satOK(t, e)
			a, z, e := m.GetQuantifierIDs(got)
			satOK(t, e)
			name, e := m.GetDeclarationName(got, 0)
			satOK(t, e)
			if weight != 17 || forall || a != qid || z != skid || name != usymbol.Numerical(0) {
				t.Fatal("original quantifier annotations changed")
			}
			if len(s.core.scopes) != 0 || s.core.numQVars != 0 || s.core.cache != s.core.cacheStack[0] {
				t.Fatal("source quantifier scope not retired")
			}
			satOK(t, s.ShiftBy(original, 3, out))
			got, e = m.RefGet(out)
			satOK(t, e)
			back := astTestPin(t, m, ast.ASTHandle{})
			satOK(t, inv.InverseShift(got, 3, back))
			restored, e := m.RefGet(back)
			satOK(t, e)
			if restored != original {
				t.Fatal("nested quantifier and annotation round trip")
			}
		})
	}
}

func TestVariableShifterSharedCacheGroundShortcutAndDeepTraversal(t *testing.T) {
	m := astTestManager(t)
	s, e := newVarShifter(m)
	satOK(t, e)
	defer func() { satOK(t, s.Close()) }()
	v := shiftTestVar(t, m, 1)
	shared := goalTestApp(t, m, ast.ASTOpNot, v)
	original := goalTestApp(t, m, ast.ASTOpOr, shared, shared)
	out := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, s.ShiftBy(original, 2, out))
	got, e := m.RefGet(out)
	satOK(t, e)
	a, e := m.GetArg(got, 0)
	satOK(t, e)
	b, e := m.GetArg(got, 1)
	satOK(t, e)
	if a != b {
		t.Fatal("shared rewritten child identity")
	}
	size, e := s.core.getCacheSize()
	satOK(t, e)
	if size != 1 {
		t.Fatal("source shared non-root cache", size)
	}
	truth, e := m.MkTrue()
	satOK(t, e)
	groundOut := astTestPin(t, m, truth)
	m.BudgetField.FailAfter(0)
	e = s.Shift(truth, 99, 88, 77, groundOut)
	m.BudgetField.FailAfter(-1)
	satOK(t, e)
	size, e = s.core.getCacheSize()
	satOK(t, e)
	if size != 1 || s.bound != 0 || s.shift1 != 2 || s.shift2 != 0 {
		t.Fatal("ground shortcut reset cache or shift parameters")
	}
	deep := v
	for i := 0; i < 1000; i++ {
		deep = goalTestApp(t, m, ast.ASTOpNot, deep)
	}
	satOK(t, s.ShiftBy(deep, 3, out))
	got, e = m.RefGet(out)
	satOK(t, e)
	for i := 0; i < 1000; i++ {
		yes, next, e := m.IsNot(got)
		satOK(t, e)
		if !yes {
			t.Fatal("deep source traversal shape", i)
		}
		got = next
	}
	index, e := m.GetVarIndex(got)
	satOK(t, e)
	if index != 4 {
		t.Fatal("deep source variable index", index)
	}
	n := uint32(len(s.core.results))
	if n != 0 || len(s.core.frames) != 0 {
		t.Fatal("source main loop left results or frames")
	}
}

func TestInverseVariableShifterNativeWrapAndDebugBoundary(t *testing.T) {
	m := astTestManager(t)
	s, e := newInvVarShifter(m)
	satOK(t, e)
	defer func() { satOK(t, s.Close()) }()
	v := shiftTestVar(t, m, 0)
	out := astTestPin(t, m, v)
	e = s.InverseShift(v, 1, out)
	if ast.ASTSourceDebug {
		if e != ast.ErrASTDomain {
			t.Fatal("source inverse shift assertion", e)
		}
	} else {
		satOK(t, e)
		h, e := m.RefGet(out)
		satOK(t, e)
		index, e := m.GetVarIndex(h)
		satOK(t, e)
		if index != ^uint32(0) {
			t.Fatal("source native unsigned inverse wrap", index)
		}
	}
	copyOwner := *s
	if e = copyOwner.InverseShift(v, 0, out); e != ast.ErrASTHandle {
		t.Fatal("copied shifter owner", e)
	}
	satOK(t, s.Close())
	if e = s.InverseShift(v, 0, out); e != ast.ErrASTHandle {
		t.Fatal("closed shifter owner", e)
	}
}

func TestVariableShifterSharedTermDifferentBinderDepthAndOutputLifetime(t *testing.T) {
	m := astTestManager(t)
	s, e := newVarShifter(m)
	satOK(t, e)
	v0, v1 := shiftTestVar(t, m, 0), shiftTestVar(t, m, 1)
	shared := goalTestApp(t, m, ast.ASTOpOr, v0, v1)
	b, e := m.MkBoolSort()
	satOK(t, e)
	quant, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: shared})
	satOK(t, e)
	astTestPin(t, m, quant)
	original := goalTestApp(t, m, ast.ASTOpOr, shared, quant, quant)
	out := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, s.ShiftBy(original, 2, out))
	shifted, e := m.RefGet(out)
	satOK(t, e)
	first, e := m.GetArg(shifted, 0)
	satOK(t, e)
	shiftTestIndices(t, m, first, []uint32{2, 3})
	second, e := m.GetArg(shifted, 1)
	satOK(t, e)
	third, e := m.GetArg(shifted, 2)
	satOK(t, e)
	if second != third {
		t.Fatal("cached quantifier identity")
	}
	body, e := m.GetQuantifierBody(second)
	satOK(t, e)
	shiftTestIndices(t, m, body, []uint32{0, 3})
	// The output owns every rebuilt node independently of cache teardown.
	satOK(t, s.Close())
	shiftTestIndices(t, m, first, []uint32{2, 3})
	shiftTestIndices(t, m, body, []uint32{0, 3})
	counts := make([]uint32, 2)
	for i, h := range []ast.ASTHandle{v0, v1} {
		counts[i], e = m.GetRefCount(h)
		satOK(t, e)
	}
	satOK(t, m.ResetRef(out))
	after, e := m.GetRefCount(v0)
	satOK(t, e)
	if after != counts[0]-1 {
		t.Fatal("shifted bound variable output ownership", after, counts[0])
	}
	after, e = m.GetRefCount(v1)
	satOK(t, e)
	if after != counts[1] {
		t.Fatal("shifted free variable retained original", after, counts[1])
	}
	if m.IsLiveHandle(shifted) {
		t.Fatal("unretained rebuilt output survived final release")
	}
}
