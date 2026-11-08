package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source rel_context::transform_rules separates each private negative-tail
// variable before finite compilation, giving its existential complement its
// own lower-stratum relation and preventing it from widening the outer join.
func TestDatalogPrivateNegativeTailHasQueryLocalComplement(t *testing.T) {
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("negative-tail query leaked bytes", budget.Used())
		}
	})
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	budget = m.Budget()
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(name)
		must(err)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = boolean
		}
		declaration, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	seed := declare("negative-private-seed", 1)
	blocked := declare("negative-private-blocked", 2)
	output := declare("negative-private-output", 1)
	must(e.AddTableFact(seed, []uint32{0}))
	must(e.AddTableFact(blocked, []uint32{0, 0}))
	x, err := m.MkVar(1, boolean)
	must(err)
	y, err := m.MkVar(0, boolean)
	must(err)
	positive, err := m.MkApp(seed, []ast.ASTHandle{x})
	must(err)
	negative, err := m.MkApp(blocked, []ast.ASTHandle{x, y})
	must(err)
	negative, err = m.MkNot(negative)
	must(err)
	body, err := m.MkAnd(positive, negative)
	must(err)
	head, err := m.MkApp(output, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, "private-negative-tail"))
	original, err := e.set.Rule(0)
	must(err)
	defer func() { must(original.Close()) }()
	predicateCount := len(e.predicates)
	for query := 0; query < 3; query++ {
		if query == 2 {
			must(e.AddTableFact(blocked, []uint32{0, 1}))
		}
		status, err := e.QueryPredicate(output)
		must(err)
		if status != lbool.FromBool(query < 2) || !e.compiled {
			t.Fatal("private negative variable query", query, status, e.compiled)
		}
		rows, err := e.predicates[output].table.Facts()
		must(err)
		if query < 2 && (len(rows) != 1 || len(rows[0]) != 1 || rows[0][0] != 0) || query == 2 && len(rows) != 0 {
			t.Fatal("existential complement changed rows", query, rows)
		}
		if query == 0 {
			unions := uint32(0)
			for i := 0; i < e.datalogStats.Size(); i++ {
				if e.datalogStats.Key(i) == "dl.union" {
					unions = e.datalogStats.UintValue(i)
				}
			}
			if unions < 2 {
				t.Fatal("private negative tail did not materialize its separate complement", unions)
			}
		}
		current, err := e.set.Rule(0)
		must(err)
		same, err := current.Equal(original)
		must(err)
		must(current.Close())
		if !same || len(e.predicates) != predicateCount {
			t.Fatal("query-local negative abstraction changed the persistent program", query)
		}
	}
}
