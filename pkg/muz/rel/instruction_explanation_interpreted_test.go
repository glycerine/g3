package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExplanationAssignmentBytecodeMapsSparseVariablesAndPublishesHead(t *testing.T) {
	for _, nullary := range []bool{false, true} {
		name := "finite"
		if nullary {
			name = "nullary"
		}
		t.Run(name, func(t *testing.T) {
			f := newTableRelationFixture(t)
			own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
			ruleSort := own(f.ast.MkDatalogRuleSort())
			symbol := func(text string) ast.ASTHandle {
				n, e := f.ast.Symbols.Intern(text)
				astTestMust(t, e)
				return own(f.ast.MkDatalogRule(n, nil))
			}
			a, b := symbol("assignment-tail-A"), symbol("assignment-tail-B")
			step, e := f.ast.Symbols.Intern("assignment-step")
			astTestMust(t, e)
			tail := own(f.ast.MkVar(3, ruleSort))
			head := own(f.ast.MkVar(7, ruleSort))
			expression := own(f.ast.MkDatalogRule(step, []ast.ASTHandle{tail}))
			assignment := own(f.ast.MkEq(head, expression))
			var finiteSorts []ast.ASTHandle
			if !nullary {
				finiteSorts = []ast.ASTHandle{f.boolean}
			}
			inputSorts := append(append([]ast.ASTHandle(nil), finiteSorts...), ruleSort, ruleSort)
			outputSorts := append(append([]ast.ASTHandle(nil), finiteSorts...), ruleSort)
			declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
				n, e := f.ast.Symbols.Intern(name)
				astTestMust(t, e)
				return own(f.ast.MkFuncDecl(n, sorts, f.boolean, ast.ASTFuncDeclInfo{}))
			}
			input := declare("assignment-input", inputSorts)
			output := declare("assignment-output", outputSorts)
			source, e := f.relations.GetExplanationRelation(input)
			astTestMust(t, e)
			key := Fact{0}
			if !nullary {
				key = Fact{1, 0}
			}
			astTestMust(t, source.addEncodedRow(key, []ast.ASTHandle{a, {}}))
			if !nullary {
				astTestMust(t, source.addEncodedRow(Fact{0, 0}, []ast.ASTHandle{b, {}}))
			}
			columns := map[uint32]int{3: len(finiteSorts), 7: len(finiteSorts) + 1}
			sorts := map[uint32]ast.ASTHandle{3: ruleSort, 7: ruleSort}
			conditions := []ast.ASTHandle{assignment}
			if !nullary {
				v := own(f.ast.MkVar(0, f.boolean))
				guard := own(f.ast.MkEq(v, f.truth))
				conditions = append([]ast.ASTHandle{guard}, conditions...)
				columns[0] = 0
				sorts[0] = f.boolean
			}
			context, e := NewExecutionContextWithRelations(f.relations, nil)
			astTestMust(t, e)
			defer func() { astTestMust(t, context.Close()) }()
			block := NewInstructionBlock()
			defer func() { astTestMust(t, block.Close()) }()
			load, e := NewLoadInstruction(f.relations, input, 0)
			astTestMust(t, e)
			astTestMust(t, block.Append(load))
			filter, e := NewFilterInterpretedInstruction(f.ast, f.relations, tableRelationTheoryRewriter(t, f.ast), 0, conditions, columns, sorts)
			astTestMust(t, e)
			astTestMust(t, block.Append(filter))
			astTestMust(t, block.Append(NewProjectionInstruction(0, []int{len(finiteSorts)}, 1)))
			astTestMust(t, block.Append(NewUnionInstruction(1, 2, VoidRegister)))
			store, e := NewStoreInstruction(f.relations, output, 2)
			astTestMust(t, e)
			astTestMust(t, block.Append(store))
			complete, e := block.Perform(context)
			astTestMust(t, e)
			if !complete {
				t.Fatal("explanation assignment program incomplete")
			}
			product, e := f.relations.GetExplanationRelation(output)
			astTestMust(t, e)
			var values []ast.ASTHandle
			if !nullary {
				values = []ast.ASTHandle{f.truth}
			}
			got, found, e := product.Explanation(values)
			astTestMust(t, e)
			want := own(f.ast.MkDatalogRule(step, []ast.ASTHandle{a}))
			if !found || got != want {
				t.Fatal("head explanation does not match grounded rule", got, found, want)
			}
			if !nullary {
				_, found, e = product.Explanation([]ast.ASTHandle{f.falsehood})
				astTestMust(t, e)
				if found {
					t.Fatal("finite guard failed to remove false tuple")
				}
			}
			// Load cloned the managed row. Its undefined head slot and both source
			// tuples remain unchanged by filtering and explanation assignment.
			sourceRow, found, e := source.ExplanationRow(values)
			astTestMust(t, e)
			if !found || len(sourceRow) != 2 || sourceRow[0] != a || !sourceRow[1].IsNull() {
				t.Fatal("assignment mutated manager input", sourceRow, found)
			}
			if !nullary {
				if count, e := source.Len(); e != nil || count != 2 {
					t.Fatal("finite filter mutated manager input", count, e)
				}
			}
			if complete, e = filter.Perform(context); e == nil || complete {
				t.Fatal("already assigned explanation slot accepted", complete, e)
			}
			badColumns := map[uint32]int{3: len(finiteSorts), 7: len(finiteSorts) + 2}
			bad, e := NewFilterInterpretedInstruction(f.ast, f.relations, tableRelationTheoryRewriter(t, f.ast), 0, []ast.ASTHandle{assignment}, badColumns, sorts)
			astTestMust(t, e)
			if complete, e = bad.Perform(context); e == nil || complete {
				t.Fatal("invalid explanation column accepted", complete, e)
			}
			astTestMust(t, bad.Close())
			// The published explanation has a distinct AST owner independent of all
			// bytecode condition roots and manager/register relation owners.
			result, e := f.ast.NewRef(got)
			astTestMust(t, e)
			defer func() { astTestMust(t, f.ast.DelRef(&result)) }()
			astTestMust(t, block.Close())
			astTestMust(t, context.Close())
			astTestMust(t, f.relations.ResetRelations())
			kept, e := f.ast.RefGet(result)
			astTestMust(t, e)
			if kept != want {
				t.Fatal("owning head explanation expired")
			}
		})
	}
}
