package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestModelImplicantArrayEqualityDecisionsTraceAndSelectOwnership(t *testing.T) {
	for _, name := range []string{"identical", "finite", "defaults", "row", "equivalent-stores", "untranslated"} {
		t.Run(name, func(t *testing.T) {
			m, md, implicant, output, tags := modelImplicantDiagnosticFixture(t)
			var pins []ast.ASTRef
			defer func() {
				for j := range pins {
					arrayTableMust(t, m.DelRef(&pins[j]))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				arrayTableMust(t, err)
				ref, err := m.NewRef(h)
				arrayTableMust(t, err)
				pins = append(pins, ref)
				return h
			}
			boolean := own(m.MkBoolSort())
			no, yes := own(m.MkFalse()), own(m.MkTrue())
			zero := own(m.ArithmeticIntegerNumeralText("0"))
			one := own(m.ArithmeticIntegerNumeralText("1"))
			integer, err := m.GetSort(zero)
			arrayTableMust(t, err)
			domain := integer
			if name == "finite" {
				domain = boolean
			}
			sort := own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{domain}), boolean))
			family, err := m.ArrayFamilyID()
			arrayTableMust(t, err)
			base := own(m.MkConstArray(sort, no))
			leftValue, rightValue := base, base
			state, line, body := implicantTrue, 0, ""
			switch name {
			case "finite":
				rightValue = own(m.MkConstArray(sort, yes))
				state, line, body = implicantX, 550, "equality is unknown: (= equality-a equality-b)\n"
			case "defaults":
				rightValue = own(m.MkConstArray(sort, yes))
				state, line, body = implicantFalse, 565, "defaults are different: (= equality-a equality-b) false true\n"
			case "row":
				leftValue = own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, zero, yes}))
				state, line = implicantFalse, 597
			case "untranslated":
				leftValue = own(m.MkFreshConst("untranslated-value", sort))
				state, line, body = implicantX, 558, "equality is unknown: (= equality-a equality-b)\n"
			case "equivalent-stores":
				leftValue = own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, zero, yes}))
				rightValue = own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{leftValue, one, no}))
			}
			constant := func(name string) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(name)
				arrayTableMust(t, err)
				return own(m.MkConst(symbol, sort))
			}
			left, right := constant("equality-a"), constant("equality-b")
			for _, entry := range []struct{ variable, value ast.ASTHandle }{{left, leftValue}, {right, rightValue}} {
				decl, err := m.GetDecl(entry.variable)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, entry.value))
			}
			equality := own(m.MkEq(left, right))
			if name == "row" {
				body = "Equality evaluation: (= equality-a equality-b)\n(select (store ((as const (Array Int Bool)) false) 0 true) 0) |-> true\n(select ((as const (Array Int Bool)) false) 0) |-> false\n"
			}
			before := m.Stats()
			for _, enabled := range []bool{false, true} {
				output.Reset()
				if enabled {
					tags.Enable("pdr")
				}
				arrayTableMust(t, implicant.setupModel(md))
				output.Reset()
				arrayTableMust(t, implicant.evalArrayEq(equality, left, right))
				if got := implicant.state(equality); got != state {
					t.Fatal("wrong source array equality decision", got, state)
				}
				arrayTableMust(t, implicant.reset())
				after := m.Stats()
				if after.LiveReferences != before.LiveReferences || after.LiveNodes != before.LiveNodes {
					t.Fatal("array equality leaked selects or values", after, before)
				}
				if !enabled || !tags.ShouldTrace("pdr") {
					if output.Len() != 0 {
						t.Fatal("disabled equality trace", output.String())
					}
					continue
				}
				header := func(n int) string {
					return fmt.Sprintf("-------- [pdr] eval_array_eq src/model/model_implicant.cpp:%d ---------\n", n)
				}
				if !strings.HasPrefix(output.String(), header(538)+"array equality: (= equality-a equality-b)\n") {
					t.Fatal("missing equality entry trace", output.String())
				}
				if line != 0 && !strings.Contains(output.String(), header(line)+body+"------------------------------------------------\n") {
					t.Fatalf("missing equality outcome trace\ngot %s\nwant %s", output.String(), body)
				}
			}
		})
	}
}
