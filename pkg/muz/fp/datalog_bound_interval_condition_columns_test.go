package fp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogConditionOnlyColumnProjectionAndUpdates(t *testing.T) {
	for _, test := range []struct {
		name  string
		kinds []string
	}{
		{name: "bound-first", kinds: []string{"bound_relation", "interval_relation"}},
		{name: "interval-first", kinds: []string{"interval_relation", "bound_relation"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			m, engine := f.manager, f.engine
			family, err := m.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := m.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			boolean, err := m.MkBoolSort()
			fragmentMust(t, err)
			declare := func(name string, arity int) ast.ASTHandle {
				symbol, cause := m.Symbols.Intern(test.name + "-bound-interval-condition-column-" + name)
				fragmentMust(t, cause)
				domain := make([]ast.ASTHandle, arity)
				for i := range domain {
					domain[i] = integer
				}
				declaration, cause := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				fragmentMust(t, cause)
				fragmentMust(t, engine.RegisterPredicate(declaration))
				return declaration
			}
			p := declare("P", 1)
			ordered := declare("Ordered", 2)
			projected := declare("Projected", 1)
			numeral := func(text string) ast.ASTHandle {
				value, cause := m.ArithmeticIntegerNumeralText(text)
				fragmentMust(t, cause)
				return value
			}
			app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
				result, cause := m.MkApp(declaration, arguments)
				fragmentMust(t, cause)
				return result
			}
			x, err := m.MkVar(0, integer)
			fragmentMust(t, err)
			y, err := m.MkVar(1, integer)
			fragmentMust(t, err)
			less, err := m.MkArithmeticLT(y, x)
			fragmentMust(t, err)
			body, err := m.MkAnd(app(p, x), less)
			fragmentMust(t, err)
			for _, head := range []ast.ASTHandle{app(ordered, y, x), app(projected, x)} {
				rule, cause := m.MkImplies(body, head)
				fragmentMust(t, cause)
				fragmentMust(t, engine.addRuleEager(rule, "bound/interval condition-only full column"))
			}
			activation, selected, err := newBoundIntervalDatalogActivation(m, engine.set, []boundIntervalDatalogPredicate{
				{declaration: p, kinds: test.kinds},
				{declaration: ordered, kinds: test.kinds},
				{declaration: projected, kinds: test.kinds},
			})
			fragmentMust(t, err)
			if !selected {
				t.Fatal("condition-column bound/interval product was not selected")
			}
			defer func() { fragmentMust(t, activation.close()) }()
			five := numeral("5")
			fragmentMust(t, activation.addFact(p, []ast.ASTHandle{five}))
			values := func(items ...int64) []*big.Rat {
				result := make([]*big.Rat, len(items))
				for i, item := range items {
					result[i] = big.NewRat(item, 1)
				}
				return result
			}
			query := func() {
				t.Helper()
				status, cause := activation.query([]ast.ASTHandle{ordered, projected})
				fragmentMust(t, cause)
				if status != lbool.Undef {
					t.Fatal("nonempty condition-column product status", status)
				}
			}
			checkInitial := func() {
				t.Helper()
				query()
				if inside, outside := activation.evaluated[ordered].ContainsFact(values(4, 5)), activation.evaluated[ordered].ContainsFact(values(6, 5)); !inside || outside {
					t.Fatal("condition-only product column did not preserve strict order", inside, outside)
				}
				if !activation.evaluated[projected].ContainsFact(values(5)) {
					t.Fatal("projecting the condition-only product column lost the predicate-tail value")
				}
				if source := activation.evaluated[p]; source == nil || !source.ContainsFact(values(5)) || source.ContainsFact(values(5, 0)) {
					t.Fatal("condition-column lowering mutated its bound/interval source")
				}
			}
			checkInitial()
			checkInitial()

			fragmentMust(t, activation.addFact(p, []ast.ASTHandle{numeral("9")}))
			query()
			if !activation.evaluated[ordered].ContainsFact(values(8, 9)) || activation.evaluated[ordered].ContainsFact(values(10, 9)) {
				t.Fatal("late product update did not receive the condition-only full column")
			}
			if !activation.evaluated[projected].ContainsFact(values(9)) || !activation.evaluated[p].ContainsFact(values(9)) {
				t.Fatal("late product update was lost during condition-column projection")
			}
		})
	}
}
