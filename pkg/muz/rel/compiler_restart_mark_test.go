package rel

// Source: dl_compiler.cpp:1184-1193,1240-1244 marks a stratum only after its
// execution completes; termination STORE must preserve incomplete work for retry.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestFiniteCompilerInterruptedStratumStoresWithoutSaturation(t *testing.T) {
	for _, all := range []bool{false, true} {
		name := "semi-naive"
		if all {
			name = "all-or-nothing"
		}
		t.Run(name, func(t *testing.T) {
			f := newTableRelationFixture(t)
			must := func(err error) { t.Helper(); astTestMust(t, err) }
			rules, err := base.NewRuleManager(f.ast)
			must(err)
			t.Cleanup(func() { must(rules.Close()) })
			set, err := base.NewRuleSet(rules)
			must(err)
			t.Cleanup(func() { must(set.Close()) })
			edge := finiteInterpretedProjectDeclaration(t, f.ast, "restart-edge", []ast.ASTHandle{f.bits, f.bits}, f.boolean)
			reach := finiteInterpretedProjectDeclaration(t, f.ast, "restart-reach", []ast.ASTHandle{f.bits}, f.boolean)
			must(rules.RegisterPredicate(edge))
			must(rules.RegisterPredicate(reach))
			app := func(predicate ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
				result, err := f.ast.MkApp(predicate, arguments)
				must(err)
				return result
			}
			add := func(head ast.ASTHandle, tail ...ast.ASTHandle) {
				rule, err := rules.NewRule(head, tail, nil, "restart")
				must(err)
				must(set.Add(rule))
				must(rule.Close())
			}
			// The lower nonrecursive edge stratum completes before reach begins.
			// The recursive chain needs several rounds to derive reach(3).
			for value := uint64(0); value < 3; value++ {
				add(app(edge, f.bv(value), f.bv(value+1)))
			}
			add(app(reach, f.bv(0)))
			x, err := f.ast.MkVar(0, f.bits)
			must(err)
			y, err := f.ast.MkVar(1, f.bits)
			must(err)
			add(app(reach, y), app(reach, x), app(edge, x, y))
			valid, err := set.Finalize()
			must(err)
			if !valid {
				t.Fatal("restart fixture is not stratified")
			}
			options := FiniteCompilerOptions{AllOrNothingDeltas: all}
			execution, termination, err := CompileFiniteRulesWithOptions(f.relations, set, nil, options)
			must(err)
			t.Cleanup(func() { must(execution.Close()); must(termination.Close()) })
			// Discover the live output register from its STORE binding so the
			// interruption depends on derived rows, not instruction numbering.
			reachRegister := VoidRegister
			for _, instruction := range termination.instructions {
				if store, ok := instruction.(*relationIOInstruction); ok && store.store && store.predicate == reach {
					reachRegister = store.register
				}
			}
			if reachRegister == VoidRegister {
				t.Fatal("recursive result has no termination STORE")
			}
			var context *ExecutionContext
			interrupt, interrupted := true, false
			context, err = NewExecutionContextWithRelations(f.relations, func() bool {
				if !interrupt {
					return false
				}
				table, err := context.FiniteRegister(reachRegister)
				must(err)
				if table == nil {
					return false
				}
				rows, err := table.Len()
				must(err)
				if rows == 2 {
					interrupted = true
					return true
				}
				return false
			})
			must(err)
			t.Cleanup(func() { must(context.Close()) })
			complete, err := execution.Perform(context)
			must(err)
			if complete || !interrupted {
				t.Fatal("execution did not stop after its first recursive progress", complete, interrupted)
			}
			if saturated, err := f.relations.IsSaturated(edge); err != nil || !saturated {
				t.Error("completed lower stratum lost its saturation mark", saturated, err)
			}
			if saturated, err := f.relations.IsSaturated(reach); err != nil || saturated {
				t.Error("interrupted recursive stratum is already saturated", saturated, err)
			}
			interrupt = false
			complete, err = termination.Perform(context)
			must(err)
			if !complete {
				t.Fatal("termination did not store partial relations")
			}
			partial, err := f.relations.GetFiniteRelation(reach)
			must(err)
			if rows, err := partial.Len(); err != nil || rows != 2 {
				t.Fatal("termination lost partial reachability", rows, err)
			}
			for _, value := range []Element{0, 1} {
				if found, err := partial.Contains(Fact{value}); err != nil || !found {
					t.Fatal("termination lost a derived tuple", value, found, err)
				}
			}
			if saturated, err := f.relations.IsSaturated(edge); err != nil || !saturated {
				t.Error("termination cleared completed lower-stratum mark", saturated, err)
			}
			if saturated, err := f.relations.IsSaturated(reach); err != nil || saturated {
				t.Error("termination marked interrupted recursion saturated", saturated, err)
			}
			must(context.Close())
			must(execution.Close())
			must(termination.Close())

			// Do not reset saturation marks: a restart must reuse completed
			// strata while extending the saved partial recursive relation.
			retry, finish, err := CompileFiniteRulesWithOptions(f.relations, set, nil, options)
			must(err)
			t.Cleanup(func() { must(retry.Close()); must(finish.Close()) })
			healthy, err := NewExecutionContextWithRelations(f.relations, nil)
			must(err)
			t.Cleanup(func() { must(healthy.Close()) })
			for _, block := range []*InstructionBlock{retry, finish} {
				complete, err = block.Perform(healthy)
				must(err)
				if !complete {
					t.Fatal("healthy restart did not complete")
				}
			}
			closure, err := f.relations.GetFiniteRelation(reach)
			must(err)
			if rows, err := closure.Len(); err != nil || rows != 4 {
				t.Fatal("restart skipped unfinished recursive closure", rows, err)
			}
			for value := Element(0); value < 4; value++ {
				if found, err := closure.Contains(Fact{value}); err != nil || !found {
					t.Fatal("restart missed reachable tuple", value, found, err)
				}
			}
			for _, predicate := range []ast.ASTHandle{edge, reach} {
				if saturated, err := f.relations.IsSaturated(predicate); err != nil || !saturated {
					t.Fatal("completed restart is not saturated", predicate, saturated, err)
				}
			}
		})
	}
}
