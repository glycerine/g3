package ast

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTCanonicalApplicationHashCollisions(t *testing.T) {
	m := astTestManager(t)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, b)
	astTestMust(t, err)
	astTestPin(t, m, variable)
	quantifier := func(weight int32) ASTHandle {
		t.Helper()
		q, err := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{b},
			Names: []symbol.Symbol{symbol.Numerical(0)}, Body: variable, Weight: weight})
		astTestMust(t, err)
		astTestPin(t, m, q)
		return q
	}
	q1, q2 := quantifier(1), quantifier(2)
	h1, err := m.GetHash(q1)
	astTestMust(t, err)
	h2, err := m.GetHash(q2)
	astTestMust(t, err)
	if q1 == q2 || h1 != h2 {
		t.Fatal("quantifier weight must distinguish two equal-hash operands")
	}
	for _, arity := range []int{1, 2, 3, 4, 16, 17} {
		t.Run(fmt.Sprint(arity), func(t *testing.T) {
			domain, args := make([]ASTHandle, arity), make([]ASTHandle, arity)
			for i := range args {
				domain[i], args[i] = b, q1
			}
			decl, err := m.MkFuncDecl(astTestSymbol(t, m, "colliding-app"), domain, b, ASTFuncDeclInfo{})
			astTestMust(t, err)
			astTestPin(t, m, decl)
			makeApp := func(q ASTHandle) ASTHandle {
				t.Helper()
				args[arity-1] = q
				app, err := m.MkApp(decl, args)
				astTestMust(t, err)
				return app
			}
			left := makeApp(q1)
			astTestMust(t, m.IncRef(left))
			right := makeApp(q2)
			astTestPin(t, m, right)
			if left == right || makeApp(q1) != left || makeApp(q2) != right {
				t.Fatal("distinct colliding applications lost their canonical identity")
			}
			astTestMust(t, m.DecRef(left))
			if makeApp(q2) != right {
				t.Fatal("collision lookup did not cross the erased entry")
			}
			size := m.table.capacity()
			for index := uint32(1); m.table.capacity() == size; index++ {
				v, err := m.MkVar(index+uint32(arity)*10000, b)
				astTestMust(t, err)
				astTestPin(t, m, v)
			}
			newLeft := makeApp(q1)
			astTestPin(t, m, newLeft)
			if newLeft == left || newLeft == right || makeApp(q2) != right || makeApp(q1) != newLeft {
				t.Fatal("rehash lost or aliased a colliding application")
			}
		})
	}
}

func TestASTCanonicalDuplicateConsumesPlacementGeneration(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(97, boolean)
	astTestMust(t, err)
	astTestPin(t, m, x)
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "placed-app"), []ASTHandle{boolean}, boolean, ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, decl)
	args := []ASTHandle{x}
	first, err := m.MkApp(decl, args)
	astTestMust(t, err)
	astTestPin(t, m, first)
	dup, err := m.MkApp(decl, args)
	astTestMust(t, err)
	if dup != first || m.freeNode == 0 {
		t.Fatal("duplicate must return its temporary placement")
	}
	slot := m.freeNode
	before := m.nodeAt(slot).generation
	dup, err = m.MkApp(decl, args)
	astTestMust(t, err)
	if dup != first || m.freeNode != slot || m.nodeAt(slot).generation != before+1 {
		t.Fatal("duplicate did not consume exactly one arena generation")
	}
	m.nodeAt(slot).generation = math.MaxUint32
	dup, err = m.MkApp(decl, args)
	astTestMust(t, err)
	if dup != first || m.freeNode == slot {
		t.Fatal("duplicate used an exhausted arena slot")
	}
	foreign := astTestManager(t)
	foreignSort, err := foreign.MkBoolSort()
	astTestMust(t, err)
	y, err := foreign.MkVar(97, foreignSort)
	astTestMust(t, err)
	astTestPin(t, foreign, y)
	if _, err := m.MkApp(decl, []ASTHandle{y}); !errors.Is(err, ErrASTHandle) {
		t.Fatal("foreign operand matched an existing application", err)
	}
}
