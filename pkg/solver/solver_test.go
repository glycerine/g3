package solver

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type solverFixture struct {
	*SimpleCheckSatResult
	manager    *ast.ASTManager
	assertions []ast.ASTHandle
	scopes     []int
	statuses   []lbool.LBool
	checks     int
}

func TestSolverDefaultConsequenceForNonBooleanValue(t *testing.T) {
	m := solverTestManager(t)
	result, err := NewSimpleCheckSatResult(m)
	solverMust(t, err)
	s := &solverFixture{SimpleCheckSatResult: result, manager: m, statuses: []lbool.LBool{lbool.True, lbool.False}}
	t.Cleanup(func() { solverMust(t, result.Close()) })
	logic, err := m.Symbols.Intern("LIA")
	solverMust(t, err)
	_, err = m.RegisterArithmeticPlugin()
	solverMust(t, err)
	var names ast.ASTBuiltinNames
	defer names.Close()
	solverMust(t, m.GetArithmeticSortNames(&names, logic))
	value, err := m.ArithmeticNumeralInt64(int64(7), true)
	solverMust(t, err)
	integer, err := m.GetSort(value)
	solverMust(t, err)
	name, err := m.Symbols.Intern("solver-int")
	solverMust(t, err)
	variable, err := m.MkConst(name, integer)
	solverMust(t, err)
	md, err := model.NewModel(m)
	solverMust(t, err)
	solverMust(t, result.SetModel(md.Core()))
	status, consequences, err := DefaultGetConsequences(s, nil, []ast.ASTHandle{variable}, func(*model.ModelCore) (ConsequenceEvaluator, error) {
		return &fixedConsequenceEvaluator{manager: m, value: value}, nil
	})
	solverMust(t, err)
	if status != lbool.True || len(consequences) != 1 || len(s.scopes) != 0 || len(s.assertions) != 0 {
		t.Fatal("non-Boolean consequence", status, len(consequences), len(s.scopes), len(s.assertions))
	}
	if _, err = m.GetSort(consequences[0]); err != nil {
		t.Fatal("non-Boolean consequence lifetime", err)
	}
}

func (s *solverFixture) Translate(*ast.ASTManager, *uparams.ParamsRef) (Solver, error) {
	return s, nil
}
func (s *solverFixture) UpdateParams(*uparams.ParamsRef) error         { return nil }
func (s *solverFixture) Params() (*uparams.ParamsRef, error)           { return nil, nil }
func (s *solverFixture) CollectParamDescrs(*uparams.ParamDescrs) error { return nil }
func (s *solverFixture) SetProduceModels(bool) error                   { return nil }
func (s *solverFixture) Assert(h ast.ASTHandle) error {
	s.assertions = append(s.assertions, h)
	return nil
}
func (s *solverFixture) AssertTracked(h, _ ast.ASTHandle) error { return s.Assert(h) }
func (s *solverFixture) Push() error {
	s.scopes = append(s.scopes, len(s.assertions))
	return nil
}
func (s *solverFixture) Pop(n uint32) error {
	for ; n != 0; n-- {
		i := len(s.scopes) - 1
		s.assertions = s.assertions[:s.scopes[i]]
		s.scopes = s.scopes[:i]
	}
	return nil
}
func (s *solverFixture) ScopeLevel() (uint32, error) { return uint32(len(s.scopes)), nil }
func (s *solverFixture) CheckSat(assumptions []ast.ASTHandle) (lbool.LBool, error) {
	status := s.statuses[s.checks]
	s.checks++
	if err := s.SetStatus(status); err != nil {
		return lbool.Undef, err
	}
	if status == lbool.False {
		if err := s.SetUnsatCore(assumptions); err != nil {
			return lbool.Undef, err
		}
	}
	return status, nil
}
func (s *solverFixture) SetProgressCallback(ProgressCallback) error { return nil }
func (s *solverFixture) NumAssertions() (uint32, error)             { return uint32(len(s.assertions)), nil }
func (s *solverFixture) Assertion(i uint32) (ast.ASTHandle, error)  { return s.assertions[i], nil }
func (s *solverFixture) NumAssumptions() (uint32, error)            { return 0, nil }
func (s *solverFixture) Assumption(uint32) (ast.ASTHandle, error)   { return ast.ASTHandle{}, nil }
func (s *solverFixture) GetConsequences(a, v []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) {
	return lbool.True, nil, nil
}
func (s *solverFixture) FindMutexes(v []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	return DefaultFindMutexes(v)
}
func (s *solverFixture) PreferredSAT([]ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	return DefaultPreferredSAT(s)
}
func (s *solverFixture) Display(out io.Writer, assumptions []ast.ASTHandle) error {
	return DefaultDisplay(s, out, assumptions)
}

type fixedConsequenceEvaluator struct {
	manager *ast.ASTManager
	value   ast.ASTHandle
}

func (e *fixedConsequenceEvaluator) EvaluateValue(ast.ASTHandle) (ast.ASTRef, error) {
	return e.manager.NewRef(e.value)
}
func (e *fixedConsequenceEvaluator) Close() error { return nil }

func TestSolverDefaultsScopedPushAssertionsAndConsequence(t *testing.T) {
	m := solverTestManager(t)
	result, err := NewSimpleCheckSatResult(m)
	solverMust(t, err)
	s := &solverFixture{SimpleCheckSatResult: result, manager: m, statuses: []lbool.LBool{lbool.True, lbool.False}}
	t.Cleanup(func() { solverMust(t, result.Close()) })
	boolSort, err := m.MkBoolSort()
	solverMust(t, err)
	name, err := m.Symbols.Intern("solver-variable")
	solverMust(t, err)
	variable, err := m.MkConst(name, boolSort)
	solverMust(t, err)
	if literal, err := IsLiteral(m, variable); err != nil || !literal {
		t.Fatal("uninterpreted constant literal", literal, err)
	}
	notVariable, err := m.MkNot(variable)
	solverMust(t, err)
	if literal, err := IsLiteral(m, notVariable); err != nil || !literal {
		t.Fatal("negated uninterpreted constant literal", literal, err)
	}
	assumptionName, err := m.Symbols.Intern("solver-assumption")
	solverMust(t, err)
	assumption, err := m.MkConst(assumptionName, boolSort)
	solverMust(t, err)
	solverMust(t, s.Assert(assumption))
	if got, err := Assertions(s); err != nil || len(got) != 1 || got[0] != assumption {
		t.Fatal("assertion snapshot", got, err)
	}
	var display strings.Builder
	solverMust(t, s.Display(&display, []ast.ASTHandle{variable}))
	if display.String() != "(declare-fun solver-assumption () Bool)\n(declare-fun solver-variable () Bool)\n(assert solver-assumption)\n" {
		t.Fatal("solver display", display.String())
	}
	scope, err := NewScopedPush(s)
	solverMust(t, err)
	solverMust(t, s.Assert(variable))
	solverMust(t, scope.Close())
	if len(s.assertions) != 1 {
		t.Fatal("scoped push did not restore assertions")
	}
	md, err := model.NewModel(m)
	solverMust(t, err)
	solverMust(t, result.SetModel(md.Core()))
	falsity, err := m.MkFalse()
	solverMust(t, err)
	status, consequences, err := DefaultGetConsequences(s, []ast.ASTHandle{assumption}, []ast.ASTHandle{variable}, func(*model.ModelCore) (ConsequenceEvaluator, error) {
		return &fixedConsequenceEvaluator{manager: m, value: falsity}, nil
	})
	solverMust(t, err)
	if status != lbool.True || len(consequences) != 1 || s.checks != 2 {
		t.Fatal("default consequences", status, len(consequences), s.checks)
	}
	if _, err = m.GetSort(consequences[0]); err != nil {
		t.Fatal("Boolean consequence lifetime", err)
	}
}
