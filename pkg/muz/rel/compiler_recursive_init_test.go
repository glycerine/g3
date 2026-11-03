package rel

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// Source: dl_compiler.cpp compile_preds_init seeds each recursive predicate
// before evaluating recursive rules, then clones all accumulated rows as deltas.
func TestFiniteCompilerInitializesRecursiveSeedsBeforeLoop(t *testing.T) {
	for _, backing := range []string{"hash", "sparse", "lazy-sparse", "checked"} {
		for _, seeds := range []string{"ground", "preloaded", "lower-stratum", "empty"} {
			t.Run(backing+"/"+seeds, func(t *testing.T) {
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
				x, err := f.ast.MkVar(0, f.boolean)
				must(err)
				predicate := func(name string, rows ...Fact) ast.ASTHandle {
					t.Helper()
					decl := finiteInterpretedProjectDeclaration(t, f.ast, name, []ast.ASTHandle{f.boolean}, f.boolean)
					must(rules.RegisterPredicate(decl))
					_, signature, err := f.relations.PredicateSignature(decl)
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
					for _, row := range rows {
						_, err = table.Add(row)
						must(err)
					}
					must(f.relations.StoreFiniteRelation(decl, table))
					return decl
				}
				application := func(decl, value ast.ASTHandle) ast.ASTHandle {
					t.Helper()
					app, err := f.ast.MkApp(decl, []ast.ASTHandle{value})
					must(err)
					return app
				}
				add := func(head ast.ASTHandle, tails []ast.ASTHandle, negative []bool) {
					t.Helper()
					rule, err := rules.NewRule(head, tails, negative, "recursive-seed")
					must(err)
					must(set.Add(rule))
					must(rule.Close())
				}
				var pRows, qRows []Fact
				if seeds == "preloaded" {
					pRows, qRows = []Fact{{1}}, []Fact{{0}}
				}
				p, q := predicate("seed-P", pRows...), predicate("seed-Q", qRows...)
				px, qx := application(p, x), application(q, x)
				// Both recursive rules precede the nonrecursive rules for their heads.
				// Each head must still acquire its own seed before the other seed.
				add(px, []ast.ASTHandle{qx}, nil)
				add(qx, []ast.ASTHandle{px}, nil)
				switch seeds {
				case "ground":
					add(application(p, f.truth), nil, nil)
					add(application(q, f.falsehood), nil, nil)
				case "lower-stratum":
					left, right, blocked := predicate("seed-Left", Fact{1}), predicate("seed-Right", Fact{0}), predicate("seed-Blocked")
					add(px, []ast.ASTHandle{application(left, x), application(blocked, x)}, []bool{false, true})
					add(qx, []ast.ASTHandle{application(right, x), application(blocked, x)}, []bool{false, true})
				}
				valid, err := set.Finalize()
				must(err)
				if !valid {
					t.Fatal("unstratified recursive fixture")
				}
				execution, termination, err := CompileFiniteRules(f.relations, set)
				must(err)
				t.Cleanup(func() { must(execution.Close()); must(termination.Close()) })
				context, err := NewExecutionContextWithRelations(f.relations, nil)
				must(err)
				t.Cleanup(func() { must(context.Close()) })
				for _, block := range []*InstructionBlock{execution, termination} {
					complete, err := block.Perform(context)
					must(err)
					if !complete {
						t.Fatal("incomplete recursive execution")
					}
				}
				for _, want := range []struct {
					predicate ast.ASTHandle
					rows      []Fact
				}{{p, []Fact{{1}, {0}}}, {q, []Fact{{0}, {1}}}} {
					table, err := f.relations.GetFiniteRelation(want.predicate)
					must(err)
					rows, err := table.Facts()
					must(err)
					if seeds == "empty" {
						if len(rows) != 0 {
							t.Fatal("unfounded cycle acquired rows", rows)
						}
					} else if !reflect.DeepEqual(rows, want.rows) {
						t.Fatalf("recursive initialization order: got %v, want %v", rows, want.rows)
					}
					saturated, err := f.relations.IsSaturated(want.predicate)
					must(err)
					if !saturated {
						t.Fatal("recursive predicate was not saturated")
					}
				}
			})
		}
	}
}
