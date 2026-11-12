package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func finiteInterpretedProjectWantTable(t *testing.T, table FiniteTable, expected ...Fact) {
	t.Helper()
	count, err := table.Len()
	if err != nil || count != len(expected) {
		t.Fatal("fact count", count, expected, err)
	}
	for _, fact := range expected {
		found, findErr := table.Contains(fact)
		if findErr != nil || !found {
			t.Fatal("missing fact", fact, findErr)
		}
	}
}

func TestFilterInterpretedAndProjectInstructionKeepsSourceAndClearsEmptyInput(t *testing.T) {
	f := newTableRelationFixture(t)
	theory := tableRelationTheoryRewriter(t, f.ast)
	signature, err := tableRelationSignature(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	source := NewHashTable(signature)
	for _, fact := range []Fact{{0, 0}, {0, 1}, {1, 1}} {
		if _, err = source.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := f.ast.MkEq(variable, f.truth)
	if err != nil {
		t.Fatal(err)
	}
	instruction, err := NewFilterInterpretedAndProjectInstruction(
		f.ast, f.relations, theory, 0,
		[]ast.ASTHandle{condition}, map[uint32]int{0: 1}, map[uint32]ast.ASTHandle{0: f.boolean},
		[]ast.ASTHandle{f.boolean, f.boolean}, []int{1}, 1,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = instruction.Close() })
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	if err = context.SetFiniteRegister(0, source); err != nil {
		t.Fatal(err)
	}
	complete, err := instruction.Perform(context)
	if err != nil || !complete {
		t.Fatal("fused instruction", complete, err)
	}
	sourceFacts, err := source.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, sourceFacts, Fact{0, 0}, Fact{0, 1}, Fact{1, 1})
	result, err := context.FiniteRegister(1)
	if err != nil || result == nil {
		t.Fatal("fused result", result, err)
	}
	resultFacts, err := result.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, resultFacts, Fact{0}, Fact{1})
	stats, err := context.Statistics()
	if err != nil || stats.FilterInterpretedProject != 1 || stats.Filter != 0 || stats.ProjectRename != 0 {
		t.Fatal("fused counters", stats, err)
	}
	if err = context.MakeEmpty(0); err != nil {
		t.Fatal(err)
	}
	complete, err = instruction.Perform(context)
	if err != nil || !complete {
		t.Fatal("empty fused instruction", complete, err)
	}
	result, err = context.FiniteRegister(1)
	if err != nil || result != nil {
		t.Fatal("empty source did not clear result", result, err)
	}
	stats, err = context.Statistics()
	if err != nil || stats.FilterInterpretedProject != 1 {
		t.Fatal("empty source changed fused counter", stats, err)
	}
}

func TestFilterInterpretedAndProjectInstructionConcreteFiniteDispatch(t *testing.T) {
	tests := []struct {
		name string
		make func(*testing.T, Signature) FiniteTable
	}{
		{"sparse", func(t *testing.T, signature Signature) FiniteTable {
			table, err := NewSparseTable(signature, 0)
			if err != nil {
				t.Fatal(err)
			}
			return table
		}},
		{"lazy-sparse", func(t *testing.T, signature Signature) FiniteTable {
			engine := NewLazySparseEngine()
			t.Cleanup(func() { _ = engine.Close() })
			table, err := engine.New(signature, 0)
			if err != nil {
				t.Fatal(err)
			}
			return table
		}},
		{"checked", func(t *testing.T, signature Signature) FiniteTable {
			return checkedPairForInstruction(t, signature)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newTableRelationFixture(t)
			signature, err := tableRelationSignature(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
			if err != nil {
				t.Fatal(err)
			}
			source := test.make(t, signature)
			for _, fact := range []Fact{{0, 0}, {0, 1}, {1, 1}} {
				if _, err = source.Add(fact); err != nil {
					t.Fatal(err)
				}
			}
			variable, err := f.ast.MkVar(0, f.boolean)
			if err != nil {
				t.Fatal(err)
			}
			condition, err := f.ast.MkEq(variable, f.truth)
			if err != nil {
				t.Fatal(err)
			}
			instruction, err := NewFilterInterpretedAndProjectInstruction(
				f.ast, f.relations, tableRelationTheoryRewriter(t, f.ast), 0,
				[]ast.ASTHandle{condition}, map[uint32]int{0: 1}, map[uint32]ast.ASTHandle{0: f.boolean},
				[]ast.ASTHandle{f.boolean, f.boolean}, []int{1}, 1,
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = instruction.Close() })
			context, err := NewExecutionContextWithRelations(f.relations, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = context.Close() })
			if err = context.SetFiniteRegister(0, source); err != nil {
				t.Fatal(err)
			}
			complete, err := instruction.Perform(context)
			if err != nil || !complete {
				t.Fatal("concrete fused instruction", complete, err)
			}
			result, err := context.FiniteRegister(1)
			if err != nil || result == nil {
				t.Fatal("concrete fused result", result, err)
			}
			if lazy, ok := result.(*LazySparseTable); ok && lazy.Forced() {
				t.Fatal("lazy fused result was forced eagerly")
			}
			finiteInterpretedProjectWantTable(t, result, Fact{0}, Fact{1})
			finiteInterpretedProjectWantTable(t, source, Fact{0, 0}, Fact{0, 1}, Fact{1, 1})
			stats, err := context.Statistics()
			if err != nil || stats.FilterInterpretedProject != 1 || stats.Filter != 0 || stats.ProjectRename != 0 {
				t.Fatal("concrete fused counters", stats, err)
			}
		})
	}
}

func finiteInterpretedProjectDeclaration(t *testing.T, manager *ast.ASTManager, name string, domain []ast.ASTHandle, boolean ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	symbol, err := manager.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	return declaration
}

func TestFiniteCompilerFusedFilterRemapsSurvivingColumns(t *testing.T) {
	f := newTableRelationFixture(t)
	theory := tableRelationTheoryRewriter(t, f.ast)
	compiler, err := newFiniteCompiler(f.relations, theory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler.closeBlocks() })
	q := finiteInterpretedProjectDeclaration(t, f.ast, "finite-fused-remap-Q", []ast.ASTHandle{f.boolean}, f.boolean)
	y, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	x, err := f.ast.MkVar(1, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	head, err := f.ast.MkApp(q, []ast.ASTHandle{x})
	if err != nil {
		t.Fatal(err)
	}
	yTrue, err := f.ast.MkEq(y, f.truth)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := tableRelationSignature(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	// The rule owns its head and tails; hold them as it would.
	for _, owned := range []ast.ASTHandle{head, yTrue} {
		reference, err := f.ast.NewRef(owned)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.ast.DelRef(&reference) })
	}
	current := compiler.fresh(signature)
	rule := finiteCompiledRule{head: head, interpreted: []ast.ASTHandle{yTrue}}
	variableColumns := map[uint32][]int{0: {0}, 1: {1}}
	binding := &interpretedBinding{}
	t.Cleanup(func() { _ = binding.release(f.ast) })
	prepared, preparedSignature, preparedOwned, preparedPresent, err := compiler.prepareInterpreted(rule, q, current, signature, false, true, variableColumns, binding)
	if err != nil || prepared != current || preparedOwned || !preparedPresent || !preparedSignature.equal(signature) {
		t.Fatal("prepared bound interpreted condition", prepared, preparedOwned, preparedPresent, err)
	}
	result, projected, owned, present, columns, err := compiler.filterInterpreted(
		rule, q, current, signature, false, true, variableColumns, binding,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !owned || !present || projected.Len() != 1 || result == current || len(columns) != 1 || len(columns[1]) != 1 || columns[1][0] != 0 {
		t.Fatal("remapped fused compiler state", result, current, projected.Domains(), owned, present, columns)
	}
	source := NewHashTable(signature)
	for _, fact := range []Fact{{1, 0}, {1, 1}, {0, 0}} {
		if _, err = source.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	if err = context.SetFiniteRegister(current, source); err != nil {
		t.Fatal(err)
	}
	complete, err := compiler.execution.Perform(context)
	if err != nil || !complete {
		t.Fatal("remapped fused execution", complete, err)
	}
	projectedTable, err := context.FiniteRegister(result)
	if err != nil || projectedTable == nil {
		t.Fatal("remapped fused result", projectedTable, err)
	}
	finiteInterpretedProjectWantTable(t, projectedTable, Fact{0}, Fact{1})
}

func TestFiniteCompilerFusesInterpretedFilterAndDeadVariableProjection(t *testing.T) {
	f := newTableRelationFixture(t)
	rules, err := base.NewRuleManager(f.ast)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rules.Close() })
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = set.Close() })
	p := finiteInterpretedProjectDeclaration(t, f.ast, "finite-fused-P", []ast.ASTHandle{f.boolean, f.boolean}, f.boolean)
	q := finiteInterpretedProjectDeclaration(t, f.ast, "finite-fused-Q", []ast.ASTHandle{f.boolean}, f.boolean)
	for _, predicate := range []ast.ASTHandle{p, q} {
		if err = rules.RegisterPredicate(predicate); err != nil {
			t.Fatal(err)
		}
	}
	_, pSignature, err := f.relations.PredicateSignature(p)
	if err != nil {
		t.Fatal(err)
	}
	pTable := NewHashTable(pSignature)
	for _, fact := range []Fact{{0, 0}, {0, 1}, {1, 1}} {
		if _, err = pTable.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err = f.relations.StoreRelation(p, pTable); err != nil {
		t.Fatal(err)
	}
	// De Bruijn variable zero is the rightmost source variable.
	y, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	x, err := f.ast.MkVar(1, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	pXY, err := f.ast.MkApp(p, []ast.ASTHandle{x, y})
	if err != nil {
		t.Fatal(err)
	}
	qX, err := f.ast.MkApp(q, []ast.ASTHandle{x})
	if err != nil {
		t.Fatal(err)
	}
	yTrue, err := f.ast.MkEq(y, f.truth)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := rules.NewRule(qX, []ast.ASTHandle{pXY, yTrue}, nil, "finite-fused-interpreted-project")
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(rule); err != nil {
		t.Fatal(err)
	}
	if err = rule.Close(); err != nil {
		t.Fatal(err)
	}
	valid, err := set.Finalize()
	if err != nil || !valid {
		t.Fatal("finalize", valid, err)
	}
	execution, termination, err := CompileFiniteRulesWithGuard(f.relations, set, tableRelationTheoryRewriter(t, f.ast))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close(); _ = termination.Close() })
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	complete, err := execution.Perform(context)
	if err != nil || !complete {
		t.Fatal("compiled execution", complete, err)
	}
	complete, err = termination.Perform(context)
	if err != nil || !complete {
		t.Fatal("compiled termination", complete, err)
	}
	qTable, found, err := f.relations.TryGetRelation(q)
	if err != nil || !found || qTable == nil {
		t.Fatal("compiled fused relation", qTable, found, err)
	}
	qFacts, err := qTable.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, qFacts, Fact{0}, Fact{1})
	stats, err := context.Statistics()
	if err != nil || stats.FilterInterpretedProject != 1 || stats.Filter != 0 || stats.ProjectRename != 0 {
		t.Fatal("compiled fused counters", stats, err)
	}
}
