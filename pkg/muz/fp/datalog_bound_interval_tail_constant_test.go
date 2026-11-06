package fp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogPositiveTailConstantsAndUpdates(t *testing.T) {
	for _, test := range []struct {
		name  string
		kinds []string
	}{
		{name: "bound-first", kinds: []string{"bound_relation", "interval_relation"}},
		{name: "interval-first", kinds: []string{"interval_relation", "bound_relation"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			m, e := f.manager, f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			declare := func(name string, arity int) ast.ASTHandle {
				symbol, cause := m.Symbols.Intern(test.name + "-bound-interval-tail-constant-" + name)
				must(cause)
				domain := make([]ast.ASTHandle, arity)
				for i := range domain {
					domain[i] = integer
				}
				declaration, cause := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				must(cause)
				must(e.RegisterPredicate(declaration))
				return declaration
			}
			p := declare("P", 3)
			q := declare("Q", 1)
			empty := declare("Empty", 1)
			s := declare("S", 1)
			joined := declare("Joined", 1)
			activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, []boundIntervalDatalogPredicate{
				{declaration: p, kinds: test.kinds},
				{declaration: q, kinds: test.kinds},
				{declaration: empty, kinds: test.kinds},
				{declaration: s, kinds: test.kinds},
				{declaration: joined, kinds: test.kinds},
			})
			must(err)
			if !selected {
				t.Fatal("exact bound/interval product was not selected")
			}
			defer func() { must(activation.close()) }()
			numeral := func(text string) ast.ASTHandle {
				value, cause := m.ArithmeticIntegerNumeralText(text)
				must(cause)
				return value
			}
			values := func(items ...int64) []*big.Rat {
				result := make([]*big.Rat, len(items))
				for i, item := range items {
					result[i] = big.NewRat(item, 1)
				}
				return result
			}
			must(activation.addFact(p, []ast.ASTHandle{numeral("2"), numeral("3"), numeral("7")}))
			must(activation.addFact(p, []ast.ASTHandle{numeral("4"), numeral("9"), numeral("8")}))
			must(activation.addFact(s, []ast.ASTHandle{numeral("3")}))
			x, err := m.MkVar(0, integer)
			must(err)
			app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
				value, cause := m.MkApp(declaration, arguments)
				must(cause)
				return value
			}
			addRule := func(head, tail ast.ASTHandle, name string) {
				formula, cause := m.MkImplies(tail, head)
				must(cause)
				must(e.AddRule(formula, name))
			}
			addRule(app(q, x), app(p, numeral("2"), x, numeral("7")), "matching bound/interval tail constants")
			addRule(app(empty, x), app(p, numeral("10"), x, numeral("7")), "nonmatching bound/interval tail constant")
			joinedBody, err := m.MkAnd(app(s, x), app(p, numeral("2"), x, numeral("7")))
			must(err)
			addRule(app(joined, x), joinedBody, "bound/interval two-tail constant join")

			status, err := activation.query([]ast.ASTHandle{empty})
			must(err)
			// C++ projection drops the empty flag after filtering out the
			// nonmatching constant column.
			if status != lbool.Undef || activation.evaluated[empty] == nil || activation.evaluated[empty].Empty() {
				t.Fatal("empty-source constant projection did not match C++ behavior", status)
			}
			query := func() *rel.IntervalBoundProduct {
				t.Helper()
				status, cause := activation.query([]ast.ASTHandle{q})
				must(cause)
				if status != lbool.Undef {
					t.Fatal("matching tail constant result status", status)
				}
				return activation.evaluated[q]
			}
			result := query()
			if result == nil || !result.ContainsFact(values(3)) || !result.ContainsFact(values(9)) {
				t.Fatal("tail constants lost the source interval's coarse cross-column hull")
			}
			if got := result.Kinds(); len(got) != 2 || got[0] != test.kinds[0] || got[1] != test.kinds[1] {
				t.Fatal("tail constant evaluation changed component order", got)
			}
			status, err = activation.query([]ast.ASTHandle{joined})
			must(err)
			if joinedResult := activation.evaluated[joined]; status != lbool.Undef || joinedResult == nil || !joinedResult.ContainsFact(values(3)) || joinedResult.ContainsFact(values(4)) {
				t.Fatal("two-tail constant join did not preserve its shared variable", status)
			}
			if source := activation.entries[p].seed; source == nil || !source.ContainsFact(values(4, 9, 8)) {
				t.Fatal("tail constant evaluation mutated its source product")
			}

			result = query()
			if !result.ContainsFact(values(9)) {
				t.Fatal("repeated query changed the tail-constant result")
			}
			must(activation.addFact(p, []ast.ASTHandle{numeral("2"), numeral("12"), numeral("7")}))
			must(activation.addFact(s, []ast.ASTHandle{numeral("12")}))
			result = query()
			if !result.ContainsFact(values(12)) || !activation.entries[p].seed.ContainsFact(values(4, 9, 8)) {
				t.Fatal("late update did not flow through tail constants without changing the source")
			}
			status, err = activation.query([]ast.ASTHandle{joined})
			must(err)
			if status != lbool.Undef || !activation.evaluated[joined].ContainsFact(values(12)) {
				t.Fatal("late update did not flow through the two-tail constant join", status)
			}
		})
	}
}
