package qe

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBoolPluginContextSolveChoosesUnitsBeforeVariableOrder(t *testing.T) {
	for _, negative := range []bool{false, true} {
		for _, reverseUnits := range []bool{false, true} {
			t.Run(fmt.Sprintf("negative=%v/reverse=%v", negative, reverseUnits), func(t *testing.T) {
				f := newFMFixture(t)
				m := f.r.manager
				a := f.constant("context-a", f.boolean)
				b := f.constant("context-b", f.boolean)
				c := f.constant("context-c", f.boolean)
				ctx, err := NewSolverContext(m, []ast.ASTHandle{a, b, c})
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, ctx.Close()) })
				p, err := NewBoolPlugin(ctx)
				fmMust(t, err)
				fmMust(t, ctx.AddPlugin(p))
				first, second := b, a
				removedIndex := uint32(1)
				if reverseUnits {
					first, second = a, b
					removedIndex = 0
				}
				removed, err := ctx.Contains(removedIndex)
				fmMust(t, err)
				moved, err := ctx.Contains(2)
				fmMust(t, err)
				unit := first
				if negative {
					unit = f.own(m.MkNot(first))
				}
				// Source flatten_and moves the bound conjunct to slot zero.
				// is_app skips it, then the first unit wins.
				bound := f.own(m.MkVar(0, f.boolean))
				formula := f.own(m.MkAndArgs([]ast.ASTHandle{unit, second, bound}))
				conjs, err := NewConjunctionEnum(m, formula)
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, conjs.Close()) })
				// Mixed atoms block both variables in the polarity stage;
				// source unit solving must take priority regardless.
				atoms, err := NewBoolAtomIndex(m, []ast.ASTHandle{a, b}, []ast.ASTHandle{a, b})
				fmMust(t, err)
				t.Cleanup(func() { fmMust(t, atoms.Close()) })
				solved, err := p.SolveContext(conjs, formula, atoms)
				fmMust(t, err)
				if !solved {
					t.Fatal("source unit was not eliminated")
				}
				variables, err := ctx.Variables()
				fmMust(t, err)
				if len(variables) != 2 || variables[removedIndex] != c {
					t.Fatal("context did not eliminate the first unit with swap-pop", variables)
				}
				if _, err := removed.Target(); err == nil {
					t.Fatal("eliminated variable's contains cache stayed open")
				}
				gotMoved, err := ctx.Contains(removedIndex)
				fmMust(t, err)
				if gotMoved != moved {
					t.Fatal("swap-pop did not move the source contains cache")
				}
				updated, present, err := ctx.CurrentFormula()
				fmMust(t, err)
				if !present {
					t.Fatal("context did not receive rewritten formula")
				}
				check, err := ast.NewContainsApp(m, first)
				fmMust(t, err)
				defer func() { fmMust(t, check.Close()) }()
				found, err := check.Check(updated)
				fmMust(t, err)
				if found {
					t.Fatal("unit variable remains in updated formula")
				}
				checkSecond, err := ast.NewContainsApp(m, second)
				fmMust(t, err)
				defer func() { fmMust(t, checkSecond.Close()) }()
				found, err = checkSecond.Check(updated)
				fmMust(t, err)
				if !found {
					t.Fatal("one solve eliminated more than one context variable")
				}
			})
		}
	}
}

func TestBoolPluginContextSolveScansBooleanVariablesAndPublishesOnce(t *testing.T) {
	for _, mode := range []string{"positive", "negative", "absent", "mixed", "nested-occurrence"} {
		t.Run(mode, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			n := f.constant("context-int", f.integer)
			a := f.constant("context-a", f.boolean)
			b := f.constant("context-b", f.boolean)
			ctx, err := NewSolverContext(m, []ast.ASTHandle{n, a, b})
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, ctx.Close()) })
			p, err := NewBoolPlugin(ctx)
			fmMust(t, err)
			fmMust(t, ctx.AddPlugin(p))
			positive, negative := []ast.ASTHandle{a, b}, []ast.ASTHandle{a}
			formula := f.own(m.MkOr(a, b))
			if mode == "negative" {
				positive, negative = []ast.ASTHandle{a}, []ast.ASTHandle{a, b}
			} else if mode == "absent" {
				formula = f.own(m.MkOr(a, f.own(m.MkNot(a))))
				positive, negative = []ast.ASTHandle{a}, []ast.ASTHandle{a}
			} else if mode == "mixed" {
				negative = []ast.ASTHandle{a, b}
			} else if mode == "nested-occurrence" {
				both := f.own(m.MkEq(a, b))
				positive = append(positive, both)
				formula = f.own(m.MkOr(formula, both))
			}
			conjs, err := NewConjunctionEnum(m, formula)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, conjs.Close()) })
			atoms, err := NewBoolAtomIndex(m, positive, negative)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, atoms.Close()) })
			solved, err := p.SolveContext(conjs, formula, atoms)
			fmMust(t, err)
			wantSolved := mode != "mixed" && mode != "nested-occurrence"
			if solved != wantSolved {
				t.Fatal("source polarity selection", solved, wantSolved)
			}
			variables, err := ctx.Variables()
			fmMust(t, err)
			wantCount := 3
			if solved {
				wantCount = 2
			}
			if len(variables) != wantCount || variables[0] != n || variables[1] != a {
				t.Fatal("source context selection", variables)
			}
			updated, present, err := ctx.CurrentFormula()
			fmMust(t, err)
			if present != solved {
				t.Fatal("declined solving changed context formula")
			}
			if !solved {
				return
			}
			truth := mode == "positive"
			expected := f.own(m.MkBoolVal(truth))
			if mode == "absent" {
				if updated != formula {
					t.Fatal("absent variable changed formula identity")
				}
			} else {
				right, err := m.GetArg(updated, 1)
				fmMust(t, err)
				if right != expected {
					t.Fatal("wrong source polarity cofactor")
				}
			}
			// Both remaining Boolean polarities are blocked; a second call
			// must decline, keeping the already published formula intact.
			next, err := NewConjunctionEnum(m, updated)
			fmMust(t, err)
			defer func() { fmMust(t, next.Close()) }()
			solved, err = p.SolveContext(next, updated, atoms)
			fmMust(t, err)
			if solved {
				t.Fatal("second solve should decline blocked remaining variables")
			}
			still, _, err := ctx.CurrentFormula()
			fmMust(t, err)
			if still != updated {
				t.Fatal("declined second solve changed prior result")
			}
		})
	}
}
