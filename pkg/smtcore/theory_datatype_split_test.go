package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtDatatypeSplitFixture(t *testing.T) (*ast.ASTManager, *SMTDatatypeTheory, ast.ASTHandle, ast.ASTHandle, []ast.ASTHandle) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_DT", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	s.kernel.smtParams.datatype.LazySplits = 2
	family, err := m.DatatypeFamilyID()
	astTestMust(t, err)
	dt := s.kernel.internalizer.theories[family].(*SMTDatatypeTheory)
	var definitions []ast.DatatypeConstructorDefinition
	for _, name := range []string{"a", "b", "c"} {
		definitions = append(definitions, ast.DatatypeConstructorDefinition{Name: astTestSymbol(t, m, "incremental-"+name), Recognizer: astTestSymbol(t, m, "incremental-is-"+name)})
	}
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: astTestSymbol(t, m, "incremental-sort"), Constructors: definitions}})
	astTestMust(t, err)
	constructors, err := m.DatatypeConstructors(sorts[0])
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "incremental-x"), sorts[0])
	astTestMust(t, err)
	astTestMust(t, s.kernel.internalizer.Internalize(x, false))
	return m, dt, x, sorts[0], constructors
}

func TestSMTDatatypeSplitCreatesOneRecognizerAndAwaitsAssignment(t *testing.T) {
	_, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	clauses := len(s.clauses)
	for i := 0; i < 3; i++ {
		astTestMust(t, dt.split(x, sort))
		slots, err := dt.recognizerSlots(x)
		astTestMust(t, err)
		if len(slots) != 1 || slots[constructors[1]].expression.IsNull() || len(s.clauses) != clauses || dt.statistics.splits != uint32(i+1) {
			t.Fatal("split expanded cases or changed source split-call statistics")
		}
		assignment, err := s.Assignment(slots[constructors[1]].literal)
		astTestMust(t, err)
		if assignment != lbool.Undef {
			t.Fatal("split asserted its chosen constructor")
		}
	}
}

func TestSMTDatatypeSplitActivatesExistingRecognizerWithoutChangingPhase(t *testing.T) {
	_, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	literal, expression, err := dt.recognizerLiteral(constructors[1], x)
	astTestMust(t, err)
	// Exercise the source's cached-but-irrelevant slot branch explicitly.
	// Merely internalizing a recognizer does not watch it with relevancy on.
	astTestMust(t, dt.addRecognizer(s.context.enodes.get(x).TheoryVar(dt.FamilyID()), s.context.enodes.get(expression)))
	if s.isRelevant(expression) {
		t.Fatal("fixture recognizer was already relevant")
	}
	astTestMust(t, dt.split(x, sort))
	data, err := s.BooleanData(literal.Var())
	astTestMust(t, err)
	if !s.isRelevant(expression) || data.TryTrueFirst() {
		t.Fatal("existing recognizer was recreated or its phase changed")
	}
	astTestMust(t, dt.split(x, sort))
	slots, err := dt.recognizerSlots(x)
	astTestMust(t, err)
	if len(slots) != 1 {
		t.Fatal("relevant unassigned recognizer did not stop splitting")
	}
}

func TestSMTDatatypeSplitAdvancesInDeclarationOrderAndBacktracks(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			m, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
			s, err := dt.State()
			astTestMust(t, err)
			astTestMust(t, dt.split(x, sort))
			slots, err := dt.recognizerSlots(x)
			astTestMust(t, err)
			preferred := slots[constructors[1]]
			astTestMust(t, s.PushScope())
			if existing {
				_, expression, err := dt.recognizerLiteral(constructors[0], x)
				astTestMust(t, err)
				astTestMust(t, dt.addRecognizer(s.context.enodes.get(x).TheoryVar(dt.FamilyID()), s.context.enodes.get(expression)))
			}
			notPreferred, err := m.MkNot(preferred.expression)
			astTestMust(t, err)
			astTestMust(t, s.Assert(notPreferred, ast.ASTHandle{}, 0))
			_, err = s.PropagateCore()
			astTestMust(t, err)
			astTestMust(t, dt.split(x, sort))
			slots, err = dt.recognizerSlots(x)
			astTestMust(t, err)
			next := slots[constructors[0]]
			if len(slots) != 2 || next.expression.IsNull() || !s.isRelevant(next.expression) {
				t.Fatal("false preference did not select first declaration slot")
			}
			data, err := s.BooleanData(next.literal.Var())
			astTestMust(t, err)
			if data.TryTrueFirst() == existing {
				t.Fatal("phase policy differs for new versus existing recognizers")
			}
			astTestMust(t, dt.split(x, sort))
			slots, err = dt.recognizerSlots(x)
			astTestMust(t, err)
			if len(slots) != 2 {
				t.Fatal("unassigned next slot was bypassed")
			}
			astTestMust(t, s.PopScopes(1))
			astTestMust(t, dt.split(x, sort))
			slots, err = dt.recognizerSlots(x)
			astTestMust(t, err)
			if len(slots) != 1 || slots[constructors[1]].expression != preferred.expression {
				t.Fatal("split preference did not survive scope rollback")
			}
		})
	}
}
