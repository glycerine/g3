package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTDatatypeWatchRegistrationAndScopedSlots(t *testing.T) {
	m, dt, x, _, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	xNode := s.context.enodes.get(x)
	_, data, err := dt.rootData(xNode.TheoryVar(dt.FamilyID()))
	astTestMust(t, err)
	literal, expression, err := dt.recognizerLiteral(constructors[1], x)
	astTestMust(t, err)
	if len(data.recognizers) != 0 {
		t.Fatal("irrelevant recognizer was watched")
	}
	astTestMust(t, s.PushScope())
	negative, err := m.MkNot(expression)
	astTestMust(t, err)
	astTestMust(t, s.Assert(negative, ast.ASTHandle{}, 0))
	_, err = s.PropagateCore()
	astTestMust(t, err)
	if len(data.recognizers) != 3 || data.recognizers[1] != s.context.enodes.get(expression) {
		t.Fatal("relevancy did not register the watched slot before false assignment")
	}
	if assignment, err := s.Assignment(literal); err != nil || assignment != lbool.False {
		t.Fatal("false recognizer assignment")
	}
	astTestMust(t, s.PopScopes(1))
	if len(data.recognizers) != 3 {
		t.Fatal("source recognizer vector resize was incorrectly trailed")
	}
	for _, node := range data.recognizers {
		if node != nil {
			t.Fatal("scoped watch survived pop")
		}
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, s.Assert(expression, ast.ASTHandle{}, 0))
	_, err = s.PropagateCore()
	astTestMust(t, err)
	_, data, err = dt.rootData(xNode.TheoryVar(dt.FamilyID()))
	astTestMust(t, err)
	if data.constructor == nil || data.constructor.declaration != constructors[1] || data.recognizers[1] != nil {
		t.Fatal("positive registration retained a watch or lost its constructor")
	}
	astTestMust(t, s.PopScopes(1))
}

func TestSMTDatatypeMergedFalseRecognizersPropagateWithSourceEvidence(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			m, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
			s, err := dt.State()
			astTestMust(t, err)
			alias, err := m.MkConst(astTestSymbol(t, m, "metadata-alias"), sort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "metadata-y"), sort)
			astTestMust(t, err)
			for _, h := range []ast.ASTHandle{alias, y} {
				astTestPin(t, m, h)
				astTestMust(t, s.Internalize(h, false))
			}
			astTestMust(t, s.addEquality(s.context.enodes.get(x), s.context.enodes.get(alias), NewSMTEqualityAxiom()))
			astTestMust(t, s.PropagateTheoryEvents())
			root, rootData, err := dt.rootData(s.context.enodes.get(x).TheoryVar(dt.FamilyID()))
			astTestMust(t, err)
			rootNode := dt.variables[root]
			first, firstExpr, err := dt.recognizerLiteral(constructors[0], rootNode.owner)
			astTestMust(t, err)
			second, secondExpr, err := dt.recognizerLiteral(constructors[1], y)
			astTestMust(t, err)
			var survivor SMTLiteral
			var survivorExpr ast.ASTHandle
			if existing {
				survivor, survivorExpr, err = dt.recognizerLiteral(constructors[2], rootNode.owner)
				astTestMust(t, err)
				astTestMust(t, s.markRelevant(survivorExpr))
			}
			for _, expression := range []ast.ASTHandle{firstExpr, secondExpr} {
				negative, err := m.MkNot(expression)
				astTestMust(t, err)
				astTestMust(t, s.Assert(negative, ast.ASTHandle{}, 0))
			}
			_, err = s.PropagateCore()
			astTestMust(t, err)
			if rootData.constructor != nil {
				t.Fatal("independent false recognizer classes reconstructed a constructor")
			}
			astTestMust(t, s.PushScope())
			clauses, bools := len(s.clauses), len(s.boolExpr)
			astTestMust(t, s.addEquality(rootNode, s.context.enodes.get(y), NewSMTEqualityAxiom()))
			astTestMust(t, s.PropagateTheoryEvents())
			if !existing {
				survivor, survivorExpr, err = dt.recognizerLiteral(constructors[2], rootNode.owner)
				astTestMust(t, err)
			}
			if assignment, err := s.Assignment(survivor); err != nil || assignment != lbool.True {
				t.Fatal("missing survivor propagation")
			}
			info, err := s.BooleanData(survivor.Var())
			astTestMust(t, err)
			owner, _ := info.Justification().Justification()
			extended, ok := owner.(*SMTExtTheoryPropagationJustification)
			if !ok || len(extended.Literals) != 2 || extended.Literals[0] != first.Negated() || extended.Literals[1] != second.Negated() || len(extended.Equalities) != 1 || extended.Consequent != survivor {
				t.Fatal("survivor lost ordered literal/equality evidence", owner)
			}
			pair := extended.Equalities[0]
			if pair.First != rootNode || pair.Second != s.context.enodes.get(y) || pair.First.root != pair.Second.root {
				t.Fatal("survivor used an invented equality")
			}
			wantNew := 0
			if !existing {
				wantNew = 1
			}
			if len(s.clauses) != clauses || len(s.boolExpr)-bools != wantNew || info.TryTrueFirst() || !s.isRelevant(survivorExpr) {
				t.Fatal("survivor used SAT guards/split phases or lost relevance")
			}
			_, err = s.PropagateCore()
			astTestMust(t, err)
			_, merged, err := dt.rootData(rootNode.TheoryVar(dt.FamilyID()))
			astTestMust(t, err)
			if merged.constructor == nil || merged.constructor.declaration != constructors[2] {
				t.Fatal("positive survivor did not reconstruct its constructor")
			}
			astTestMust(t, s.PopScopes(1))
			_, restored, err := dt.rootData(rootNode.TheoryVar(dt.FamilyID()))
			astTestMust(t, err)
			if restored.constructor != nil || restored.recognizers[1] != nil {
				t.Fatal("merged constructor/watch escaped scope rollback")
			}
		})
	}
}

func TestSMTDatatypeConstructorMetadataAndModelProcedureRollback(t *testing.T) {
	f := newSMTDatatypeConflictFixture(t, false)
	s := f.k.internalizer
	g, err := NewSMTModelGenerator(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	astTestMust(t, g.SetContext(s.context))
	astTestMust(t, g.InitializeModel(f.k.modelConfig))
	astTestMust(t, f.dt.InitializeModel(g))
	if _, err = f.dt.ModelValue(s.context.enodes.get(f.x), g); err == nil {
		t.Fatal("constructor-free theory variable received an arbitrary fallback model value")
	}
	ctor, _, err := f.m.DatatypeConstructorApp(f.left)
	astTestMust(t, err)
	element, err := f.m.GetArg(f.left, 0)
	astTestMust(t, err)
	elementSort, err := f.m.GetSort(element)
	astTestMust(t, err)
	other, err := f.m.MkConst(astTestSymbol(t, f.m, "metadata-other-element"), elementSort)
	astTestMust(t, err)
	second, err := f.m.MkApp(ctor, []ast.ASTHandle{other})
	astTestMust(t, err)
	astTestPin(t, f.m, second)
	astTestMust(t, s.Internalize(second, false))
	firstNode, secondNode := s.context.enodes.get(f.left), s.context.enodes.get(second)
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, s.PushScope())
		astTestMust(t, s.addEquality(firstNode, secondNode, NewSMTEqualityAxiom()))
		events := s.PendingTheoryEqualities()
		if len(events) != 1 {
			t.Fatal("constructor merge fixture has unexpected pending events", events)
		}
		event := events[0]
		firstRoot, _, err := f.dt.rootData(event.Left)
		astTestMust(t, err)
		secondRoot, _, err := f.dt.rootData(event.Right)
		astTestMust(t, err)
		winner := secondRoot
		if f.dt.find.Size(firstRoot) > f.dt.find.Size(secondRoot) {
			winner = firstRoot
		}
		expected := f.dt.data[winner].constructor
		astTestMust(t, s.PropagateTheoryEvents())
		_, data, err := f.dt.rootData(firstNode.TheoryVar(f.dt.FamilyID()))
		astTestMust(t, err)
		if data.constructor != expected {
			t.Fatal("merge replaced the winning root's constructor")
		}
		procedure, err := f.dt.ModelValue(firstNode.root, g)
		astTestMust(t, err)
		dependencies := procedure.Dependencies()
		if len(dependencies) != 1 {
			t.Fatal("wrong constructor dependency count")
		}
		dependency, err := dependencies[0].Enode()
		astTestMust(t, err)
		if dependency != expected.arguments[0].root {
			t.Fatal("model procedure did not use its stored constructor arguments")
		}
		value, err := procedure.MakeValue(g, []ast.ASTHandle{element})
		astTestMust(t, err)
		matchedCtor, matched, err := f.m.DatatypeConstructorApp(value)
		astTestMust(t, err)
		if !matched || matchedCtor != ctor || procedure.IsFresh() {
			t.Fatal("invalid datatype model procedure")
		}
		astTestMust(t, s.PopScopes(1))
		for _, node := range []*SMTEnode{firstNode, secondNode} {
			_, own, err := f.dt.rootData(node.TheoryVar(f.dt.FamilyID()))
			astTestMust(t, err)
			if own.constructor != node {
				t.Fatal("constructor metadata did not restore on pop")
			}
		}
	}
}

func TestSMTDatatypeMergePreservesFirstWatchPerConstructor(t *testing.T) {
	m, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "metadata-watch-y"), sort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	astTestMust(t, s.Internalize(y, false))
	var expressions []ast.ASTHandle
	for _, value := range []ast.ASTHandle{x, y} {
		_, expression, err := dt.recognizerLiteral(constructors[1], value)
		astTestMust(t, err)
		expressions = append(expressions, expression)
		astTestMust(t, s.markRelevant(expression))
	}
	_, err = s.PropagateCore()
	astTestMust(t, err)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(s.context.enodes.get(x), s.context.enodes.get(y), NewSMTEqualityAxiom()))
	events := s.PendingTheoryEqualities()
	if len(events) != 1 {
		t.Fatal("unexpected watch merge events", events)
	}
	secondRoot, _, err := dt.rootData(events[0].Right)
	astTestMust(t, err)
	expected := dt.data[secondRoot].recognizers[1]
	astTestMust(t, s.PropagateTheoryEvents())
	_, merged, err := dt.rootData(s.context.enodes.get(x).TheoryVar(dt.FamilyID()))
	astTestMust(t, err)
	if merged.recognizers[1] != expected {
		t.Fatal("merge overwrote the winning root's watched slot")
	}
	astTestMust(t, s.PopScopes(1))
	for i, value := range []ast.ASTHandle{x, y} {
		_, restored, err := dt.rootData(s.context.enodes.get(value).TheoryVar(dt.FamilyID()))
		astTestMust(t, err)
		if restored.recognizers[1] != s.context.enodes.get(expressions[i]) {
			t.Fatal("watch metadata did not restore on pop")
		}
	}
}

func TestSMTDatatypeRelevanceBeforePushRegistersAtBaseScope(t *testing.T) {
	m, dt, x, _, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	literal, expression, err := dt.recognizerLiteral(constructors[1], x)
	astTestMust(t, err)
	astTestMust(t, s.markRelevant(expression))
	_, data, err := dt.rootData(s.context.enodes.get(x).TheoryVar(dt.FamilyID()))
	astTestMust(t, err)
	watched := s.context.enodes.get(expression)
	if len(data.recognizers) != 3 || data.recognizers[1] != watched {
		t.Fatal("source relevance callback was deferred past its scope")
	}
	astTestMust(t, s.PushScope())
	_, err = s.PropagateCore()
	astTestMust(t, err)
	astTestMust(t, s.PopScopes(1))
	if data.recognizers[1] != watched {
		t.Fatal("base-level watch was trailed at the later propagation scope")
	}
	astTestMust(t, s.PushScope())
	negative, err := m.MkNot(expression)
	astTestMust(t, err)
	astTestMust(t, s.Assert(negative, ast.ASTHandle{}, 0))
	_, err = s.PropagateCore()
	astTestMust(t, err)
	if value, err := s.Assignment(literal); err != nil || value != lbool.False {
		t.Fatal("surviving base watch failed to accept its later assignment")
	}
	astTestMust(t, s.PopScopes(1))
}
