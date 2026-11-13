package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: dl_compiler.cpp compile_rule_evaluation uses full tails once per
// recursive iteration with all_or_nothing_deltas, instead of one evaluation
// for each recursive tail's delta. Both modes compute the same finite closure.
func TestDatalogFiniteAllOrNothingDeltaOption(t *testing.T) {
	trueValue, falseValue := true, false
	for _, tc := range []struct {
		name          string
		module, local *bool
		all           bool
	}{
		{name: "default"},
		{name: "module", module: &trueValue, all: true},
		{name: "local", local: &trueValue, all: true},
		{name: "local-override", module: &trueValue, local: &falseValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			m, globals := newPDRLifecycleServices(t)
			for _, key := range []string{"fixedpoint.datalog.unbound_compressor", "fixedpoint.datalog.similarity_compressor"} {
				must(globals.SourceSetName(key, "false"))
			}
			if tc.module != nil {
				value := "false"
				if *tc.module {
					value = "true"
				}
				must(globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", value))
			}
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(e.Close()) })
			must(e.SelectEngine("datalog"))
			if tc.local != nil {
				local := globals.Context.EmptyValue()
				key := "datalog.all_or_nothing_deltas"
				must(globals.Context.SetBoolName(&local, &key, *tc.local))
				must(e.SetQueryParameters(&local))
				must(globals.Context.Del(&local))
			}
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("finite-delta-path")
			must(err)
			predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(predicate))
			must(e.AddTableFact(predicate, []uint32{0, 1}))
			must(e.AddTableFact(predicate, []uint32{1, 0}))
			variables := make([]ast.ASTHandle, 3)
			for index := range variables {
				variables[index], err = m.MkVar(uint32(index), boolean)
				must(err)
			}
			application := func(first, second int) ast.ASTHandle {
				t.Helper()
				result, err := m.MkApp(predicate, []ast.ASTHandle{variables[first], variables[second]})
				must(err)
				return result
			}
			body, err := m.MkAnd(application(0, 1), application(1, 2))
			must(err)
			rule, err := m.MkImplies(body, application(0, 2))
			must(err)
			must(e.addRuleEager(rule, "finite-delta-transitive"))
			for query := 0; query < 2; query++ {
				status, err := e.QueryPredicate(predicate)
				must(err)
				if status != lbool.True || !e.compiled {
					t.Fatal("finite transitive query was not compiled", status, e.compiled)
				}
				rows, err := e.predicates[predicate].table.Facts()
				must(err)
				seen := [4]bool{}
				for _, row := range rows {
					if len(row) != 2 || row[0] > 1 || row[1] > 1 {
						t.Fatal("invalid Boolean closure row", row)
					}
					seen[2*row[0]+row[1]] = true
				}
				if len(rows) != 4 || seen != [4]bool{true, true, true, true} {
					t.Fatal("delta option changed transitive closure", rows)
				}
				joins, ordinaryJoins := uint32(0), uint32(0)
				for index := 0; index < e.datalogStats.Size(); index++ {
					switch e.datalogStats.Key(index) {
					case "dl.join_project":
						joins = e.datalogStats.UintValue(index)
					case "dl.join":
						ordinaryJoins = e.datalogStats.UintValue(index)
					}
				}
				// rel_context's m_ectx counters accumulate across queries:
				// the second query adds its 2 join_projects to the first's 4.
				want := uint32(4)
				if query != 0 {
					want = 6
				}
				if tc.all {
					want /= 2
				}
				if joins != want || ordinaryJoins != 0 {
					t.Fatal("recursive delta execution counts", query, joins, ordinaryJoins, "want", want, 0)
				}
			}
		})
	}
}
