package model

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestModelImplicantConeCollectionAndModelOrder(t *testing.T) {
	for _, disjunction := range []bool{false, true} {
		name := "duplicate-roots"
		if disjunction {
			name = "first-true-disjunct"
		}
		t.Run(name, func(t *testing.T) {
			m, md, implicant, output, tags := modelImplicantDiagnosticFixture(t)
			var pins []ast.ASTRef
			defer func() {
				for n := range pins {
					arrayTableMust(t, m.DelRef(&pins[n]))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				arrayTableMust(t, err)
				r, err := m.NewRef(h)
				arrayTableMust(t, err)
				pins = append(pins, r)
				return h
			}
			boolean := own(m.MkBoolSort())
			yes := own(m.MkTrue())
			variables := make([]ast.ASTHandle, 3)
			equalities := make([]ast.ASTHandle, 3)
			for n, label := range []string{"cone-p", "cone-q", "cone-unrelated"} {
				symbol, err := m.Symbols.Intern(label)
				arrayTableMust(t, err)
				variables[n] = own(m.MkConst(symbol, boolean))
				decl, err := m.GetDecl(variables[n])
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, yes))
				equalities[n] = own(m.MkEq(variables[n], yes))
			}
			formulas := []ast.ASTHandle{variables[0], variables[1], variables[1]}
			expected := equalities[:2]
			if disjunction {
				formulas = []ast.ASTHandle{own(m.MkOrArray(variables[:2]))}
				expected = equalities[:1]
			}
			tags.Enable("pdr")
			tags.Enable("pdr_verbose")
			before := m.Stats()
			result, err := implicant.MinimizeModel(formulas, md)
			arrayTableMust(t, err)
			if len(result) != len(expected) {
				t.Fatal("incorrect selected cone size", len(result), len(expected))
			}
			for n, ref := range result {
				got, err := m.RefGet(ref)
				arrayTableMust(t, err)
				if got != expected[n] {
					t.Fatal("cone output did not preserve model registration order", n, got, expected[n])
				}
			}
			if tags.ShouldTrace("pdr_verbose") {
				text := output.String()
				p := strings.Index(text, "collect: cone-p\n")
				q := strings.Index(text, "collect: cone-q\n")
				if p < 0 || strings.Count(text, "collect: cone-p\n") != 1 {
					t.Fatal("missing or duplicated first literal", text)
				}
				if disjunction {
					if q >= 0 {
						t.Fatal("collector chose an extra true disjunct", text)
					}
				} else if q < 0 || q >= p || strings.Count(text, "collect: cone-q\n") != 1 {
					t.Fatal("collector lost source LIFO order or duplicate suppression", text)
				}
			}
			if tags.ShouldTrace("pdr") {
				count := "3 ==> 2\n"
				if disjunction {
					count = "3 ==> 1\n"
				}
				text := output.String()
				if strings.Count(text, count) != 1 {
					t.Fatal("wrong source reduction count", text)
				}
				collectAt := strings.LastIndex(text, "collect: ")
				summaryAt := strings.Index(text, count)
				resultAt := strings.Index(text, "pruned model:\n")
				if tags.ShouldTrace("pdr_verbose") && !(collectAt < summaryAt && summaryAt < resultAt) {
					t.Fatal("cone diagnostics in wrong source order", text)
				}
			}
			arrayTableMust(t, CloseImplicantResults(m, &result))
			after := m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
				t.Fatal("cone pruning leaked references", after, before)
			}
		})
	}
}
