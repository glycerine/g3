package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestModelDiagramRequiresConcreteCounterexample(t *testing.T) {
	m, _, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := tableTest{t, m, md}
	s := f.term(m.MkUninterpretedSort(astTestSymbol(t, m, "diagram-open"), nil))
	value := f.term(m.MkModelValue(0, s))
	astTestMust(t, md.RegisterValue(value))
	x := f.term(m.MkVar(0, s))
	y := f.term(m.MkVar(1, s))
	// With five binders the one named element and five anonymous
	// representatives exceed the enumeration limit. Only an anonymous
	// binding can falsify this identity in the open universe.
	q := f.forall(f.term(m.MkEq(x, y)), s, s, s, s, s)
	f.check(q, lbool.Undef)
	astTestMust(t, md.FreezeUniverse(s))
	f.check(q, lbool.False)
}

func TestModelDiagramLimitsReturnUnknown(t *testing.T) {
	m := astTestManager(t)
	d := tableDiagram{model: &tableModel{manager: m}, work: tableDiagramWork}
	if d.decision(0, []int{-1, -2}) != 0 || !d.stopped {
		t.Fatal("work limit allowed a new decision")
	}
	d = tableDiagram{model: &tableModel{manager: m}, nodes: make([]tableDecision, tableDiagramNodes)}
	if d.decision(0, []int{-1, -2}) != 0 || !d.stopped {
		t.Fatal("node limit allowed a new decision")
	}
}

// Check congruence of a unary table with one exceptional row and an identity
// default. Both methods evaluate the same compiled model. Two of the four
// binders affect the formula.
func BenchmarkModelTableCongruence(b *testing.B) {
	m := astTestManager(b)
	boolean, err := m.MkBoolSort()
	astTestMust(b, err)
	sort, err := m.MkUninterpretedSort(astTestSymbol(b, m, "diagram-elements"), nil)
	astTestMust(b, err)
	g := tableModel{
		manager: m,
		sorts:   []ast.ASTHandle{{}, boolean, boolean},
		terms: []tableTerm{
			{}, {op: tableVariable, index: 0, sort: sort}, {op: tableVariable, index: 2, sort: sort},
			{op: ast.ASTOpEq, args: []int{1, 2}}, {op: tableApplication, args: []int{1}},
			{op: tableApplication, args: []int{2}}, {op: ast.ASTOpEq, args: []int{4, 5}},
			{op: ast.ASTOpImplies, args: []int{3, 6}},
		},
		functions: []tableFunction{{fallback: 1, entries: []tableEntry{{keys: [tableMaxArity]int{3}, value: 4}}}},
	}
	for i := 0; i < 16; i++ {
		g.sorts = append(g.sorts, sort)
	}
	for _, width := range []int{6, 16} {
		domain := make([]int, width)
		for i := range domain {
			domain[i] = 3 + i
		}
		domains := [][]int{domain, domain, domain, domain}
		b.Run(fmt.Sprintf("Width%d/Diagram", width), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				status, _ := g.checkDiagram(7, domains)
				if status != lbool.False {
					b.Fatal(status)
				}
			}
		})
		b.Run(fmt.Sprintf("Width%d/Enumerate", width), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				g.work = 1 << 30
				for _, a := range domain {
					for _, v := range domain {
						for _, c := range domain {
							for _, d := range domain {
								bindings := [4]int{d, c, v, a}
								if g.evaluate(7, bindings[:], 0) != tableTrue || g.incomplete {
									b.Fatal("congruence failed")
								}
							}
						}
					}
				}
			}
		})
	}
}
