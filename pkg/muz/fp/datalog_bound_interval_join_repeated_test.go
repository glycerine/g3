package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogTwoTailOccurrenceProjection(t *testing.T) {
	for _, kinds := range [][]string{
		{"bound_relation", "interval_relation"},
		{"interval_relation", "bound_relation"},
	} {
		for _, test := range []struct {
			name       string
			leftArgs   []int
			rightArgs  []int
			headArgs   []int
			leftFacts  [][]int64
			rightFacts [][]int64
			inside     [][]int64
			outside    [][]int64
			guard      int64
			selfJoin   bool
		}{
			{
				name: "all-shared-occurrences", leftArgs: []int{0, 0}, rightArgs: []int{0, 0}, headArgs: []int{0, 0, 0, 0},
				leftFacts: [][]int64{{1, 3}, {8, 6}}, rightFacts: [][]int64{{4, 5}, {7, 9}},
				inside: [][]int64{{5, 5, 5, 5}}, outside: [][]int64{{6, 5, 5, 5}, {5, 6, 5, 5}, {5, 5, 6, 5}, {5, 5, 5, 6}}, guard: 5,
			},
			{
				name: "early-local-projection-is-coarse", leftArgs: []int{0, 0}, rightArgs: []int{1}, headArgs: []int{0},
				leftFacts: [][]int64{{1, 10}, {2, 20}}, rightFacts: [][]int64{{5}, {6}},
				inside: [][]int64{{10}, {20}}, outside: [][]int64{{2}, {9}, {21}},
			},
			{
				name: "head-multiplicity-keeps-local-equality", leftArgs: []int{0, 0}, rightArgs: []int{1}, headArgs: []int{0, 0},
				leftFacts: [][]int64{{1, 10}, {2, 20}}, rightFacts: [][]int64{{5}, {6}},
				outside: [][]int64{{1, 10}},
			},
			{
				name: "right-local-repetition-and-source-rename", leftArgs: []int{1}, rightArgs: []int{0, 0}, headArgs: []int{1, 0, 0},
				leftFacts: [][]int64{{20}, {30}}, rightFacts: [][]int64{{1, 3}, {8, 6}},
				// The source rename copies raw vector elements before merging
				// classes, restoring the first repeated column's wider hull.
				inside: [][]int64{{20, 1, 1}, {30, 8, 8}}, outside: [][]int64{{20, 0, 1}, {30, 8, 9}},
			},
			{
				name: "shared-constants-and-guard", leftArgs: []int{0, -1, 0}, rightArgs: []int{0}, headArgs: []int{0, 0},
				leftFacts: [][]int64{{1, 7, 3}, {8, 7, 6}}, rightFacts: [][]int64{{4}, {7}},
				inside: [][]int64{{4, 4}, {5, 5}}, outside: [][]int64{{3, 4}, {6, 5}, {5, 6}}, guard: 5,
			},
			{
				name: "last-local-occurrence-binds-guard", leftArgs: []int{0, 0}, rightArgs: []int{1}, headArgs: []int{0},
				leftFacts: [][]int64{{1, 10}, {2, 20}}, rightFacts: [][]int64{{5}, {6}},
				inside: [][]int64{{10}, {15}}, outside: [][]int64{{2}, {9}, {16}}, guard: 15,
			},
			{
				name: "self-join-repeated-head", leftArgs: []int{0, 0}, rightArgs: []int{0, 0}, headArgs: []int{0, 0, 0},
				leftFacts: [][]int64{{1, 3}, {8, 6}}, selfJoin: true,
				inside: [][]int64{{3, 3, 3}, {6, 6, 6}}, outside: [][]int64{{2, 3, 3}, {3, 7, 3}},
			},
		} {
			t.Run(kinds[0]+"/"+test.name, func(t *testing.T) {
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
				declare := func(name string, arity int, order []string) ast.ASTHandle {
					symbol, cause := m.Symbols.Intern("bound-interval-two-tail-occurrences-" + name)
					must(cause)
					sorts := make([]ast.ASTHandle, arity)
					for i := range sorts {
						sorts[i] = integer
					}
					declaration, cause := m.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
					must(cause)
					must(e.RegisterPredicate(declaration))
					predicates = append(predicates, boundIntervalDatalogPredicate{declaration: declaration, kinds: order})
					return declaration
				}
				p := declare("P", len(test.leftArgs), kinds)
				q := p
				if !test.selfJoin {
					q = declare("Q", len(test.rightArgs), []string{kinds[1], kinds[0]})
				}
				h := declare("Head", len(test.headArgs), kinds)
				activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, predicates)
				must(err)
				if !selected {
					t.Fatal("bound/interval product was not selected")
				}
				defer func() { must(activation.close()) }()
				numeral := func(value int64) ast.ASTHandle {
					result, cause := m.ArithmeticIntegerNumeralText(big.NewInt(value).String())
					must(cause)
					return result
				}
				addFacts := func(declaration ast.ASTHandle, facts [][]int64) {
					for _, fact := range facts {
						arguments := make([]ast.ASTHandle, len(fact))
						for i, value := range fact {
							arguments[i] = numeral(value)
						}
						must(activation.addFact(declaration, arguments))
					}
				}
				addFacts(p, test.leftFacts)
				addFacts(q, test.rightFacts)
				application := func(declaration ast.ASTHandle, indexes []int) ast.ASTHandle {
					arguments := make([]ast.ASTHandle, len(indexes))
					for i, index := range indexes {
						if index < 0 {
							arguments[i] = numeral(7)
						} else {
							arguments[i], err = m.MkVar(uint32(index), integer)
							must(err)
						}
					}
					result, cause := m.MkApp(declaration, arguments)
					must(cause)
					return result
				}
				terms := []ast.ASTHandle{application(p, test.leftArgs), application(q, test.rightArgs)}
				if test.guard != 0 {
					x, cause := m.MkVar(0, integer)
					must(cause)
					condition, cause := m.MkArithmeticLE(x, numeral(test.guard))
					must(cause)
					terms = append(terms, condition)
				}
				body, err := m.MkAndArray(terms)
				must(err)
				formula, err := m.MkImplies(body, application(h, test.headArgs))
				must(err)
				must(e.AddRule(formula, test.name))
				contains := func(declaration ast.ASTHandle, values []int64) bool {
					arguments := make([]*big.Rat, len(values))
					for i, value := range values {
						arguments[i] = big.NewRat(value, 1)
					}
					return activation.evaluated[declaration].ContainsFact(arguments)
				}
				check := func() {
					t.Helper()
					status, cause := activation.query([]ast.ASTHandle{h})
					must(cause)
					want := lbool.Undef
					if len(test.inside) == 0 {
						want = lbool.False
					}
					if status != want {
						t.Fatal("two-tail status", status, want)
					}
					for _, values := range test.inside {
						if !contains(h, values) {
							t.Fatal("missing source-supported tuple", values)
						}
					}
					for _, values := range test.outside {
						if contains(h, values) {
							t.Fatal("tuple outside source interval hull", values)
						}
					}
					for _, values := range test.leftFacts {
						if !contains(p, values) {
							t.Fatal("join mutated source", values)
						}
					}
				}
				check()
				answer, err := activation.relationFormula(h)
				must(err)
				defer func() { must(m.DelRef(&answer)) }()
				m.Limit.Cancel()
				_, cause := activation.query([]ast.ASTHandle{h})
				m.Limit.ResetCancel()
				if cause == nil {
					t.Fatal("canceled join query succeeded")
				}
				if _, cause = activation.relationFormula(h); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
					t.Fatal("canceled query published stale answer", cause)
				}
				if _, cause = m.RefGet(answer); cause != nil {
					t.Fatal("canceled query invalidated caller-owned formula", cause)
				}
				check()
			})
		}
	}
}
