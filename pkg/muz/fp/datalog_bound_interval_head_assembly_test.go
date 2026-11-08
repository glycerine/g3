package fp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogHeadAssemblyAndUpdates(t *testing.T) {
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
				symbol, cause := m.Symbols.Intern(test.name + "-bound-interval-assembly-" + name)
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
			p := declare("P", 1)
			q := declare("Q", 4)
			activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, []boundIntervalDatalogPredicate{
				{declaration: p, kinds: test.kinds},
				{declaration: q, kinds: test.kinds},
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
			for _, value := range []string{"2", "4"} {
				must(activation.addFact(p, []ast.ASTHandle{numeral(value)}))
			}
			x, err := m.MkVar(0, integer)
			must(err)
			y, err := m.MkVar(1, integer)
			must(err)
			pTail, err := m.MkApp(p, []ast.ASTHandle{x})
			must(err)
			seven := numeral("7")
			qHead, err := m.MkApp(q, []ast.ASTHandle{seven, x, x, y})
			must(err)
			rule, err := m.MkImplies(pTail, qHead)
			must(err)
			must(e.addRuleEager(rule, "bound/interval constant repeated and head-only columns"))

			values := func(items ...int64) []*big.Rat {
				result := make([]*big.Rat, len(items))
				for i, item := range items {
					result[i] = big.NewRat(item, 1)
				}
				return result
			}
			query := func() *rel.IntervalBoundProduct {
				t.Helper()
				status, cause := activation.query([]ast.ASTHandle{q})
				must(cause)
				if status != lbool.Undef {
					t.Fatal("nonempty assembled product status", status)
				}
				return activation.evaluated[q]
			}
			result := query()
			if result == nil || !result.ContainsFact(values(7, 2, 4, -100)) {
				t.Fatal("assembled product lost the source's coarse repeated-column hull")
			}
			if result.ContainsFact(values(8, 2, 4, -100)) {
				t.Fatal("constant head column lost its interval singleton")
			}
			if !result.ContainsFact(values(7, 2, 2, 100)) {
				t.Fatal("head-only product column was not full")
			}
			if source := activation.evaluated[p]; source == nil || !source.ContainsFact(values(4)) {
				t.Fatal("head assembly mutated its source product")
			}

			result = query()
			if !result.ContainsFact(values(7, 4, 2, 0)) {
				t.Fatal("repeated query changed assembled product columns")
			}
			must(activation.addFact(p, []ast.ASTHandle{numeral("10")}))
			result = query()
			if !result.ContainsFact(values(7, 2, 10, 0)) {
				t.Fatal("late product update did not flow through head assembly")
			}
		})
	}
}
