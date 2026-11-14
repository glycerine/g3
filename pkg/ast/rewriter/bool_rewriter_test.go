package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:89-280;
// src/ast/rewriter/rewriter_types.h:29-49.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"

	"reflect"
	"testing"
)

func TestBoolRewriterACSourceStatusesOrderAndOutputOwnership(t *testing.T) {
	m := astTestManager(t)
	rw := boolRewriter{manager: m}
	p := goalTestAtom(t, m, "rw-p")
	q := goalTestAtom(t, m, "rw-q")
	np := goalTestApp(t, m, ast.ASTOpNot, p)
	truth, e := m.MkTrue()
	satOK(t, e)
	falsity, e := m.MkFalse()
	satOK(t, e)
	out := astTestPin(t, m, q)
	for _, or := range []bool{false, true} {
		t.Run(fmt.Sprint(or), func(t *testing.T) {
			unit, absorb := truth, falsity
			if or {
				unit, absorb = falsity, truth
			}
			for _, tc := range []struct {
				args      []ast.ASTHandle
				want      ast.ASTHandle
				status    brStatus
				remaining []ast.ASTHandle
			}{
				{nil, unit, brDone, nil},
				{[]ast.ASTHandle{p}, p, brDone, nil},
				{[]ast.ASTHandle{p, q}, q, brFailed, nil},
				{[]ast.ASTHandle{unit, p, unit}, p, brDone, nil},
				{[]ast.ASTHandle{q, p, q, p}, ast.ASTHandle{}, brDone, []ast.ASTHandle{q, p}},
				{[]ast.ASTHandle{np, np, q}, ast.ASTHandle{}, brDone, []ast.ASTHandle{np, q}},
				{[]ast.ASTHandle{p, np}, absorb, brDone, nil},
				{[]ast.ASTHandle{np, p}, absorb, brDone, nil},
				{[]ast.ASTHandle{p, absorb, q}, absorb, brDone, nil},
			} {
				satOK(t, m.SetRef(out, q))
				var status brStatus
				var e error
				if or {
					status, e = rw.mkNFlatOrCore(tc.args, out)
				} else {
					status, e = rw.mkNFlatAndCore(tc.args, out)
				}
				satOK(t, e)
				if status != tc.status {
					t.Fatal("source status", tc.args, status, tc.status)
				}
				got, e := m.RefGet(out)
				satOK(t, e)
				if tc.remaining == nil {
					if got != tc.want {
						t.Fatal("source result/failed output assignment", got, tc.want)
					}
				} else {
					n, e := m.GetNumArgs(got)
					satOK(t, e)
					var args []ast.ASTHandle
					for i := uint32(0); i < n; i++ {
						h, e := m.GetArg(got, i)
						satOK(t, e)
						args = append(args, h)
					}
					if !reflect.DeepEqual(args, tc.remaining) {
						t.Fatal("literal source order", args, tc.remaining)
					}
				}
			}
		})
	}
	// An output assigned from an unpinned input must own it independently.
	b, e := m.MkBoolSort()
	satOK(t, e)
	h, e := m.MkConst(astTestSymbol(t, m, "rw-owned"), b)
	satOK(t, e)
	status, e := rw.mkNFlatAndCore([]ast.ASTHandle{h, h}, out)
	satOK(t, e)
	if status != brDone {
		t.Fatal(status)
	}
	n, e := m.GetRefCount(h)
	satOK(t, e)
	if n != 1 {
		t.Fatal("output did not own singleton", n)
	}
	satOK(t, m.ResetRef(out))
	if m.IsLiveHandle(h) {
		t.Fatal("singleton output leaked")
	}
}

func TestBoolRewriterACOneLevelFlatteningAndSpill(t *testing.T) {
	m := astTestManager(t)
	rw := boolRewriter{manager: m}
	out := astTestPin(t, m, ast.ASTHandle{})
	for _, or := range []bool{false, true} {
		op := int32(ast.ASTOpAnd)
		if or {
			op = ast.ASTOpOr
		}
		p := goalTestAtom(t, m, fmt.Sprint("flat-p", or))
		q := goalTestAtom(t, m, fmt.Sprint("flat-q", or))
		r := goalTestAtom(t, m, fmt.Sprint("flat-r", or))
		deep := goalTestApp(t, m, op, p, q)
		nested := goalTestApp(t, m, op, deep, r)
		var status brStatus
		var e error
		if or {
			status, e = rw.mkFlatOrCore([]ast.ASTHandle{q, nested}, out)
		} else {
			status, e = rw.mkFlatAndCore([]ast.ASTHandle{q, nested}, out)
		}
		satOK(t, e)
		if status != brDone {
			t.Fatal(status)
		}
		got, e := m.RefGet(out)
		satOK(t, e)
		for i, want := range []ast.ASTHandle{q, deep, r} {
			h, e := m.GetArg(got, uint32(i))
			satOK(t, e)
			if h != want {
				t.Fatal("source depth-one flattening", i, h, want)
			}
		}
		var args []ast.ASTHandle
		for i := 0; i < 40; i++ {
			args = append(args, goalTestAtom(t, m, fmt.Sprintf("spill-%t-%d", or, i)))
		}
		child := goalTestApp(t, m, op, args...)
		if or {
			status, e = rw.mkFlatOrCore([]ast.ASTHandle{child, args[0]}, out)
		} else {
			status, e = rw.mkFlatAndCore([]ast.ASTHandle{child, args[0]}, out)
		}
		satOK(t, e)
		if status != brDone {
			t.Fatal(status)
		}
		got, e = m.RefGet(out)
		satOK(t, e)
		n, e := m.GetNumArgs(got)
		satOK(t, e)
		if n != 40 {
			t.Fatal("source buffer spill/duplicate", n)
		}
		for i, want := range args {
			h, e := m.GetArg(got, uint32(i))
			satOK(t, e)
			if h != want {
				t.Fatal("spill order", i)
			}
		}
	}
	for _, u := range []uint32{0, 1, 2, 3, 4, ^uint32(0)} {
		want := brRewriteFull
		if u < 3 {
			want = brStatus(u)
		}
		if unsignedToBRStatus(u) != want {
			t.Fatal("source rewrite depth conversion", u)
		}
	}
}

func TestBoolRewriterACIndependentAssignmentsThroughCDCL(t *testing.T) {
	for _, or := range []bool{false, true} {
		for row := 0; row < 32; row++ {
			m := astTestManager(t)
			rw := boolRewriter{manager: m}
			p := goalTestAtom(t, m, fmt.Sprint("table-p", row, or))
			q := goalTestAtom(t, m, fmt.Sprint("table-q", row, or))
			np := goalTestApp(t, m, ast.ASTOpNot, p)
			nq := goalTestApp(t, m, ast.ASTOpNot, q)
			truth, e := m.MkTrue()
			satOK(t, e)
			falsity, e := m.MkFalse()
			satOK(t, e)
			choices := []ast.ASTHandle{p, q, np, nq, truth, falsity}
			indices := []int{row % 6, (row / 6) % 6, (row*5 + 1) % 6}
			args := []ast.ASTHandle{choices[indices[0]], choices[indices[1]], choices[indices[2]]}
			op := int32(ast.ASTOpAnd)
			if or {
				op = ast.ASTOpOr
			}
			original := goalTestApp(t, m, op, args...)
			out := astTestPin(t, m, original)
			if or {
				_, e = rw.mkFlatOrCore(args, out)
			} else {
				_, e = rw.mkFlatAndCore(args, out)
			}
			satOK(t, e)
			rewritten, e := m.RefGet(out)
			satOK(t, e)
			for bits := 0; bits < 4; bits++ {
				vals := []bool{bits&1 != 0, bits&2 != 0, bits&1 == 0, bits&2 == 0, true, false}
				want := !or
				for _, idx := range indices {
					if or {
						want = want || vals[idx]
					} else {
						want = want && vals[idx]
					}
				}
				for _, formula := range []ast.ASTHandle{original, rewritten} {
					s, _, _ := satSolverCheckFixture(t)
					tr, e := TestNewGoalTranslator(m, s)
					satOK(t, e)
					forms := []ast.ASTHandle{formula}
					for i, h := range []ast.ASTHandle{p, q} {
						if bits&(1<<i) == 0 {
							h, e = m.MkNot(h)
							satOK(t, e)
						}
						forms = append(forms, h)
					}
					satOK(t, tr.Compile(forms))
					r, e := s.Check(nil)
					satOK(t, e)
					if (r == lbool.True) != want || r == lbool.Undef {
						t.Fatal("independent truth table", or, row, bits, r, want)
					}
					satOK(t, tr.Close())
					satOK(t, s.Close())
				}
			}
		}
	}
}
