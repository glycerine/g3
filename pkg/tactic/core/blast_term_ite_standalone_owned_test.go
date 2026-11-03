package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp:65-88,189-197.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBlastTermITEIntoKeepsHoistedResultOwned(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprintf("proofs=%t", proofs), func(t *testing.T) {
			f := newBlastTermITEFixture(t, proofs)
			choice, err := f.m.MkIte(f.cond, f.one, f.two)
			simplifyMust(t, err)
			application, err := f.m.MkApp(f.f, []ast.ASTHandle{choice})
			simplifyMust(t, err)
			formula, err := f.m.MkEq(application, f.one)
			simplifyMust(t, err)
			simplifyPin(t, f.m, formula)
			inputDeclarations := make(map[ast.ASTHandle]bool)
			visit := func(root ast.ASTHandle, declaration func(ast.ASTHandle)) {
				t.Helper()
				seen := make(map[ast.ASTHandle]bool)
				todo := []ast.ASTHandle{root}
				for len(todo) != 0 {
					node := todo[len(todo)-1]
					todo = todo[:len(todo)-1]
					if seen[node] {
						continue
					}
					seen[node] = true
					kind, cause := f.m.GetKind(node)
					simplifyMust(t, cause)
					if kind != ast.ASTApp {
						t.Fatal("unexpected non-application in ground expression", kind)
					}
					decl, cause := f.m.GetDecl(node)
					simplifyMust(t, cause)
					family, cause := f.m.GetFamilyID(node)
					simplifyMust(t, cause)
					if family == ast.NullFamilyID {
						declaration(decl)
					}
					count, cause := f.m.GetNumArgs(node)
					simplifyMust(t, cause)
					for index := uint32(0); index < count; index++ {
						argument, cause := f.m.GetArg(node, index)
						simplifyMust(t, cause)
						todo = append(todo, argument)
					}
				}
			}
			visit(formula, func(declaration ast.ASTHandle) { inputDeclarations[declaration] = true })
			mode, err := f.m.ProofMode()
			simplifyMust(t, err)
			before := f.m.Stats().LiveReferences
			out, err := f.m.NewRef(formula)
			simplifyMust(t, err)
			defer func() {
				if out != (ast.ASTRef{}) {
					simplifyMust(t, f.m.DelRef(&out))
				}
			}()
			simplifyMust(t, BlastTermITEInto(f.m, formula, out))
			got, err := f.m.RefGet(out)
			simplifyMust(t, err)
			// Dereference every node after traversal destruction, before creating
			// the expected graph, so another owner cannot hide a stale result.
			visit(got, func(declaration ast.ASTHandle) {
				if !inputDeclarations[declaration] {
					t.Fatal("hoisting introduced a fresh uninterpreted declaration", declaration)
				}
			})
			if got == formula {
				t.Fatal("term choice was not hoisted")
			}
			left, err := f.m.MkApp(f.f, []ast.ASTHandle{f.one})
			simplifyMust(t, err)
			right, err := f.m.MkApp(f.f, []ast.ASTHandle{f.two})
			simplifyMust(t, err)
			leftEquality, err := f.m.MkEq(left, f.one)
			simplifyMust(t, err)
			rightEquality, err := f.m.MkEq(right, f.one)
			simplifyMust(t, err)
			want, err := f.m.MkIte(f.cond, leftEquality, rightEquality)
			simplifyMust(t, err)
			if got != want {
				t.Fatal("hoisted expression differs from source branches", got, want)
			}
			afterMode, err := f.m.ProofMode()
			simplifyMust(t, err)
			if afterMode != mode || f.m.Stats().LiveReferences != before+1 {
				t.Fatal("standalone traversal changed proof mode or left references", mode, afterMode, before, f.m.Stats().LiveReferences)
			}
			simplifyMust(t, f.m.DelRef(&out))
			if f.m.Stats().LiveReferences != before {
				t.Fatal("caller output reference was not released")
			}
			if _, err = f.m.GetKind(got); err == nil {
				t.Fatal("unowned hoisted result survived caller output release")
			}
		})
	}
}
