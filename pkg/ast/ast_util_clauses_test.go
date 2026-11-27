package ast

// Source: src/ast/ast_util.cpp:94-158; src/ast/ast_util.h:78-93.

import (
	"errors"
	"fmt"
	"testing"
)

func TestASTUtilAtomsLiteralsAndClauses(t *testing.T) {
	m := astTestManager(t)
	boolSort, e := m.MkBoolSort()
	astTestMust(t, e)
	otherSort, e := m.MkSort(astTestSymbol(t, m, "U"), ASTSortInfo{})
	astTestMust(t, e)
	b, e := m.MkVar(0, boolSort)
	astTestMust(t, e)
	x, e := m.MkVar(0, otherSort)
	astTestMust(t, e)
	y, e := m.MkVar(1, otherSort)
	astTestMust(t, e)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	constant, e := m.MkConst(astTestSymbol(t, m, "p"), boolSort)
	astTestMust(t, e)
	eq, e := m.MkEq(x, y)
	astTestMust(t, e)
	conjunction, e := m.MkAnd(b, constant)
	astTestMust(t, e)
	notB, e := m.MkNot(b)
	astTestMust(t, e)

	for _, tc := range []struct {
		h    ASTHandle
		atom bool
		lit  bool
	}{{b, true, true}, {truth, true, true}, {constant, true, true}, {eq, true, true}, {x, false, false}, {conjunction, false, false}, {notB, false, true}} {
		atom, err := m.IsAtom(tc.h)
		astTestMust(t, err)
		literal, err := m.IsLiteral(tc.h)
		astTestMust(t, err)
		if atom != tc.atom || literal != tc.lit {
			t.Fatalf("%v: atom=%v literal=%v, want %v/%v", tc.h, atom, literal, tc.atom, tc.lit)
		}
	}
	atom, sign, e := m.LiteralAtomSign(notB)
	astTestMust(t, e)
	if atom != b || !sign {
		t.Fatal("negative literal", atom, sign)
	}
	atom, sign, e = m.LiteralAtomSign(eq)
	astTestMust(t, e)
	if atom != eq || sign {
		t.Fatal("positive literal", atom, sign)
	}
	if _, _, e = m.LiteralAtomSign(conjunction); !errors.Is(e, ErrASTDomain) {
		t.Fatal("nonliteral accepted", e)
	}

	clause, e := m.MkOrArgs([]ASTHandle{b, notB, eq})
	astTestMust(t, e)
	badClause, e := m.MkOr(b, conjunction)
	astTestMust(t, e)
	for _, tc := range []struct {
		h    ASTHandle
		want bool
	}{{b, true}, {clause, true}, {conjunction, false}, {badClause, false}} {
		got, err := m.IsClause(tc.h)
		astTestMust(t, err)
		if got != tc.want {
			t.Fatalf("%v: clause=%v, want %v", tc.h, got, tc.want)
		}
	}
	for _, tc := range []struct {
		root ASTHandle
		want []ASTHandle
	}{
		{b, []ASTHandle{b}}, {notB, []ASTHandle{notB}}, {clause, []ASTHandle{b, notB, eq}}, {badClause, nil},
	} {
		view, ok, err := m.ClauseLiteralView(tc.root)
		astTestMust(t, err)
		if ok != (tc.want != nil) || int(view.Count()) != len(tc.want) {
			t.Fatal("clause view", ok, view.Count(), tc.want)
		}
		for i, want := range tc.want {
			got, err := view.At(uint32(i))
			if err != nil || got != want {
				t.Fatal("clause view order", i, got, err)
			}
		}
		if _, err := view.At(view.Count()); !errors.Is(err, ErrASTDomain) {
			t.Fatal("clause view bounds", err)
		}
	}
	n, e := m.ClauseNumLiterals(clause)
	astTestMust(t, e)
	if n != 3 {
		t.Fatal("clause size", n)
	}
	for i, want := range []ASTHandle{b, notB, eq} {
		got, err := m.ClauseLiteral(clause, uint32(i))
		astTestMust(t, err)
		if got != want {
			t.Fatal("clause literal", i, got, want)
		}
	}
	if n, e = m.ClauseNumLiterals(b); e != nil || n != 1 {
		t.Fatal("singleton clause", n, e)
	}
	if _, e = m.ClauseLiteral(clause, 3); !errors.Is(e, ErrASTDomain) {
		t.Fatal("clause bounds", e)
	}
	if _, e = m.ClauseNumLiterals(badClause); !errors.Is(e, ErrASTDomain) {
		t.Fatal("nonclause accepted", e)
	}
}

func BenchmarkASTClauseLiteralScan(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	for _, n := range []int{1, 8, 64, 512} {
		args := make([]ASTHandle, n)
		for i := range args {
			args[i], err = m.MkVar(uint32(i), sort)
			astTestMust(b, err)
		}
		cls, err := m.MkOrArgs(args)
		astTestMust(b, err)
		astTestPin(b, m, cls)
		for _, view := range []bool{false, true} {
			name := "Repeated"
			if view {
				name = "View"
			}
			b.Run(fmt.Sprintf("%d/%s", n, name), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if view {
						lits, ok, err := m.ClauseLiteralView(cls)
						if err != nil || !ok || int(lits.Count()) != n {
							b.Fatal(lits.Count(), ok, err)
						}
						for i, want := range args {
							v, err := lits.At(uint32(i))
							if err != nil || v != want {
								b.Fatal(i, v, err)
							}
						}
					} else {
						count, err := m.ClauseNumLiterals(cls)
						if err != nil || int(count) != n {
							b.Fatal(count, err)
						}
						for i, want := range args {
							v, err := m.ClauseLiteral(cls, uint32(i))
							if err != nil || v != want {
								b.Fatal(i, v, err)
							}
						}
					}
				}
			})
		}
	}
}
