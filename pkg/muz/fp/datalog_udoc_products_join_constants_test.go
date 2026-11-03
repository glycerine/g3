package fp

// Source: dl_util.h variable_intersection only joins variable columns;
// dl_compiler.cpp filters literal tail arguments after the product join.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func addPublicUDocPairConstantRule(f *publicUDocPairFixture, name string, left, right, head ast.ASTHandle, reverse bool) ast.ASTHandle {
	f.t.Helper()
	if reverse {
		left, right = right, left
	}
	return f.rule(name, f.pin(f.m.MkAnd(left, right)), head)
}

func checkPublicUDocPairConstantPlan(f *publicUDocPairFixture, head ast.ASTHandle, keys, filters int) {
	f.t.Helper()
	if f.e.udocDatalog == nil || f.e.udocDatalog.products == nil {
		f.t.Fatal("constant pair used ordinary finite fallback")
	}
	plan := f.e.udocDatalog.products.program.rules[head]
	if plan == nil || plan.pair == nil || len(plan.pair.leftColumns) != keys || len(plan.pair.rightColumns) != keys || len(plan.filters) != filters {
		f.t.Fatal("literal tail arguments changed variable join keys or lost filters", plan)
	}
}

func TestDatalogUDocProductsPairConstantsComponents(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			for _, tc := range []struct {
				name                           string
				boolean                        bool
				constant                       uint32
				firstKey, secondDoc, secondKey uint32
				status                         lbool.LBool
				rows                           [][]uint32
			}{
				{"bv-match", false, 1, 1, 2, 2, lbool.True, [][]uint32{{2, 1, 0}}},
				{"bv-doc-empty", false, 2, 2, 2, 2, lbool.False, nil},
				{"bv-table-empty", false, 1, 2, 2, 2, lbool.False, nil},
				{"bool-match", true, 1, 1, 1, 1, lbool.True, [][]uint32{{1, 0}}},
				{"bool-empty", true, 0, 1, 1, 1, lbool.False, nil},
				{"bool-coarse-false-formula", true, 1, 2, 1, 2, lbool.True, nil},
			} {
				t.Run(fmt.Sprintf("%s/reverse=%v/%s", kind, reverse, tc.name), func(t *testing.T) {
					f := newPublicUDocPairFixture(t, kind, reverse)
					var output, left, right, head ast.ASTHandle
					keys := 0
					if tc.boolean {
						output = f.declare("pair-constant-output", f.bits, f.boolean)
						value := f.no
						if tc.constant != 0 {
							value = f.yes
						}
						left, right = f.app(f.first, f.x, value), f.app(f.second, f.x, f.c)
						head, keys = f.app(output, f.x, f.c), 1
					} else {
						output = f.declare("pair-constant-output", f.bits, f.boolean, f.boolean)
						left, right = f.app(f.first, f.values[tc.constant], f.b), f.app(f.second, f.z, f.c)
						head = f.app(output, f.z, f.b, f.c)
					}
					addPublicUDocPairConstantRule(f, "pair-constant-rule", left, right, head, reverse)
					f.seed(1, tc.firstKey, tc.secondDoc, tc.secondKey)
					firstTable, secondTable := f.e.predicates[f.finite].table, f.e.predicates[f.secondTable].table
					for run := 0; run < 2; run++ {
						f.check(output, tc.status, tc.rows...)
						checkPublicUDocPairConstantPlan(f, output, keys, 1)
						if tc.status == lbool.True {
							f.checkProduct(kind, output)
						}
					}
					if f.e.predicates[f.finite].table != firstTable || f.e.predicates[f.secondTable].table != secondTable {
						t.Fatal("tail constant selection replaced source storage")
					}
					for _, source := range []ast.ASTHandle{f.finite, f.secondTable} {
						if count, err := f.e.predicates[source].table.Len(); err != nil || count != 1 {
							t.Fatal("tail constant selection mutated source facts", count, err)
						}
					}
				})
			}
		}
	}
}

func TestDatalogUDocProductsPairConstantsAreNotJoinKeys(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%v", kind, reverse), func(t *testing.T) {
				f := newPublicUDocPairFixture(t, kind, reverse)
				var outputs []ast.ASTHandle
				for _, same := range []bool{false, true} {
					name := fmt.Sprintf("pair-literal-only-%v", same)
					output := f.declare(name, f.boolean, f.bits)
					outputs = append(outputs, output)
					value := f.yes
					if same {
						value = f.no
					}
					left, right := f.app(f.pair, f.values[1], f.b, f.no), f.app(f.second, f.z, value)
					addPublicUDocPairConstantRule(f, name+"-rule", left, right, f.app(output, f.b, f.z), reverse)
				}
				f.seed(1, 1, 1, 1)
				fragmentMust(t, f.e.AddTableFact(f.secondTable, []uint32{1, 1}))
				for _, output := range outputs {
					f.check(output, lbool.True, []uint32{1, 1})
					checkPublicUDocPairConstantPlan(f, output, 0, 3)
					f.checkProduct(kind, output)
				}
			})
		}
	}
}

func TestDatalogUDocProductsPairConstantsQueryAndMutation(t *testing.T) {
	for _, kind := range []string{"hashtable", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f := newPublicUDocPairFixture(t, kind, true)
			output := f.declare("pair-constant-permuted", f.boolean, f.bits)
			bodyLeft, bodyRight := f.app(f.first, f.x, f.yes), f.app(f.second, f.x, f.c)
			formula := addPublicUDocPairConstantRule(f, "pair-constant-permuted-rule", bodyLeft, bodyRight, f.app(output, f.c, f.x), true)
			f.seed(1, 1, 1, 1)
			f.check(output, lbool.True, []uint32{0, 1})
			original := f.e.udocDatalog.products.result[output]
			for _, tc := range []struct {
				query    ast.ASTHandle
				bindings []ast.ASTHandle
				want     bool
			}{
				{f.app(output, f.c, f.x), []ast.ASTHandle{f.no, f.values[1]}, true},
				{f.app(output, f.no, f.values[1]), nil, true},
				{f.app(output, f.yes, f.values[1]), nil, false},
			} {
				status, err := f.e.QueryExpression(tc.query)
				fragmentMust(t, err)
				if status == lbool.Undef || (status == lbool.True) != tc.want || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil {
					t.Fatal("constant pair expression status or clone", status, tc.want)
				}
				clone := f.e.queryClone.udocDatalog.products
				if clone == nil || clone.result[output] == nil || clone.result[output] == original {
					t.Fatal("constant pair expression shared mutable producer result")
				}
				answer, err := f.e.ExpressionAnswer()
				fragmentMust(t, err)
				fragmentMust(t, f.e.clearQueryResources())
				f.checkFormula(answer, tc.bindings, tc.want)
				fragmentMust(t, f.m.DelRef(&answer))
			}
			fragmentMust(t, f.e.UpdateRule(formula, "pair-constant-permuted-rule"))
			f.check(output, lbool.True, []uint32{0, 1})
			fragmentMust(t, f.e.AddTableFact(f.secondTable, []uint32{1, 1}))
			f.check(output, lbool.True, []uint32{0, 1}, []uint32{1, 1})
			// A nonmatching Boolean fact leaves the true-only filter's answer
			// unchanged; the original source still owns both Boolean rows.
			fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{0, 1, 0}))
			f.check(output, lbool.True, []uint32{0, 1}, []uint32{1, 1})
			f.check(f.first, lbool.True, []uint32{1, 0}, []uint32{1, 1})
		})
	}
}
