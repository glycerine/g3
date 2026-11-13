package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelCheckerContextRecoveryUsesOldestEqualityMember(t *testing.T) {
	m, state, registry, _ := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	value, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	integer, err := m.GetSort(value)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "oldest-context-p", integer, boolean)
	body, err := m.MkApp(p, []ast.ASTHandle{x})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "oldest-context-x")}, Body: body})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(q, true))
	nodes := make([]*SMTEnode, 2)
	for i, generation := range []uint32{1, 9} {
		name := "old-context-term"
		if i == 1 {
			name = "new-context-term"
		}
		term, err := m.MkConst(astTestSymbol(t, m, name), integer)
		astTestMust(t, err)
		astTestMust(t, state.SetGeneration(generation))
		astTestMust(t, state.Internalize(term, false))
		nodes[i] = state.context.enodes.get(term)
	}
	astTestMust(t, state.addEquality(nodes[0], nodes[1], NewSMTEqualityAxiom()))
	root := nodes[0].Root()
	if root != nodes[1] || root.Generation() != 9 || root.EqualityClassMinGeneration() != nodes[0] {
		t.Fatal("fixture did not create a newer class root")
	}
	checker, err := NewSMTModelChecker(state, 1)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, checker.Close()) })
	astTestMust(t, checker.SetCandidateTerms(map[*SMTEnode]ast.ASTHandle{root: value}, nil))
	inserted, err := checker.addCounterexample(q, []ast.ASTHandle{value})
	astTestMust(t, err)
	if !checker.HasNewInstances() || checker.newInstances[0].bindings[0] != nodes[0].owner || registry.schedule.HasWork() {
		t.Fatal("context recovery did not pin the oldest term before restart")
	}
	astTestMust(t, checker.Restart())
	if !inserted || len(registry.schedule.newEntries) != 1 {
		t.Fatal("recovered context counterexample was not queued")
	}
	entry := registry.schedule.newEntries[0]
	if checker.valueTerms[value] != nodes[0].owner || entry.Generation != 0 || entry.Fingerprint.args[0] != root {
		t.Fatal("context recovery used the newer representative instead of the oldest class member", entry.Generation)
	}
}

func TestSMTModelCheckerInversePreservesGenerationBeforeFingerprintCanonicalization(t *testing.T) {
	for _, fromInverse := range []bool{false, true} {
		name := "unrestricted-context-lookup"
		if fromInverse {
			name = "recovered-inverse-term"
		}
		t.Run(name, func(t *testing.T) {
			m, state, registry, _ := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
			seven, err := m.ArithmeticIntegerNumeralText("7")
			astTestMust(t, err)
			three, err := m.ArithmeticIntegerNumeralText("3")
			astTestMust(t, err)
			integer, err := m.GetSort(seven)
			astTestMust(t, err)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			predicate := smtQuickPredicate(t, m, "inverse-generation-p", integer, boolean)
			x, err := m.MkVar(0, integer)
			astTestMust(t, err)
			body, err := m.MkApp(predicate, []ast.ASTHandle{x})
			astTestMust(t, err)
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "inverse-generation-x")}, Body: body})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(q, true))
			original, err := NewSMTEnode(m, seven, state.context.enodes.get, SMTEnodeOptions{Generation: 8})
			astTestMust(t, err)
			astTestMust(t, state.context.RegisterEnode(original, true))
			representative, err := NewSMTEnode(m, three, state.context.enodes.get, SMTEnodeOptions{Generation: 1})
			astTestMust(t, err)
			astTestMust(t, state.context.RegisterEnode(representative, true))
			original.root = representative
			checker, err := NewSMTModelChecker(state, 1)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, checker.Close()) })
			checker.valueTerms = map[ast.ASTHandle]ast.ASTHandle{seven: three}
			generation := uint32(0)
			if fromInverse {
				generation = 8
			}
			inserted, err := checker.enqueueCounterexample(q, []ast.ASTHandle{seven}, fromInverse, generation)
			astTestMust(t, err)
			astTestMust(t, checker.Restart())
			if !inserted || len(registry.schedule.newEntries) != 1 {
				t.Fatal("counterexample was not queued")
			}
			entry := registry.schedule.newEntries[0]
			want := uint32(0)
			if fromInverse {
				want = 8
			}
			if entry.Generation != want {
				t.Fatal("inverse generation replaced before fingerprint construction", entry.Generation, want)
			}
			if len(entry.Fingerprint.args) != 1 || entry.Fingerprint.args[0] != representative {
				t.Fatal("fingerprint canonicalization changed")
			}
		})
	}
}

func TestSMTModelCheckerRejectsUnrecoverablePrivateBindings(t *testing.T) {
	m, state, registry, _ := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "private-binding-sort"), nil)
	astTestMust(t, err)
	private, err := m.MkModelValue(0, sort)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	_, err = m.RegisterArrayPlugin()
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "private-binding-function"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	array, err := m.MkAsArray(function)
	astTestMust(t, err)
	checker, err := NewSMTModelChecker(state, 1)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, checker.Close()) })
	for _, value := range []ast.ASTHandle{private, array} {
		sort, err := m.GetSort(value)
		astTestMust(t, err)
		x, err := m.MkVar(0, sort)
		astTestMust(t, err)
		p := smtQuickPredicate(t, m, "private-binding-p", sort, boolean)
		body, err := m.MkApp(p, []ast.ASTHandle{x})
		astTestMust(t, err)
		q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "private-binding-x")}, Body: body})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(q, true))
		before := state.context.enodes.count()
		inserted, err := checker.addCounterexample(q, []ast.ASTHandle{value})
		astTestMust(t, err)
		if inserted || !checker.Exhausted() || state.context.enodes.count() != before || registry.schedule.HasWork() {
			t.Fatal("private model binding entered logical context", value)
		}
	}
}

func TestSMTModelCheckerFiniteUniverseRecoversModelFinderTerm(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	manager := state.quantifierManager
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "finite-inverse-sort"), nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "finite-inverse-p", sort, boolean)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(p, []ast.ASTHandle{x})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "finite-inverse-x")}, Body: body})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(q, true))
	astTestMust(t, state.context.SetRelevant(q, true))
	astTestMust(t, state.context.SetAssignment(q, lbool.True))
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	astTestMust(t, manager.modelFinder.FixModel(proto))
	checker := manager.modelChecker
	// There are no pre-existing ground representatives in the model
	// generator's map; recovery must use the freshly built AUF inverse.
	checker.valueTerms = nil
	satisfied, err := checker.Check(proto, []ast.ASTHandle{q})
	astTestMust(t, err)
	astTestMust(t, checker.Restart())
	if satisfied || checker.Exhausted() || len(state.quantifiers.schedule.newEntries) != 1 {
		t.Fatal("finite private value was not recovered through its AUF inverse", satisfied, checker.Exhausted())
	}
	args := state.quantifiers.schedule.newEntries[0].Fingerprint.args
	if len(args) != 1 {
		t.Fatal("finite inverse counterexample arity")
	}
	private, err := m.IsModelValue(args[0].owner)
	astTestMust(t, err)
	if private {
		t.Fatal("private universe value escaped instead of the AUF context term")
	}
}

func TestSMTModelCheckerMixedUniverseRecoversFiniteInverse(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	state := solver.kernel.internalizer
	manager := state.quantifierManager
	u, err := m.MkUninterpretedSort(astTestSymbol(t, m, "mixed-inverse-sort"), nil)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkFuncDecl(astTestSymbol(t, m, "mixed-inverse-p"), []ast.ASTHandle{u, integer}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	x, err := m.MkVar(1, u)
	astTestMust(t, err)
	y, err := m.MkVar(0, integer)
	astTestMust(t, err)
	body, err := m.MkApp(p, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{u, integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "mixed-inverse-x"), astTestSymbol(t, m, "mixed-inverse-y")}, Body: body})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(q, true))
	astTestMust(t, state.context.SetRelevant(q, true))
	astTestMust(t, state.context.SetAssignment(q, lbool.True))
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	factory, err := protomodel.NewArithmeticFactory(m)
	astTestMust(t, err)
	astTestMust(t, proto.RegisterFactory(factory))
	astTestMust(t, manager.modelFinder.FixModel(proto))
	checker := manager.modelChecker
	checker.valueTerms = nil
	satisfied, err := checker.Check(proto, []ast.ASTHandle{q})
	astTestMust(t, err)
	astTestMust(t, checker.Restart())
	if satisfied || checker.Exhausted() || len(state.quantifiers.schedule.newEntries) != 1 {
		t.Fatal("mixed finite/infinite counterexample did not recover the finite inverse", satisfied, checker.Exhausted(), len(state.quantifiers.schedule.newEntries))
	}
	for _, node := range state.quantifiers.schedule.newEntries[0].Fingerprint.args {
		private, err := containsPrivateModelTerm(m, node.owner, true)
		astTestMust(t, err)
		if private {
			t.Fatal("mixed-domain model value escaped into the context")
		}
	}
}
