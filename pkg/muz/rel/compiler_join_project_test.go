package rel

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// Source: dl_compiler.cpp get_local_indexes_for_projection and
// compile_rule_evaluation_run. Negative indexes denote Boolean constants.
func TestFiniteCompilerFusesJoinProjection(t *testing.T) {
	cases := []struct {
		name                   string
		left, right, head      []int
		leftRows, rightRows    []Fact
		want                   []Fact
		guard, negative        bool
		join, joinProject      uint32
		project, filterProject uint32
	}{
		{"shared-key", []int{0, 1}, []int{1, 2}, []int{0, 2}, []Fact{{0, 0}, {1, 1}}, []Fact{{0, 1}, {1, 0}}, []Fact{{0, 1}, {1, 0}}, false, false, 0, 1, 0, 0},
		{"repeated-head", []int{0, 1}, []int{0, 2}, []int{0, 0}, []Fact{{0, 0}, {1, 1}}, []Fact{{0, 1}, {1, 0}}, []Fact{{0, 0}, {1, 1}}, false, false, 0, 1, 0, 0},
		{"repeated-local", []int{0, 1, 1}, []int{2}, []int{0}, []Fact{{0, 0, 1}, {1, 1, 1}}, []Fact{{0}}, []Fact{{1}}, false, false, 0, 1, 0, 0},
		{"nullary", []int{0}, []int{0}, nil, []Fact{{0}, {1}}, []Fact{{1}}, []Fact{{}}, false, false, 0, 1, 0, 0},
		{"guard-variable", []int{0, 1}, []int{1, 2}, []int{0}, []Fact{{0, 0}, {1, 1}}, []Fact{{0, 1}, {1, 0}}, []Fact{{1}}, true, false, 0, 1, 0, 1},
		{"negative-variable", []int{0, 1}, []int{1, 2}, []int{0}, []Fact{{0, 0}, {1, 1}}, []Fact{{0, 1}, {1, 0}}, []Fact{{1}}, false, true, 0, 1, 1, 0},
		{"constant-column", []int{0, -1}, []int{1, 0}, []int{1}, []Fact{{0, 1}, {1, 0}}, []Fact{{1, 0}, {0, 1}}, []Fact{{1}}, false, false, 0, 1, 0, 0},
		{"all-columns-needed", []int{0}, []int{1}, []int{0, 1}, []Fact{{0}}, []Fact{{1}}, []Fact{{0, 1}}, false, false, 1, 0, 0, 0},
	}
	for _, backing := range []string{"hash", "sparse", "lazy-sparse", "checked"} {
		for _, tc := range cases {
			t.Run(backing+"/"+tc.name, func(t *testing.T) {
				f := newTableRelationFixture(t)
				must := func(err error) { t.Helper(); astTestMust(t, err) }
				lazy := NewLazySparseEngine()
				t.Cleanup(func() { must(lazy.Close()) })
				rules, err := base.NewRuleManager(f.ast)
				must(err)
				t.Cleanup(func() { must(rules.Close()) })
				set, err := base.NewRuleSet(rules)
				must(err)
				t.Cleanup(func() { must(set.Close()) })
				vars := make([]ast.ASTHandle, 3)
				for index := range vars {
					vars[index], err = f.ast.MkVar(uint32(index), f.boolean)
					must(err)
				}
				makePredicate := func(name string, columns []int, rows []Fact) (ast.ASTHandle, ast.ASTHandle) {
					t.Helper()
					sorts := make([]ast.ASTHandle, len(columns))
					args := make([]ast.ASTHandle, len(columns))
					for index, column := range columns {
						sorts[index] = f.boolean
						if column < 0 {
							args[index] = f.truth
						} else {
							args[index] = vars[column]
						}
					}
					predicate := finiteInterpretedProjectDeclaration(t, f.ast, "join-project-"+name, sorts, f.boolean)
					must(rules.RegisterPredicate(predicate))
					_, signature, err := f.relations.PredicateSignature(predicate)
					must(err)
					var table FiniteTable
					switch {
					case len(columns) == 0, backing == "hash":
						table = NewHashTable(signature)
					case backing == "sparse":
						table, err = NewSparseTable(signature, 0)
					case backing == "lazy-sparse":
						table, err = lazy.New(signature, 0)
					case backing == "checked":
						table = checkedPairForInstruction(t, signature)
					}
					must(err)
					for _, row := range rows {
						_, err = table.Add(row)
						must(err)
					}
					must(f.relations.StoreFiniteRelation(predicate, table))
					application, err := f.ast.MkApp(predicate, args)
					must(err)
					return predicate, application
				}
				left, leftApp := makePredicate("left", tc.left, tc.leftRows)
				right, rightApp := makePredicate("right", tc.right, tc.rightRows)
				head, headApp := makePredicate("head", tc.head, nil)
				tails := []ast.ASTHandle{leftApp, rightApp}
				negated := []bool{false, false}
				if tc.negative {
					_, negative := makePredicate("negative", []int{2}, []Fact{{1}})
					tails = append(tails, negative)
					negated = append(negated, true)
				}
				if tc.guard {
					guard, err := f.ast.MkEq(vars[1], f.truth)
					must(err)
					tails = append(tails, guard)
					negated = append(negated, false)
				}
				rule, err := rules.NewRule(headApp, tails, negated, "join-project-rule")
				must(err)
				must(set.Add(rule))
				must(rule.Close())
				valid, err := set.Finalize()
				must(err)
				if !valid {
					t.Fatal("unstratified join fixture")
				}
				execution, termination, err := CompileFiniteRulesWithGuard(f.relations, set, tableRelationTheoryRewriter(t, f.ast))
				must(err)
				t.Cleanup(func() { must(execution.Close()); must(termination.Close()) })
				context, err := NewExecutionContextWithRelations(f.relations, nil)
				must(err)
				t.Cleanup(func() { must(context.Close()) })
				for _, block := range []*InstructionBlock{execution, termination} {
					complete, err := block.Perform(context)
					must(err)
					if !complete {
						t.Fatal("incomplete compiled join")
					}
				}
				for _, want := range []struct {
					predicate ast.ASTHandle
					facts     []Fact
				}{{head, tc.want}, {left, tc.leftRows}, {right, tc.rightRows}} {
					table, err := f.relations.GetFiniteRelation(want.predicate)
					must(err)
					finiteInterpretedProjectWantTable(t, table, want.facts...)
					signature, err := table.Signature()
					must(err)
					matches := false
					switch {
					case signature.Len() == 0, backing == "hash":
						_, matches = table.(*HashTable)
					case backing == "sparse":
						_, matches = table.(*SparseTable)
					case backing == "lazy-sparse":
						_, matches = table.(*LazySparseTable)
					case backing == "checked":
						_, matches = table.(*FiniteCheckedPair)
					}
					if !matches {
						t.Fatalf("compiled join changed %s backing to %T", backing, table)
					}
				}
				stats, err := context.Statistics()
				must(err)
				if stats.Join != tc.join || stats.JoinProject != tc.joinProject || stats.ProjectRename != tc.project || stats.FilterInterpretedProject != tc.filterProject {
					t.Fatal("join projection counters", fmt.Sprintf("%+v", stats))
				}
			})
		}
	}
}
