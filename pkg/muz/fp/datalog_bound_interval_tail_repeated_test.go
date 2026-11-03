package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogRepeatedTailColumnsAndUpdates(t *testing.T) {
	for _, kinds := range [][]string{
		{"bound_relation", "interval_relation"},
		{"interval_relation", "bound_relation"},
	} {
		t.Run(kinds[0], func(t *testing.T) {
			f := newDirectFactFixture(t)
			m, e := f.manager, f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			var predicates []boundIntervalDatalogPredicate
			declare := func(name string, arity int) ast.ASTHandle {
				symbol, cause := m.Symbols.Intern("bound-interval-repeated-tail-" + name)
				must(cause)
				domain := make([]ast.ASTHandle, arity)
				for i := range domain {
					domain[i] = integer
				}
				declaration, cause := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				must(cause)
				must(e.RegisterPredicate(declaration))
				predicates = append(predicates, boundIntervalDatalogPredicate{declaration: declaration, kinds: kinds})
				return declaration
			}
			p, mixed, disjoint := declare("P", 2), declare("Mixed", 5), declare("Disjoint", 2)
			q, reordered := declare("Q", 1), declare("Reordered", 3)
			guarded, empty, wrongConstant := declare("Guarded", 4), declare("Empty", 1), declare("WrongConstant", 1)
			guardedProjected, guardedConstant := declare("GuardedProjected", 3), declare("GuardedConstant", 3)
			coarseProjected := declare("CoarseProjected", 2)
			activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, predicates)
			must(err)
			if !selected {
				t.Fatal("exact bound/interval product was not selected")
			}
			defer func() { must(activation.close()) }()
			numeral := func(value int64) ast.ASTHandle {
				handle, cause := m.ArithmeticIntegerNumeralText(big.NewInt(value).String())
				must(cause)
				return handle
			}
			addFact := func(declaration ast.ASTHandle, values ...int64) {
				t.Helper()
				arguments := make([]ast.ASTHandle, len(values))
				for i, value := range values {
					arguments[i] = numeral(value)
				}
				must(activation.addFact(declaration, arguments))
			}
			addFact(p, 1, 3)
			addFact(p, 4, 6)
			addFact(mixed, 1, 7, 3, 20, 2)
			addFact(mixed, 4, 7, 6, 30, 5)
			addFact(disjoint, 1, 10)
			addFact(disjoint, 2, 20)
			x, err := m.MkVar(0, integer)
			must(err)
			y, err := m.MkVar(1, integer)
			must(err)
			app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
				handle, cause := m.MkApp(declaration, arguments)
				must(cause)
				return handle
			}
			addRule := func(head, body ast.ASTHandle, name string) {
				formula, cause := m.MkImplies(body, head)
				must(cause)
				must(e.AddRule(formula, name))
			}
			addRule(app(q, x), app(p, x, x), "repeated tail intersection")
			mixedTail := app(mixed, x, numeral(7), x, y, x)
			addRule(app(reordered, y, x, x), mixedTail, "triple repeated tail with constant and reordered repeated head")
			condition, err := m.MkArithmeticLE(x, numeral(3))
			must(err)
			guardedBody, err := m.MkAnd(mixedTail, condition)
			must(err)
			addRule(app(guarded, y, x, x, x), guardedBody, "filter repeated tail before repeated head assembly")
			addRule(app(guardedProjected, y, x, x), guardedBody, "filter before dropping a repeated occurrence")
			addRule(app(coarseProjected, x, x), guardedBody, "source fused projection of an absent variable")
			constantBody, err := m.MkAnd(app(p, x, x), condition)
			must(err)
			addRule(app(guardedConstant, numeral(7), x, x), constantBody, "filter repeated tail before head constant join")
			addRule(app(empty, x), app(disjoint, x, x), "disjoint repeated tail")
			addRule(app(wrongConstant, x), app(mixed, x, numeral(99), x, y, x), "nonmatching constant before repeated tail")
			values := func(items ...int64) []*big.Rat {
				result := make([]*big.Rat, len(items))
				for i, value := range items {
					result[i] = big.NewRat(value, 1)
				}
				return result
			}
			contains := func(declaration ast.ASTHandle, items ...int64) bool {
				t.Helper()
				product := activation.evaluated[declaration]
				if product == nil {
					t.Fatal("missing evaluated product")
				}
				return product.ContainsFact(values(items...))
			}
			check := func(maxX, maxY int64) {
				t.Helper()
				status, cause := activation.query([]ast.ASTHandle{q, reordered, guarded})
				must(cause)
				if status != lbool.Undef {
					t.Fatal("nonempty approximate repeated-tail status", status)
				}
				if !contains(q, 3) || !contains(q, maxX) || contains(q, 2) || contains(q, maxX+1) {
					t.Fatal("repeated tail did not intersect its source column intervals")
				}
				if !contains(reordered, 20, 3, 3) || !contains(reordered, maxY, maxX, maxX) || contains(reordered, 20, 2, 2) {
					t.Fatal("constant/triple repetition/head reordering changed the intersection")
				}
				if !contains(guarded, maxY, 3, 3, 3) || contains(guarded, maxY, 4, 3, 3) || contains(guarded, maxY, 3, 4, 3) || contains(guarded, maxY, 3, 3, 4) {
					t.Fatal("condition filtering lost a repeated column's equivalence class")
				}
				if !contains(guardedProjected, maxY, 3, 3) || contains(guardedProjected, maxY, 3, 4) || contains(guardedProjected, maxY, 4, 3) {
					t.Fatal("dropping an unused occurrence happened before interpreted filtering")
				}
				if !contains(guardedConstant, 7, 3, 3) || contains(guardedConstant, 7, 3, 4) || contains(guardedConstant, 7, 4, 3) {
					t.Fatal("head constant assembly happened before repeated-column filtering")
				}
				if !contains(coarseProjected, 3, maxX) || contains(coarseProjected, 4, 3) {
					t.Fatal("fused projection did not preserve source clone coarsening and last-occurrence binding")
				}
				if !activation.evaluated[empty].Empty() || !activation.evaluated[wrongConstant].Empty() {
					t.Fatal("disjoint repetitions or nonmatching constant did not produce empty output")
				}
				if !activation.entries[p].seed.ContainsFact(values(1, 6)) || !activation.entries[mixed].seed.ContainsFact(values(1, 7, 6, 30, 5)) {
					t.Fatal("repeated-tail filtering mutated source hulls")
				}
				if got := activation.evaluated[reordered].Kinds(); len(got) != 2 || got[0] != kinds[0] || got[1] != kinds[1] {
					t.Fatal("repeated-tail lowering changed component order", got)
				}
			}
			check(4, 30)
			check(4, 30)
			addFact(p, 6, 6)
			addFact(mixed, 6, 7, 6, 40, 6)
			check(6, 40)

			// Two-tail repetitions need the source cross-tail occurrence
			// equality order, independently of this single-tail lowering.
			body, err := m.MkAnd(app(p, x, x), app(q, x))
			must(err)
			addRule(app(empty, x), body, "repeated two-tail boundary")
			if _, cause := activation.query([]ast.ASTHandle{empty}); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
				t.Fatal("unaudited repeated two-tail shape was accepted", cause)
			}
			if _, cause := activation.relationFormula(q); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
				t.Fatal("failed repeated two-tail query exposed stale output", cause)
			}
		})
	}
}
