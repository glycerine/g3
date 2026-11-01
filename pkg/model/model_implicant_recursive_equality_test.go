package model

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestModelImplicantRecursiveArrayEqualityContinuesAfterEqualRows(t *testing.T) {
	for _, name := range []string{"nested-default-true", "nested-default-false", "nested-row-true", "nested-row-late-false", "nested-finite-unknown", "unknown-default", "unknown-row"} {
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
			yes, no := own(m.MkTrue()), own(m.MkFalse())
			zero, one := own(m.ArithmeticIntegerNumeralText("0")), own(m.ArithmeticIntegerNumeralText("1"))
			integer, err := m.GetSort(zero)
			arrayTableMust(t, err)
			family, err := m.ArrayFamilyID()
			arrayTableMust(t, err)
			mkSort := func(domain, rangeSort ast.ASTHandle) ast.ASTHandle {
				return own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{domain}), rangeSort))
			}
			store := func(base, index, value ast.ASTHandle) ast.ASTHandle {
				return own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, index, value}))
			}
			constant := func(name string, sort ast.ASTHandle) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(name)
				arrayTableMust(t, err)
				return own(m.MkConst(symbol, sort))
			}
			innerSort := mkSort(integer, boolean)
			innerBase := own(m.MkConstArray(innerSort, no))
			p := store(store(innerBase, zero, yes), one, yes)
			q := store(store(innerBase, one, yes), zero, yes)
			outerSort := mkSort(integer, innerSort)
			base := own(m.MkConstArray(outerSort, innerBase))
			leftValue, rightValue := base, base
			expected, expectedCalls, unknownLine := implicantTrue, 2, ""
			switch name {
			case "nested-default-true":
				leftValue = own(m.MkConstArray(outerSort, p))
				rightValue = own(m.MkConstArray(outerSort, q))
			case "nested-default-false":
				leftValue = own(m.MkConstArray(outerSort, p))
				expected = implicantFalse
			case "nested-row-true":
				leftValue = store(store(base, one, p), zero, p)
				rightValue = store(store(base, one, q), zero, q)
				expectedCalls = 5
			case "nested-row-late-false":
				leftValue = store(store(base, one, p), zero, p)
				rightValue = store(store(base, one, innerBase), zero, q)
				expected, expectedCalls = implicantFalse, 3
			case "nested-finite-unknown":
				innerSort = mkSort(boolean, boolean)
				outerSort = mkSort(integer, innerSort)
				p = own(m.MkConstArray(innerSort, no))
				q = own(m.MkConstArray(innerSort, yes))
				leftValue = own(m.MkConstArray(outerSort, p))
				rightValue = own(m.MkConstArray(outerSort, q))
				expected, unknownLine = implicantX, ":550 ---------\nequality is unknown: "
			case "unknown-default", "unknown-row":
				outerSort = innerSort
				u, v := constant("unknown-u", boolean), constant("unknown-v", boolean)
				if name == "unknown-default" {
					leftValue = own(m.MkConstArray(outerSort, u))
					rightValue = own(m.MkConstArray(outerSort, v))
					unknownLine = ":574 ---------\nequality is unknown: "
				} else {
					leftValue = store(innerBase, zero, u)
					rightValue = store(innerBase, zero, v)
					unknownLine = ":609 ---------\nequality is unknown: "
				}
				expected, expectedCalls = implicantX, 1
			}
			left, right := constant("recursive-left", outerSort), constant("recursive-right", outerSort)
			for _, entry := range []struct{ h, value ast.ASTHandle }{{left, leftValue}, {right, rightValue}} {
				decl, err := m.GetDecl(entry.h)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, entry.value))
			}
			equality := own(m.MkEq(left, right))
			modelConstants, err := md.Core().NumConstants()
			arrayTableMust(t, err)
			before := m.Stats()
			tags.Enable("pdr")
			arrayTableMust(t, implicant.setupModel(md))
			arrayTableMust(t, implicant.evalArrayEq(equality, left, right))
			if got := implicant.state(equality); got != expected {
				t.Fatal("wrong recursive equality decision", got, expected)
			}
			arrayTableMust(t, implicant.reset())
			remainingConstants, err := md.Core().NumConstants()
			arrayTableMust(t, err)
			if remainingConstants != modelConstants {
				t.Fatal("array equality completed the model", remainingConstants, modelConstants)
			}
			after := m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
				t.Fatal("recursive equality leaked AST ownership", after, before)
			}
			if tags.ShouldTrace("pdr") {
				calls := strings.Count(output.String(), "[pdr] eval_array_eq src/model/model_implicant.cpp:538")
				if calls != expectedCalls {
					t.Fatal("wrong source recursion/continuation count", calls, expectedCalls, output.String())
				}
				if unknownLine != "" && !strings.Contains(output.String(), "[pdr] eval_array_eq src/model/model_implicant.cpp"+unknownLine) {
					t.Fatal("missing recursive unknown diagnostic", output.String())
				}
			}
		})
	}
}
