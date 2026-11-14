package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTLearnedPersistenceNativeReleaseRetries(t *testing.T) {
	for _, stage := range []string{"preserved", "unit", "all", "above", "end", "abandoned"} {
		t.Run(stage, func(t *testing.T) {
			f := newSMTNativeCleanupFixture(t)
			m, state := f.manager, f.state
			expression := smtRelevancyBool(t, m, "learned-release-frontier")
			astTestMust(t, state.Internalize(expression, true))
			literal, err := state.Literal(expression)
			astTestMust(t, err)
			astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			proof, err := m.MkProofAsserted(expression)
			astTestMust(t, err)
			astTestMust(t, m.IncRef(proof))
			astTestMust(t, m.IncRef(expression))
			unit := smtPersistentLearnedUnit{expression: expression, proof: proof, ownerLevel: 1, installedLevel: 1}
			var saved []smtPreservedLearnedClause
			if stage == "preserved" {
				saved = []smtPreservedLearnedClause{{proof: proof, atoms: []smtRetainedLemmaAtom{{expression: expression, sign: literal.Sign()}}}}
			} else if stage != "unit" {
				state.learnedUnits = []smtPersistentLearnedUnit{unit}
			}
			release := func() error {
				switch stage {
				case "preserved":
					return state.releasePreservedLearnedClauses(saved)
				case "unit":
					return state.releaseLearnedUnit(&unit)
				case "all":
					return state.releaseLearnedUnits()
				case "above":
					return state.discardLearnedUnitsAbove(0)
				case "end":
					return state.endSearchPersistence()
				default:
					return state.discardLearnedUnitsFrom(0)
				}
			}
			beforeProof, err := m.GetRefCount(proof)
			astTestMust(t, err)
			var failure error
			f.refuse(t, func() { failure = release() })
			if !errors.Is(failure, ast.ErrASTPending) {
				t.Fatal("release did not refuse", failure)
			}
			afterProof, err := m.GetRefCount(proof)
			astTestMust(t, err)
			if afterProof != beforeProof {
				t.Fatal("refusal consumed proof owner", beforeProof, afterProof)
			}
			if stage == "preserved" {
				if saved[0].proof != proof || saved[0].atoms[0].expression != expression {
					t.Fatal("refusal dropped preserved clause")
				}
			} else if stage == "unit" {
				if unit.proof != proof || unit.expression != expression {
					t.Fatal("refusal zeroed unit")
				}
			} else if len(state.learnedUnits) != 1 || state.learnedUnits[0].proof != proof || state.learnedUnits[0].expression != expression {
				t.Fatal("refusal truncated learned units")
			}
			astTestMust(t, release())
			astTestMust(t, release())
			if stage == "preserved" && (!saved[0].proof.IsNull() || !saved[0].atoms[0].expression.IsNull()) {
				t.Fatal("retry left clause roots")
			}
			if stage == "unit" && (!unit.proof.IsNull() || !unit.expression.IsNull()) {
				t.Fatal("retry left unit roots")
			}
			if stage != "preserved" && stage != "unit" && len(state.learnedUnits) != 0 {
				t.Fatal("retry left unit records")
			}
		})
	}
}

func TestSMTLearnedClauseRestoreFailureResumesWithoutDuplicates(t *testing.T) {
	f := newSMTNativeCleanupFixture(t)
	m, state := f.manager, f.state
	relevancy, err := NewSMTRelevancyPropagator(state.context, true)
	astTestMust(t, err)
	astTestMust(t, state.SetRelevancy(relevancy))
	smtTestUserPush(t, state)
	var expressions []ast.ASTHandle
	var literals []SMTLiteral
	for _, name := range []string{"restore-first", "restore-second", "restore-third"} {
		expression := smtRelevancyBool(t, m, name)
		expressions = append(expressions, expression)
		astTestMust(t, state.Internalize(expression, true))
		literal, err := state.Literal(expression)
		astTestMust(t, err)
		literals = append(literals, literal)
	}
	astTestMust(t, state.AddLearnedClause([]SMTLiteral{literals[0], literals[1]}))
	astTestMust(t, state.AddLearnedClause([]SMTLiteral{literals[1].Negated(), literals[2]}))
	for i := range state.clauses {
		astTestMust(t, state.clauses[i].clause.SetActivity(uint32(7+i)))
	}
	saved, err := state.preserveLearnedClauses(0)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.releasePreservedLearnedClauses(saved)) })
	astTestMust(t, state.PopScopes(1))
	// Reach the boundary after the first real installation, then refuse
	// the next row through the native manager rather than a fake hook.
	astTestMust(t, state.restoreLearnedClauses(saved[:1]))
	var cause error
	f.refuse(t, func() { cause = state.restoreLearnedClauses(saved) })
	if !errors.Is(cause, ast.ErrASTPending) || !saved[0].installed || saved[1].installed || len(state.clauses) != 1 || saved[1].atoms[0].expression.IsNull() {
		t.Fatal("refusal lost partial installation or pending roots", cause, len(state.clauses))
	}
	astTestMust(t, state.restoreLearnedClauses(saved))
	astTestMust(t, state.restoreLearnedClauses(saved))
	if len(state.clauses) != 2 {
		t.Fatal("retry duplicated or lost clauses", len(state.clauses))
	}
	for i, clause := range state.clauses {
		activity, err := clause.clause.Activity()
		astTestMust(t, err)
		if activity != uint32(7+i) {
			t.Fatal("retry lost activity", i, activity)
		}
		for _, atom := range saved[i].atoms {
			if !atom.expression.IsNull() {
				t.Fatal("completed restoration kept root")
			}
		}
	}
	first, err := state.Literal(expressions[0])
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(first.Negated(), NewSMTBooleanAxiom()))
	_, err = state.Propagate()
	astTestMust(t, err)
	for _, expression := range expressions[1:] {
		literal, err := state.Literal(expression)
		astTestMust(t, err)
		value, err := state.Assignment(literal)
		astTestMust(t, err)
		if value != lbool.True {
			t.Fatal("restored clauses lost propagation", value)
		}
	}
}

func TestSMTLearnedAssignmentResumesPublishedScalarBeforeContext(t *testing.T) {
	if !ast.ASTValidateHandles {
		t.Skip("read accessors do not refuse re-entrant calls in normal builds (ast_validate_handles_off.go)")
	}
	f := newSMTNativeCleanupFixture(t)
	state, m := f.state, f.manager
	expression := smtRelevancyBool(t, m, "learned-assignment-continuation")
	astTestMust(t, state.Internalize(expression, true))
	literal, err := state.Literal(expression)
	astTestMust(t, err)
	before, assigned := state.assignment[literal.Index()], len(state.assignedLiterals)
	justification := NewSMTBooleanAxiom()
	var cause error
	f.refuse(t, func() { cause = state.AssignLiteral(literal, justification) })
	if !errors.Is(cause, ast.ErrASTPending) || state.assignment[literal.Index()] != lbool.True || len(state.assignedLiterals) != assigned {
		t.Fatal("test did not reach partial scalar publication", cause, len(state.assignedLiterals))
	}
	pending := state.learnedAssignmentRemainder(literal, before, assigned, justification)
	if pending.stage != smtLearnedAssignmentContext {
		t.Fatal("missing context continuation", pending.stage)
	}
	f.refuse(t, func() { cause = state.finishLearnedAssignment(&pending) })
	if !errors.Is(cause, ast.ErrASTPending) || pending.stage != smtLearnedAssignmentContext {
		t.Fatal("refusal consumed unfinished assignment", cause, pending.stage)
	}
	astTestMust(t, state.finishLearnedAssignment(&pending))
	astTestMust(t, state.finishLearnedAssignment(&pending))
	if pending.stage != smtLearnedAssignmentDone || len(state.assignedLiterals) != assigned+1 || state.context.assignments.get(expression) != lbool.True || state.booleanData[literal.Var()].Justification() != justification {
		t.Fatal("assignment continuation lost metadata or duplicated publication", pending.stage, len(state.assignedLiterals))
	}
}
