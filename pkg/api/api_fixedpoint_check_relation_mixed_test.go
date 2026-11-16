package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// TestAPIFixedpointCheckDocJoinsBoolTable: AUDIT.md FUZZ-F5 (relation
// plugins). With datalog.check_relation doc, a Bool predicate (no udoc
// signature) holds a table_relation and a bit-vector predicate a
// check_relation over udoc; their join asks check_relation_plugin for a join
// of a table and a check_relation. z3 4.7.1 aborts with std::bad_cast there
// (fork fix: check_relation_plugin offers no operation unless every argument
// is a check_relation, and relation_manager goes on to the other plugin).
// The expected answer is the fixed fork's C API answer
// (Z3_fixedpoint_add_fact, Z3_fixedpoint_query_relations twice): the
// product_relation join keeps no correlation of its inner relations.
func TestAPIFixedpointCheckDocJoinsBoolTable(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetSymbol("engine", "datalog"))
	astTestMust(t, params.SetSymbol("datalog.check_relation", "doc"))
	astTestMust(t, f.fp.SetParameters(params))
	astTestMust(t, params.Close())
	boolean, err := f.c.BoolSort()
	astTestMust(t, err)
	bv, err := f.c.BitVectorSort(3)
	astTestMust(t, err)
	p, err := f.fp.DeclareRelation("p", []ast.ASTHandle{boolean})
	astTestMust(t, err)
	q, err := f.fp.DeclareRelation("q", []ast.ASTHandle{bv})
	astTestMust(t, err)
	r, err := f.fp.DeclareRelation("r", []ast.ASTHandle{bv})
	astTestMust(t, err)
	astTestMust(t, f.fp.AddTableFact(p, []uint32{1}))
	astTestMust(t, f.fp.AddTableFact(q, []uint32{5}))
	x := f.pin(f.c.BoundVariable(1, boolean))
	y := f.pin(f.c.BoundVariable(0, bv))
	body := f.pin(f.m.MkAndArray([]ast.ASTHandle{f.pin(f.m.MkApp(p, []ast.ASTHandle{x})), f.pin(f.m.MkApp(q, []ast.ASTHandle{y}))}))
	xName, err := f.m.Symbols.Intern("x")
	astTestMust(t, err)
	yName, err := f.m.Symbols.Intern("y")
	astTestMust(t, err)
	rule := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean, bv}, Names: []symbol.Symbol{xName, yName}, Body: f.pin(f.m.MkImplies(body, f.pin(f.m.MkApp(r, []ast.ASTHandle{y}))))}))
	astTestMust(t, f.fp.AddRule(rule, "mixed"))
	want := "(and (= (:var 0) #b101)\n     (or (= (:var 0) #b000)\n         (= (:var 0) #b001)\n         (= (:var 0) #b010)\n         (= (:var 0) #b011)\n         (= (:var 0) #b100)\n         (= (:var 0) #b101)\n         (= (:var 0) #b110)\n         (= (:var 0) #b111)))"
	for run := 0; run < 2; run++ {
		status, err := f.fp.QueryRelations([]ast.ASTHandle{r})
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("status", run, status)
		}
		answer, err := f.fp.LastAnswer()
		astTestMust(t, err)
		h, err := f.m.RefGet(answer)
		astTestMust(t, err)
		text, err := f.c.ASTString(h, APIASTPrintSMTFull)
		astTestMust(t, err)
		astTestMust(t, f.m.DelRef(&answer))
		if text != want {
			t.Fatalf("answer %d:\n%s\nwant\n%s", run, text, want)
		}
	}
}
