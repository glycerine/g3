package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazySparseCompiledInterpretedFilterDefersAndFusesProject(t *testing.T) {
	f := newTableRelationFixture(t)
	theory := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazySparseEngine()
	t.Cleanup(func() { _ = engine.Close() })
	sorts := []ast.ASTHandle{f.bits, f.boolean}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	table, err := engine.New(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = table.Add(Fact{1, 0})
	_, _ = table.Add(Fact{2, 1})
	shared, err := table.Project(nil)
	if err != nil {
		t.Fatal(err)
	}
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := f.ast.MkEq(variable, f.truth)
	if err != nil {
		t.Fatal(err)
	}
	baseline := f.ast.Stats().LiveReferences
	instruction, err := NewFilterInterpretedInstruction(
		f.ast, f.relations, theory, 0,
		[]ast.ASTHandle{condition, f.truth},
		map[uint32]int{0: 1}, map[uint32]ast.ASTHandle{0: f.boolean},
	)
	if err != nil {
		t.Fatal(err)
	}
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	if err = context.SetFiniteRegister(0, table); err != nil {
		t.Fatal(err)
	}
	complete, err := instruction.Perform(context)
	if err != nil || !complete {
		t.Fatal("compiled interpreted filter", complete, err)
	}
	if table.Forced() {
		t.Fatal("compiled interpreted filter forced eagerly")
	}
	if got := f.ast.Stats().LiveReferences; got != baseline+4 {
		// Two instruction refs and two independent lazy-node refs.
		t.Fatal("condition ownership before instruction close", got, baseline)
	}
	if err = instruction.Close(); err != nil {
		t.Fatal(err)
	}
	if got := f.ast.Stats().LiveReferences; got != baseline+2 {
		t.Fatal("lazy conditions did not outlive instruction", got, baseline)
	}
	projectedValue, err := finiteTableProject(table, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	projected := projectedValue.(*LazySparseTable)
	if projected.Forced() {
		t.Fatal("interpreted/project fusion was eager")
	}
	facts, err := projected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{2})
	if table.root.left != nil {
		t.Fatal("forced compiler filter kept source edge")
	}
	sharedFacts, err := shared.Facts()
	if err != nil {
		t.Fatal("shared source was consumed", err)
	}
	instructionWantFacts(t, sharedFacts, Fact{1, 0}, Fact{2, 1})
}
