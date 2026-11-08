package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTInternalizerNegatedReservedTruthLiterals(t *testing.T) {
	for _, truth := range []bool{false, true} {
		t.Run(fmt.Sprint(truth), func(t *testing.T) {
			m := astTestManager(t)
			_, _, state := smtInternalizerFixture(t, m, nil)
			constant, err := m.MkFalse()
			if truth {
				constant, err = m.MkTrue()
			}
			astTestMust(t, err)
			negated, err := m.MkNot(constant)
			astTestMust(t, err)
			ref, err := m.NewRef(negated)
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&ref)) }()
			astTestMust(t, state.Internalize(negated, true))
			literal, err := state.Literal(negated)
			astTestMust(t, err)
			want := SMTTrueLiteral
			if truth {
				want = SMTFalseLiteral
			}
			if literal != want {
				t.Fatal("reserved negation literal", literal, want)
			}
			astTestMust(t, state.Assert(negated, ast.ASTHandle{}, 0))
			if state.Inconsistent() != truth {
				t.Fatal("reserved assertion conflict", state.Inconsistent(), truth)
			}
		})
	}
}

func TestSMTInternalizerBooleanVariablesLiteralsAssignmentsAndScopes(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyBool(t, m, "internalizer-state-a")
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	variable, err := state.MkBoolVar(a)
	astTestMust(t, err)
	if variable != SMTFirstBoolVar {
		t.Fatal("first source Boolean variable", variable)
	}
	astTestMust(t, state.PushScope())
	positive, err := state.Literal(a)
	astTestMust(t, err)
	negation, err := m.MkNot(a)
	astTestMust(t, err)
	negative, err := state.Literal(negation)
	astTestMust(t, err)
	if positive != NewSMTLiteral(variable, false) || negative != positive.Negated() {
		t.Fatal("literal mapping", positive, negative)
	}
	astTestMust(t, state.SetAssignment(variable, lbool.True))
	if value, err := state.Assignment(positive); err != nil || value != lbool.True {
		t.Fatal("positive assignment", value, err)
	}
	if value, err := state.Assignment(negative); err != nil || value != lbool.False {
		t.Fatal("negative assignment", value, err)
	}
	watches, err := state.WatchList(negative)
	astTestMust(t, err)
	astTestMust(t, watches.InsertLiteral(positive))
	data, err := state.BooleanData(variable)
	astTestMust(t, err)
	data.SetEquality()
	if !data.IsAtom() || !data.IsEquality() || watches.Size() != 1 {
		t.Fatal("Boolean data or watch allocation")
	}
	astTestMust(t, state.PopScopes(1))
	if _, found := state.BoolVar(a); !found {
		t.Fatal("preexisting Boolean variable was removed")
	}
	if value, err := state.Assignment(positive); err != nil || value != lbool.Undef {
		t.Fatal("assignment scope rollback", value, err)
	}
	stats := state.Statistics()
	// context::init counts the true variable as well.
	if stats.NumMkBoolVar != 2 || stats.NumDelBoolVar != 0 {
		t.Fatal("Boolean variable statistics", stats)
	}
}

func TestSMTInternalizerEnodeGenerationCongruenceAndRollback(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "InternalizerStateSort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, _ := smtEnodeConstant(t, m, 2230, sort, 0)
	b, _ := smtEnodeConstant(t, m, 2231, sort, 0)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	astTestMust(t, state.SetGeneration(3))
	astTestMust(t, state.SetCachedGeneration(b, 7))
	an, err := state.MkEnode(a, false, false, true)
	astTestMust(t, err)
	bn, err := state.MkEnode(b, false, false, true)
	astTestMust(t, err)
	if an.Generation() != 3 || bn.Generation() != 7 || state.Statistics().MaxGeneration != 3 {
		t.Fatal("current and cached generations", an.Generation(), bn.Generation(), state.Statistics())
	}
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	f, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "internalizer-state-f"), []ast.ASTHandle{sort}, sort, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	fa, err := m.MkApp(f, []ast.ASTHandle{a})
	astTestMust(t, err)
	fb, err := m.MkApp(f, []ast.ASTHandle{b})
	astTestMust(t, err)
	bn.root = an
	astTestMust(t, state.PushScope())
	fan, err := state.MkEnode(fa, false, false, true)
	astTestMust(t, err)
	_, err = state.MkEnode(fb, false, false, true)
	astTestMust(t, err)
	pending := state.PendingCongruences()
	if len(pending) != 1 || pending[0].Existing != fan || pending[0].Node.owner != fb {
		t.Fatal("pending congruence", pending)
	}
	astTestMust(t, state.PopScopes(1))
	if context.enodes.get(fa) != nil || context.enodes.get(fb) != nil || len(state.PendingCongruences()) != 0 {
		t.Fatal("enode scope rollback")
	}
	bn.root = bn
	stats := state.Statistics()
	// context::init's true and false enodes count in m_num_mk_enode.
	if stats.NumMkEnode != 4+smtInitEnodes || stats.NumDelEnode != 2 {
		t.Fatal("enode statistics", stats)
	}
}
