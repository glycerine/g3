package rel

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// Source: dl_compiler.cpp compile_rule_evaluation runs full tails once when
// all_or_nothing_deltas is enabled, including rules with no recursive tails.
func TestFiniteCompilerAllOrNothingDeltas(t *testing.T) {
	for _, backing := range []string{"hash", "sparse", "lazy-sparse", "checked"} {
		for _, all := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/all=%v", backing, all), func(t *testing.T) {
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
				predicate := finiteInterpretedProjectDeclaration(t, f.ast, "delta-Path", []ast.ASTHandle{f.boolean, f.boolean}, f.boolean)
				must(rules.RegisterPredicate(predicate))
				_, signature, err := f.relations.PredicateSignature(predicate)
				must(err)
				var table FiniteTable
				switch backing {
				case "hash":
					table = NewHashTable(signature)
				case "sparse":
					table, err = NewSparseTable(signature, 0)
				case "lazy-sparse":
					table, err = lazy.New(signature, 0)
				case "checked":
					table = checkedPairForInstruction(t, signature)
				}
				must(err)
				must(f.relations.StoreFiniteRelation(predicate, table))
				vars := make([]ast.ASTHandle, 3)
				for index := range vars {
					vars[index], err = f.ast.MkVar(uint32(index), f.boolean)
					must(err)
				}
				app := func(first, second ast.ASTHandle) ast.ASTHandle {
					t.Helper()
					value, err := f.ast.MkApp(predicate, []ast.ASTHandle{first, second})
					must(err)
					return value
				}
				add := func(head ast.ASTHandle, tails ...ast.ASTHandle) {
					t.Helper()
					rule, err := rules.NewRule(head, tails, nil, "delta-path")
					must(err)
					must(set.Add(rule))
					must(rule.Close())
				}
				add(app(vars[0], vars[2]), app(vars[0], vars[1]), app(vars[1], vars[2]))
				add(app(f.falsehood, f.truth))
				add(app(f.truth, f.falsehood))
				valid, err := set.Finalize()
				must(err)
				if !valid {
					t.Fatal("unstratified transitive closure")
				}
				execution, termination, err := CompileFiniteRulesWithOptions(f.relations, set, nil, FiniteCompilerOptions{AllOrNothingDeltas: all})
				must(err)
				t.Cleanup(func() { must(execution.Close()); must(termination.Close()) })
				var loop *whileInstruction
				for _, instruction := range execution.instructions {
					if candidate, ok := instruction.(*whileInstruction); ok {
						loop = candidate
					}
				}
				if loop == nil || len(loop.controls) != 1 {
					t.Fatal("missing recursive delta control")
				}
				joins, unions := 0, 0
				for _, instruction := range loop.body.instructions {
					operation, ok := instruction.(*tableInstruction)
					if !ok {
						continue
					}
					switch operation.kind {
					case "join_project":
						joins++
						if all {
							if operation.first != operation.second || operation.first == loop.controls[0] {
								t.Fatal("all-or-nothing join did not use both full relations")
							}
						} else if (operation.first == loop.controls[0]) == (operation.second == loop.controls[0]) {
							t.Fatal("semi-naive join must substitute exactly one delta")
						}
					case "union":
						unions++
					}
				}
				wantJoins, wantUnions := 2, 2
				if all {
					wantJoins, wantUnions = 1, 3
				}
				if joins != wantJoins || unions != wantUnions {
					t.Fatalf("loop joins/unions %d/%d, want %d/%d", joins, unions, wantJoins, wantUnions)
				}
				context, err := NewExecutionContextWithRelations(f.relations, nil)
				must(err)
				t.Cleanup(func() { must(context.Close()) })
				for _, block := range []*InstructionBlock{execution, termination} {
					complete, err := block.Perform(context)
					must(err)
					if !complete {
						t.Fatal("incomplete transitive closure")
					}
				}
				table, err = f.relations.GetFiniteRelation(predicate)
				must(err)
				finiteInterpretedProjectWantTable(t, table, Fact{0, 0}, Fact{0, 1}, Fact{1, 0}, Fact{1, 1})
				stats, err := context.Statistics()
				must(err)
				if stats.JoinProject != uint32(2*wantJoins) || stats.Union != uint32(2+2*wantUnions) {
					t.Fatalf("two-iteration evaluation counters: %+v", stats)
				}
			})
		}
	}
}
