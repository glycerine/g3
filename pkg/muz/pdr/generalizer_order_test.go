package pdr

// Source: context::init_core_generalizers and expand_node in
// src/muz/pdr/pdr_context.cpp, lines 1815-1862 and 2077-2087.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestContextMultiCorePrecedesConvexGeneralization(t *testing.T) {
	for _, closed := range []bool{true, false} {
		name := "interior"
		if closed {
			name = "closure"
		}
		t.Run(name, func(t *testing.T) {
			m, owner, original, originalX, closeBase := closureFixture(t)
			defer closeBase()
			defer func() { closureMust(t, owner.Close()) }()
			realSort, err := m.GetSort(originalX)
			closureMust(t, err)
			closureMust(t, original.Close())
			closureMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			owner.contexts.solverStart = len(owner.contexts.solvers)
			boolean, err := m.MkBoolSort()
			closureMust(t, err)
			name, err := m.Symbols.Intern("generalizer_order")
			closureMust(t, err)
			declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{realSort, boolean}, boolean, ast.ASTFuncDeclInfo{})
			closureMust(t, err)
			transformer, err := NewPredicateTransformer(owner, declaration, ReachableNoCache)
			closureMust(t, err)
			defer func() { closureMust(t, transformer.Close()) }()
			signature, err := transformer.Signature()
			closureMust(t, err)
			variables := make([]ast.ASTHandle, len(signature))
			for i, symbol := range signature {
				variable, err := owner.OldToNew(symbol, 0)
				closureMust(t, err)
				variables[i], err = m.MkApp(variable, nil)
				closureMust(t, err)
			}
			x, p := variables[0], variables[1]
			zero, err := m.ArithmeticRealNumeralText("0")
			closureMust(t, err)
			one, err := m.ArithmeticRealNumeralText("1")
			closureMust(t, err)
			negative, err := m.MkArithmeticLT(x, zero)
			closureMust(t, err)
			notP, err := m.MkNot(p)
			closureMust(t, err)
			initial, err := m.MkAnd(notP, negative)
			closureMust(t, err)
			truth, err := m.MkTrue()
			closureMust(t, err)
			closureMust(t, transformer.InitializeSystem(initial, truth))
			frame, err := m.MkArithmeticLE(x, zero)
			closureMust(t, err)
			_, err = transformer.AddProperty(frame, 0)
			closureMust(t, err)
			positive, err := m.MkArithmeticGE(x, one)
			closureMust(t, err)
			context := &Context{manager: m, owner: owner, useMultiCore: true, useInductive: true, useConvexClosure: closed, useConvexInterior: !closed}
			cores, err := context.generalizeBlockingCores(&ModelNode{transformer: transformer}, []ast.ASTHandle{positive, p}, true)
			closureMust(t, err)
			defer func() { closureMust(t, closeGeneralizedCores(m, cores)) }()
			if len(cores) == 0 {
				t.Fatal("generalization lost the blocked region")
			}
			// Source multicore first drops x>=1: p alone blocks the initial
			// state. Convex generalization must therefore receive p. Convex
			// first instead learns an arithmetic region and loses p=true,x=-1.
			for _, value := range []struct {
				p    bool
				x    string
				want lbool.LBool
			}{{true, "-1", lbool.True}, {false, "1", lbool.False}} {
				for _, core := range cores {
					solver, err := owner.contexts.Fresh()
					closureMust(t, err)
					for _, literal := range core.literals {
						closureMust(t, solver.Assert(literal))
					}
					booleanValue, err := m.MkBoolVal(value.p)
					closureMust(t, err)
					booleanEq, err := m.MkEq(p, booleanValue)
					closureMust(t, err)
					closureMust(t, solver.Assert(booleanEq))
					number, err := m.ArithmeticRealNumeralText(value.x)
					closureMust(t, err)
					numericEq, err := m.MkEq(x, number)
					closureMust(t, err)
					closureMust(t, solver.Assert(numericEq))
					status, err := solver.Check(nil)
					closureMust(t, err)
					closureMust(t, solver.Close())
					if status != value.want {
						t.Fatal("generalized region changed the source multicore result", value.p, value.x, status, value.want)
					}
				}
			}
		})
	}
}
