package tactic

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func atomMapInterpretedApplication(t *testing.T, m *ast.ASTManager, name string, arg ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	boolean, err := m.MkBoolSort()
	satOK(t, err)
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	satOK(t, err)
	result, err := m.MkApp(decl, []ast.ASTHandle{arg})
	satOK(t, err)
	astTestPin(t, m, result)
	return result
}

func TestAtom2BoolVarGoalTranslatorClassificationAndExternalFlags(t *testing.T) {
	for _, defaultExternal := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "default-external"}[defaultExternal], func(t *testing.T) {
			m := astTestManager(t)
			s, _, _ := satSolverCheckFixture(t)
			mapping, err := NewAtom2BoolVar(m)
			satOK(t, err)
			defer func() { satOK(t, mapping.Close()) }()
			p := goalTestAtom(t, m, "classification-p")
			q := goalTestAtom(t, m, "classification-q")
			application := atomMapInterpretedApplication(t, m, "classification-f", p)
			tr, err := NewGoalTranslatorWithAtomMap(m, s, mapping, defaultExternal)
			satOK(t, err)
			defer func() { satOK(t, tr.Close()) }()
			satOK(t, tr.Compile([]ast.ASTHandle{q, p, application}))
			pv, err := mapping.ToBoolVar(p)
			satOK(t, err)
			qv, err := mapping.ToBoolVar(q)
			satOK(t, err)
			av, err := mapping.ToBoolVar(application)
			satOK(t, err)
			if pv == sat.NullBoolVar || qv == sat.NullBoolVar || av == sat.NullBoolVar || pv == av || qv == av {
				t.Fatal("atom mapping", pv, qv, av)
			}
			pExternal, err := s.SourceIsExternal(pv)
			satOK(t, err)
			qExternal, err := s.SourceIsExternal(qv)
			satOK(t, err)
			aExternal, err := s.SourceIsExternal(av)
			satOK(t, err)
			// The interpreted application is handled by quick_for_each_expr;
			// its uninterpreted argument is therefore an interface variable.
			if !pExternal || qExternal != defaultExternal || !aExternal {
				t.Fatal("source external selection", defaultExternal, pExternal, qExternal, aExternal)
			}
			if interpreted, err := mapping.InterpretedAtoms(); err != nil || !interpreted {
				t.Fatal("cumulative interpreted flag", interpreted, err)
			}
			atoms, err := tr.InterpretedAtoms()
			satOK(t, err)
			if len(atoms) != 1 || atoms[0] != application {
				t.Fatal("per-call interpreted atoms", atoms)
			}
		})
	}
}

func TestAtom2BoolVarBooleanInterfaceSourceOperatorBoundary(t *testing.T) {
	for _, test := range []struct {
		name      string
		make      func(*testing.T, *ast.ASTManager, ast.ASTHandle, ast.ASTHandle) ast.ASTHandle
		externalP bool
	}{
		{"or", func(t *testing.T, m *ast.ASTManager, p, q ast.ASTHandle) ast.ASTHandle {
			return goalTestApp(t, m, ast.ASTOpOr, p, q)
		}, false},
		{"not", func(t *testing.T, m *ast.ASTManager, p, _ ast.ASTHandle) ast.ASTHandle {
			return goalTestApp(t, m, ast.ASTOpNot, p)
		}, false},
		{"iff", func(t *testing.T, m *ast.ASTManager, p, q ast.ASTHandle) ast.ASTHandle {
			return goalTestApp(t, m, ast.ASTOpIff, p, q)
		}, false},
		{"bool-eq", func(t *testing.T, m *ast.ASTManager, p, q ast.ASTHandle) ast.ASTHandle {
			return goalTestApp(t, m, ast.ASTOpEq, p, q)
		}, false},
		{"bool-ite", func(t *testing.T, m *ast.ASTManager, p, q ast.ASTHandle) ast.ASTHandle {
			return goalTestApp(t, m, ast.ASTOpIte, p, q, p)
		}, false},
		// collect_boolean_interface_proc deliberately omits AND from its basic
		// descent branch. Preserve that source behavior even though goal2sat can
		// encode AND itself.
		{"and-omission", func(t *testing.T, m *ast.ASTManager, p, q ast.ASTHandle) ast.ASTHandle {
			return goalTestApp(t, m, ast.ASTOpAnd, p, q)
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			s, _, _ := satSolverCheckFixture(t)
			mapping, err := NewAtom2BoolVar(m)
			satOK(t, err)
			defer func() { satOK(t, mapping.Close()) }()
			p := goalTestAtom(t, m, "interface-p")
			q := goalTestAtom(t, m, "interface-q")
			formula := test.make(t, m, p, q)
			tr, err := NewGoalTranslatorWithAtomMap(m, s, mapping, false)
			satOK(t, err)
			defer func() { satOK(t, tr.Close()) }()
			satOK(t, tr.Compile([]ast.ASTHandle{formula}))
			pv, err := mapping.ToBoolVar(p)
			satOK(t, err)
			if pv == sat.NullBoolVar {
				t.Fatal("formula did not reach p")
			}
			external, err := s.SourceIsExternal(pv)
			satOK(t, err)
			if external != test.externalP {
				t.Fatal("interface external flag", external, test.externalP)
			}
		})
	}
}

func TestAtom2BoolVarDependencyInterfacePrecedesFormulaCollection(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	mapping, err := NewAtom2BoolVar(m)
	satOK(t, err)
	defer func() { satOK(t, mapping.Close()) }()
	label := goalTestAtom(t, m, "dependency-interface-label")
	formula := goalTestAtom(t, m, "dependency-interface-formula")
	dependency, err := m.DependencyLeaf(label)
	satOK(t, err)
	g := goalTestNew(t, m, false, true)
	satOK(t, g.AssertWithProof(formula, ast.ASTHandle{}, dependency))
	tr, err := NewGoalTranslatorWithAtomMap(m, s, mapping, false)
	satOK(t, err)
	defer func() { satOK(t, tr.Close()) }()
	satOK(t, tr.CompileGoal(g))
	lv, err := mapping.ToBoolVar(label)
	satOK(t, err)
	fv, err := mapping.ToBoolVar(formula)
	satOK(t, err)
	if lv == sat.NullBoolVar || fv == sat.NullBoolVar {
		t.Fatal("dependency/formula mapping", lv, fv)
	}
	labelExternal, err := s.SourceIsExternal(lv)
	satOK(t, err)
	formulaExternal, err := s.SourceIsExternal(fv)
	satOK(t, err)
	if !labelExternal || formulaExternal {
		t.Fatal("dependency-first Boolean interface", labelExternal, formulaExternal)
	}
	bindings, err := tr.DependencyLiterals()
	satOK(t, err)
	if len(bindings) != 1 || bindings[0].Expression != label || bindings[0].Literal.Variable() != lv {
		t.Fatal("dependency literal mapping", bindings)
	}
}

func TestAtom2BoolVarPerCallDiscoveryAndCumulativeMapState(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	mapping, err := NewAtom2BoolVar(m)
	satOK(t, err)
	defer func() { satOK(t, mapping.Close()) }()
	p := goalTestAtom(t, m, "per-call-p")
	application := atomMapInterpretedApplication(t, m, "per-call-f", p)
	for call := 0; call < 3; call++ {
		tr, err := NewGoalTranslatorWithAtomMap(m, s, mapping, true)
		satOK(t, err)
		satOK(t, tr.Compile([]ast.ASTHandle{application}))
		current, err := tr.InterpretedAtoms()
		satOK(t, err)
		if call == 0 {
			if len(current) != 1 || current[0] != application {
				t.Fatal("initial interpreted publication", current)
			}
		} else if len(current) != 0 {
			t.Fatal("existing map atom republished", call, current)
		}
		satOK(t, tr.Close())
		if interpreted, err := mapping.InterpretedAtoms(); err != nil || !interpreted {
			t.Fatal("cumulative mapping state", call, interpreted, err)
		}
	}
}

func TestAtom2BoolVarInverseFailurePrefixAndRetry(t *testing.T) {
	success := false
	for stop := int64(0); stop < 64 && !success; stop++ {
		t.Run(string(rune('A'+stop)), func(t *testing.T) {
			m := astTestManager(t)
			mapping, err := NewAtom2BoolVar(m)
			satOK(t, err)
			p := goalTestAtom(t, m, "inverse-failure-p")
			q := goalTestAtom(t, m, "inverse-failure-q")
			satOK(t, mapping.Insert(p, 0))
			satOK(t, mapping.Insert(q, 1))
			var inverse satGoalLiteralExpressions
			m.Budget().FailAfter(stop)
			err = mapping.mkInverse(&inverse)
			m.Budget().FailAfter(-1)
			if err == nil {
				success = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("inverse failure class", err)
			}
			// Every published prefix remains owning and can be destroyed before a
			// clean retry, matching expr_ref_vector lexical ownership.
			satOK(t, inverse.close(m))
			if err != nil {
				satOK(t, mapping.mkInverse(&inverse))
				satOK(t, inverse.close(m))
			}
			satOK(t, mapping.Close())
		})
	}
	if !success {
		t.Fatal("no successful inverse frontier")
	}
}

func TestAtom2BoolVarSATTacticModelAndResidualBranchSelection(t *testing.T) {
	for _, interpreted := range []bool{false, true} {
		t.Run(map[bool]string{false: "boolean-model", true: "residual"}[interpreted], func(t *testing.T) {
			m, r, _ := satTacticFixture(t)
			p := goalTestAtom(t, m, "branch-p")
			formula := p
			if interpreted {
				formula = atomMapInterpretedApplication(t, m, "branch-f", p)
			}
			g := goalTestNew(t, m, false, false)
			satOK(t, g.Assert(formula))
			var result tactic.TacticResult
			defer func() { satOK(t, result.Close()) }()
			satOK(t, r.Apply(g, &result))
			if interpreted {
				if result.DecidedSat() || result.DecidedUnsat() || result.ModelConverter == nil {
					t.Fatal("interpreted atom bypassed sat2goal", result.DecidedSat(), result.DecidedUnsat(), result.ModelConverter)
				}
				size, err := g.Size()
				satOK(t, err)
				if size == 0 {
					t.Fatal("residual branch recovered no formula")
				}
			} else {
				if !result.DecidedSat() || result.ModelConverter == nil {
					t.Fatal("pure Boolean atom did not build model", result.DecidedSat(), result.ModelConverter)
				}
				var ref tactic.ModelRef
				defer func() { satOK(t, ref.Close()) }()
				satOK(t, result.ModelConverter.ApplyUnindexed(&ref))
				model, err := ref.Get()
				satOK(t, err)
				decl, err := m.GetDecl(p)
				satOK(t, err)
				value, err := model.ConstantInterpretation(decl)
				satOK(t, err)
				if !m.IsTrue(value) {
					t.Fatal("pure Boolean model value", value)
				}
			}
		})
	}
}

func TestAtom2BoolVarScopePopFeedsIncrementalSolverIdentity(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	mapping, err := NewAtom2BoolVar(m)
	satOK(t, err)
	defer func() { satOK(t, mapping.Close()) }()
	p := goalTestAtom(t, m, "scope-p")
	compile := func(form ast.ASTHandle) sat.BoolVar {
		t.Helper()
		tr, err := NewGoalTranslatorWithAtomMap(m, s, mapping, true)
		satOK(t, err)
		satOK(t, tr.Compile([]ast.ASTHandle{form}))
		satOK(t, tr.Close())
		v, err := mapping.ToBoolVar(p)
		satOK(t, err)
		return v
	}
	satOK(t, s.UserPush())
	satOK(t, mapping.Push())
	first := compile(p)
	if first == sat.NullBoolVar {
		t.Fatal("scoped atom absent")
	}
	status, err := s.Check(nil)
	satOK(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	satOK(t, mapping.Pop(1))
	satOK(t, s.UserPop(1))
	if got, err := mapping.ToBoolVar(p); err != nil || got != sat.NullBoolVar {
		t.Fatal("scope pop did not erase atom", got, err)
	}
	second := compile(p)
	if second == sat.NullBoolVar || second == first {
		t.Fatal("new scope reused retired SAT identity", first, second)
	}
}
