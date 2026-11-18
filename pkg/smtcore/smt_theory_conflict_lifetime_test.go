package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTTheoryConflictParametersFollowNestedScopeLifetime(t *testing.T) {
	f := newSMTRealDiffFixture(t)
	m, state := f.manager, f.state
	expression := smtRelevancyBool(t, m, "scoped-theory-conflict")
	astTestMust(t, state.Internalize(expression, true))
	literal, err := state.Literal(expression)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	baseline := m.Stats()
	for round := 0; round < 3; round++ {
		astTestMust(t, state.PushScope())
		parameter, err := bigTestParameter(m, big.NewRat(int64(round+2), 3))
		astTestMust(t, err)
		astTestMust(t, state.SetTheoryConflict(f.family, []SMTLiteral{literal}, parameter))
		// The caller releases its parameter. The scoped justification owns a copy.
		astTestMust(t, m.DelParameter(&parameter))
		conflict, found := state.Conflict()
		if !found {
			t.Fatal("conflict missing")
		}
		object, _ := conflict.Justification()
		owner, ok := object.(*SMTTheoryConflictJustification)
		if !ok || len(owner.Parameters()) != 1 {
			t.Fatal("owned parameter missing")
		}
		expected, err := bigTestParameter(m, big.NewRat(int64(round+2), 3))
		astTestMust(t, err)
		equal, err := m.ParameterEqual(owner.Parameters()[0], expected)
		astTestMust(t, err)
		astTestMust(t, m.DelParameter(&expected))
		if !equal {
			t.Fatal("caller release changed copied coefficient")
		}
		astTestMust(t, state.PushScope())
		// Source keeps the first conflict. A redundant report must neither replace
		// its parameters nor publish another scoped owner.
		count := len(state.ownedJustifications)
		other, err := bigTestParameter(m, big.NewRat(99, 1))
		astTestMust(t, err)
		astTestMust(t, state.SetTheoryConflict(f.family, []SMTLiteral{literal}, other))
		astTestMust(t, m.DelParameter(&other))
		astTestMust(t, state.PopScopes(1))
		current, _ := state.Conflict()
		currentOwner, _ := current.Justification()
		if currentOwner != owner || len(owner.Parameters()) != 1 || len(state.ownedJustifications) != count {
			t.Fatal("nested scope replaced or released outer conflict")
		}
		astTestMust(t, state.PopScopes(1))
		if state.Inconsistent() || len(owner.Parameters()) != 0 || m.Stats().LiveParameters != baseline.LiveParameters {
			t.Fatal("theory parameters survived conflict scope", baseline, m.Stats())
		}
	}
}

func TestSMTTheoryConflictRejectsPartialParameterCopies(t *testing.T) {
	for stop := 0; stop < 4; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newSMTRealDiffFixture(t)
			m, state := f.manager, f.state
			expression := smtRelevancyBool(t, m, "conflict-copy-prefix")
			astTestMust(t, state.Internalize(expression, true))
			literal, err := state.Literal(expression)
			astTestMust(t, err)
			astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
			baseline := m.Stats()
			parameters := []ast.ASTParameter{}
			for _, n := range []int64{2, 5, 11} {
				p, err := bigTestParameter(m, big.NewRat(n, 7))
				astTestMust(t, err)
				parameters = append(parameters, p)
			}
			astTestMust(t, state.PushScope())
			before := m.Stats()
			owners := len(state.ownedJustifications)
			// A stale final handle refuses construction after each possible valid
			// prefix. Copying into an existing AST parameter page need not allocate.
			invalid := append(append([]ast.ASTParameter(nil), parameters[:stop]...), ast.ASTParameter{})
			err = state.SetTheoryConflict(f.family, []SMTLiteral{literal}, invalid...)
			if !errors.Is(err, ast.ErrASTHandle) {
				t.Fatal("accepted invalid parameter", err)
			}
			after := m.Stats()
			if state.Inconsistent() || len(state.ownedJustifications) != owners || after.LiveParameters != before.LiveParameters || after.LiveReferences != before.LiveReferences {
				t.Fatal("failed parameter copy retained partial state", before, after)
			}
			astTestMust(t, state.SetTheoryConflict(f.family, []SMTLiteral{literal}, parameters...))
			for i := range parameters {
				astTestMust(t, m.DelParameter(&parameters[i]))
			}
			conflict, _ := state.Conflict()
			object, _ := conflict.Justification()
			owner := object.(*SMTTheoryConflictJustification)
			if len(owner.Parameters()) != 3 {
				t.Fatal("completed conflict lost copied parameters")
			}
			astTestMust(t, state.PopScopes(1))
			if len(owner.Parameters()) != 0 || m.Stats().LiveParameters != baseline.LiveParameters {
				t.Fatal("parameter copies survived pop")
			}
		})
	}
}
