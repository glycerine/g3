package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExplanationProjectionUnionBytecodePublishesTypedAndFiniteOutputs(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		d, e := f.ast.MkFuncDecl(sym, sorts, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestPin(t, f.ast, d)
		return d
	}
	explain := func(name string) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		v, e := f.ast.MkConst(sym, ruleSort)
		astTestMust(t, e)
		astTestPin(t, f.ast, v)
		return v
	}
	a, b, c, old := explain("bytecode-A"), explain("bytecode-B"), explain("bytecode-C"), explain("bytecode-old")
	input := declare("bytecode-explanation-input", []ast.ASTHandle{f.boolean, f.boolean, ruleSort})
	typedOutput := declare("bytecode-explanation-output", []ast.ASTHandle{f.boolean, ruleSort})
	finiteOutput := declare("bytecode-finite-output", []ast.ASTHandle{f.boolean})
	source, err := f.relations.GetExplanationRelation(input)
	astTestMust(t, err)
	for _, row := range []struct {
		values      []ast.ASTHandle
		explanation ast.ASTHandle
	}{{[]ast.ASTHandle{f.falsehood, f.falsehood}, a}, {[]ast.ASTHandle{f.falsehood, f.truth}, b}, {[]ast.ASTHandle{f.truth, f.falsehood}, c}} {
		astTestMust(t, source.AddFact(row.values, row.explanation))
	}
	target, err := f.relations.GetExplanationRelation(typedOutput)
	astTestMust(t, err)
	astTestMust(t, target.AddFact([]ast.ASTHandle{f.falsehood}, old))
	finite, err := f.relations.GetRelation(finiteOutput)
	astTestMust(t, err)
	_, err = finite.Add(Fact{0})
	astTestMust(t, err)
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	block := NewInstructionBlock()
	defer func() { astTestMust(t, block.Close()) }()
	appendIO := func(predicate ast.ASTHandle, register uint32, store bool) {
		var instruction Instruction
		var e error
		if store {
			instruction, e = NewStoreInstruction(f.relations, predicate, register)
		} else {
			instruction, e = NewLoadInstruction(f.relations, predicate, register)
		}
		astTestMust(t, e)
		astTestMust(t, block.Append(instruction))
	}
	appendIO(input, 0, false)
	astTestMust(t, block.Append(NewProjectionInstruction(0, []int{1}, 1)))
	appendIO(typedOutput, 2, false)
	astTestMust(t, block.Append(NewUnionInstruction(1, 2, 3)))
	appendIO(typedOutput, 2, true)
	astTestMust(t, block.Append(NewProjectionInstruction(1, []int{1}, 4)))
	appendIO(finiteOutput, 5, false)
	astTestMust(t, block.Append(NewUnionInstruction(4, 5, 6)))
	appendIO(finiteOutput, 5, true)
	widen := NewWidenInstruction(1, 7, 8)
	astTestMust(t, block.Append(widen))
	complete, err := block.Perform(context)
	astTestMust(t, err)
	if !complete {
		t.Fatal("projection/union program did not complete")
	}
	published, err := f.relations.GetExplanationRelation(typedOutput)
	astTestMust(t, err)
	for _, row := range []struct{ value, want ast.ASTHandle }{{f.falsehood, old}, {f.truth, c}} {
		got, found, e := published.Explanation([]ast.ASTHandle{row.value})
		astTestMust(t, e)
		if !found || got != row.want {
			t.Fatal("published explanation", got, found, row.want)
		}
	}
	delta, err := context.ExplanationRegister(3)
	astTestMust(t, err)
	for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
		got, found, e := delta.Explanation([]ast.ASTHandle{value})
		astTestMust(t, e)
		if found != (value == f.truth) || found && got != c {
			t.Fatal("explanation delta", value, got, found)
		}
	}
	finite, err = f.relations.GetRelation(finiteOutput)
	astTestMust(t, err)
	finiteDelta, err := context.Register(6)
	astTestMust(t, err)
	for _, n := range []Element{0, 1} {
		found, e := finite.Contains(Fact{n})
		astTestMust(t, e)
		if !found {
			t.Fatal("missing finite output", n)
		}
		found, e = finiteDelta.Contains(Fact{n})
		astTestMust(t, e)
		if found != (n == 1) {
			t.Fatal("finite delta", n, found)
		}
	}
	// The source register is preserved, including the explanation chosen before
	// projected-key collisions. Output owner transfer does not alias that input.
	projected, err := context.ExplanationRegister(1)
	astTestMust(t, err)
	got, found, err := projected.Explanation([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	if !found || got != a {
		t.Fatal("projection lost first explanation", got, found)
	}
	astTestMust(t, context.MakeEmpty(8))
	complete, err = widen.Perform(context)
	astTestMust(t, err)
	if !complete {
		t.Fatal("repeated widening incomplete")
	}
	if empty, e := context.ExplanationRegister(8); e != nil || empty != nil {
		t.Fatal("unchanged widening published a nonempty delta", empty, e)
	}
	invalid := NewProjectionInstruction(0, []int{3}, 1)
	defer func() { astTestMust(t, invalid.Close()) }()
	if complete, e := invalid.Perform(context); e == nil || complete {
		t.Fatal("invalid projection accepted", complete, e)
	}
	unchanged, err := context.ExplanationRegister(1)
	astTestMust(t, err)
	if unchanged != projected {
		t.Fatal("invalid projection replaced prior result")
	}
	alias := NewUnionInstruction(1, 1, 1)
	defer func() { astTestMust(t, alias.Close()) }()
	if complete, e := alias.Perform(context); e == nil || complete {
		t.Fatal("aliased explanation delta accepted", complete, e)
	}
	stats, err := context.Statistics()
	astTestMust(t, err)
	if stats.Union != 5 || stats.ProjectRename != 3 {
		t.Fatal("typed operation counters", stats.Union, stats.ProjectRename)
	}
}

func TestExplanationProjectionToNullaryOutputBytecode(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	name, err := f.ast.Symbols.Intern("nullary-bytecode-fact")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(name, ruleSort)
	astTestMust(t, err)
	astTestPin(t, f.ast, explanation)
	p, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.truth}, explanation))
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	astTestMust(t, context.SetExplanationRegister(0, p))
	block := NewInstructionBlock()
	defer func() { astTestMust(t, block.Close()) }()
	astTestMust(t, block.Append(NewProjectionInstruction(0, []int{0, 1}, 1)))
	astTestMust(t, block.Append(NewUnionInstruction(1, 2, 3)))
	complete, err := block.Perform(context)
	astTestMust(t, err)
	if !complete {
		t.Fatal("nullary projection program incomplete")
	}
	for _, index := range []uint32{2, 3} {
		table, err := context.Register(index)
		astTestMust(t, err)
		found, err := table.Contains(Fact{})
		astTestMust(t, err)
		if !found {
			t.Fatal("nullary output/delta missing", index)
		}
	}
}
