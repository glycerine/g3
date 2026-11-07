package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTArrayBackendSelection(t *testing.T) {
	for _, test := range []struct {
		logic        string
		auto, simple bool
	}{
		{"QF_AX", true, true},
		{"QF_AX", false, false},
		{"ALL", true, false},
		{"QF_AUFBV", true, true},
	} {
		t.Run(fmt.Sprint(test.logic, "/", test.auto), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			family, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			params, err := cfg.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
			name := "auto_config"
			astTestMust(t, cfg.Context.SetBoolName(&params, &name, test.auto))
			solver, err := NewSMTSolver(m, &params, test.logic, SMTSolverConfig{Params: cfg.Context})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			core := smtArrayCore(solver.kernel.internalizer.theories[family])
			if status != lbool.True || core == nil || core.simple != test.simple {
				t.Fatal("array setup selected incorrect backend", status, core)
			}
		})
	}
}

func TestSMTArrayBackendOperationBoundary(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, op := range []int32{ast.ASTArrayOpStore, ast.ASTArrayOpSelect, ast.ASTArrayOpConst, ast.ASTArrayOpDefault, ast.ASTArrayOpMap, ast.ASTArrayOpAsArray, ast.ASTArrayOpExt} {
			t.Run(fmt.Sprint(simple, "/", op), func(t *testing.T) {
				m := astTestManager(t)
				family, err := m.RegisterArrayPlugin()
				astTestMust(t, err)
				_, _, state := smtInternalizerFixture(t, m, nil)
				var plugin SMTTheoryInternalizer
				if simple {
					plugin, err = NewSMTSimpleArrayTheory(family)
				} else {
					plugin, err = NewSMTArrayTheory(family)
				}
				astTestMust(t, err)
				astTestMust(t, state.RegisterTheory(plugin))
				core := smtArrayCore(plugin)
				boolean, err := m.MkBoolSort()
				astTestMust(t, err)
				sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
				astTestMust(t, err)
				array, err := m.MkConst(astTestSymbol(t, m, "boundary-array"), sort)
				astTestMust(t, err)
				value, err := m.MkTrue()
				astTestMust(t, err)
				function, err := m.MkFuncDecl(astTestSymbol(t, m, "boundary-f"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				var expression ast.ASTHandle
				switch op {
				case ast.ASTArrayOpStore:
					expression, err = m.MkFamilyApp(family, op, nil, []ast.ASTHandle{array, value, value})
				case ast.ASTArrayOpSelect:
					expression, err = m.MkFamilyApp(family, op, nil, []ast.ASTHandle{array, value})
				case ast.ASTArrayOpConst:
					expression, err = m.MkConstArray(sort, value)
				case ast.ASTArrayOpDefault:
					expression, err = m.MkFamilyApp(family, op, nil, []ast.ASTHandle{array})
				case ast.ASTArrayOpMap:
					expression, err = m.MkArrayMap(function, ast.SliceArguments([]ast.ASTHandle{array}))
				case ast.ASTArrayOpAsArray:
					expression, err = m.MkAsArray(function)
				case ast.ASTArrayOpExt:
					var decl ast.ASTHandle
					decl, err = m.MkArrayExtDecl(sort, 0)
					astTestMust(t, err)
					expression, err = m.MkApp(decl, []ast.ASTHandle{array, array})
				}
				astTestMust(t, err)
				astTestMust(t, state.PushScope())
				handled, err := core.InternalizeTerm(expression)
				astTestMust(t, err)
				wantHandled := !simple || op == ast.ASTArrayOpStore || op == ast.ASTArrayOpSelect
				wantUnsupported := op != ast.ASTArrayOpExt && (simple && !wantHandled || op == ast.ASTArrayOpAsArray)
				if handled != wantHandled || core.unsupported != wantUnsupported {
					t.Fatal("array operation boundary", handled, core.unsupported, "want", wantHandled, wantUnsupported)
				}
				astTestMust(t, state.PopScopes(1))
				if core.unsupported {
					t.Fatal("unsupported operation survived scope pop")
				}
			})
		}
	}
}

func TestSMTArrayBooleanGateEnodesRetainAntecedentsAndScope(t *testing.T) {
	for _, operation := range []int32{ast.ASTArrayOpSelect, ast.ASTArrayOpDefault} {
		for _, truth := range []bool{false, true} {
			t.Run(fmt.Sprint(operation, "/", truth), func(t *testing.T) {
				m := astTestManager(t)
				family, err := m.RegisterArrayPlugin()
				astTestMust(t, err)
				_, _, state := smtInternalizerFixture(t, m, nil)
				theory, err := NewSMTArrayTheory(family)
				astTestMust(t, err)
				astTestMust(t, state.RegisterTheory(theory))
				boolean, err := m.MkBoolSort()
				astTestMust(t, err)
				sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
				astTestMust(t, err)
				array, err := m.MkConst(astTestSymbol(t, m, "array-Boolean-gate"), sort)
				astTestMust(t, err)
				arguments := []ast.ASTHandle{array}
				if operation == ast.ASTArrayOpSelect {
					index, err := m.MkTrue()
					astTestMust(t, err)
					arguments = append(arguments, index)
				}
				expression, err := m.MkFamilyApp(family, operation, nil, arguments)
				astTestMust(t, err)
				astTestMust(t, state.PushScope())
				astTestMust(t, state.Internalize(expression, true))
				literal, err := state.Literal(expression)
				astTestMust(t, err)
				if !truth {
					literal = literal.Negated()
				}
				data, err := state.BooleanData(literal.Var())
				astTestMust(t, err)
				if !data.IsENode() {
					t.Fatal("Boolean array term lost its source enode flag in gate context")
				}
				astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
				_, err = state.PropagateCore()
				astTestMust(t, err)
				resolver, err := NewSMTConflictResolver(state)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, resolver.Close()) })
				premise, err := resolver.expander.LiteralForENode(state.context.enodes.get(expression))
				astTestMust(t, err)
				if premise != literal {
					t.Fatal("Boolean enode antecedent polarity", premise, literal)
				}
				astTestMust(t, state.PopScopes(1))
				if _, found := state.BoolVar(expression); found || state.context.enodes.get(expression) != nil {
					t.Fatal("child array gate survived scope pop")
				}
			})
		}
	}
}
