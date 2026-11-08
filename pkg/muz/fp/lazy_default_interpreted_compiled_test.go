package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogDefaultLazySparseCompiledInterpretedProject(t *testing.T) {
	f := newDirectFactFixture(t)
	e, manager := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "lazy_sparse"))
	boolean, err := manager.MkBoolSort()
	must(err)
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, newErr := manager.Symbols.Intern(name)
		must(newErr)
		sorts := make([]ast.ASTHandle, arity)
		for index := range sorts {
			sorts[index] = boolean
		}
		declaration, newErr := manager.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(newErr)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	predicate := declare("default-lazy-interpreted-source", 2)
	result := declare("default-lazy-interpreted-result", 1)
	truth, err := manager.MkTrue()
	must(err)
	falsity, err := manager.MkFalse()
	must(err)
	for _, fact := range [][]ast.ASTHandle{{falsity, falsity}, {truth, truth}, {truth, falsity}} {
		must(e.AddFact(predicate, fact))
	}
	x, err := manager.MkVar(1, boolean)
	must(err)
	y, err := manager.MkVar(0, boolean)
	must(err)
	tail, err := manager.MkApp(predicate, []ast.ASTHandle{x, y})
	must(err)
	guard, err := manager.MkEq(y, truth)
	must(err)
	body, err := manager.MkAnd(tail, guard)
	must(err)
	head, err := manager.MkApp(result, []ast.ASTHandle{x})
	must(err)
	rule, err := manager.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, "default-lazy-interpreted-project"))
	status, err := e.QueryPredicate(result)
	must(err)
	if status != lbool.True {
		t.Fatal("guarded lazy result", status)
	}
	compiled, err := e.LastQueryUsedCompiledCode()
	must(err)
	if !compiled {
		t.Fatal("guarded lazy query bypassed relational bytecode")
	}
	table := e.predicates[result].table
	if _, ok := table.(*rel.LazySparseTable); !ok || !e.lazyFinite.Owns(table) {
		t.Fatalf("guarded result representation %T", table)
	}
	facts, err := table.Facts()
	must(err)
	if len(facts) != 1 || len(facts[0]) != 1 || facts[0][0] != 1 {
		t.Fatal("guarded lazy tuples", facts)
	}
}
