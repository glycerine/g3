package smtcore

// Source: src/smt/smt_setup.cpp operator()/setup_unknown and
// src/smt/smt_context.cpp copy_plugins/register_plugin.

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtSetupPluginOrder(t *testing.T, k *SMTKernel) []int {
	t.Helper()
	byFamily := map[ast.FamilyID]int{}
	for plugin := smtSetupArith; plugin <= smtSetupFPA; plugin++ {
		family, err := k.smtSetupPluginFamily(plugin)
		astTestMust(t, err)
		byFamily[family] = plugin
	}
	var order []int
	for _, theory := range k.internalizer.theoryOrder {
		plugin, ok := byFamily[theory.FamilyID()]
		if !ok {
			plugin = -1
		}
		order = append(order, plugin)
	}
	return order
}

func smtSetupPluginSolver(t *testing.T, logic string, autoConfig bool) (*SMTSolver, ast.ASTHandle) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	name := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&params, &name, autoConfig))
	s, err := NewSMTSolver(m, &params, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	n, err := m.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	astTestMust(t, err)
	sort, err := m.GetSort(n)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "plugin-set-x"), sort)
	astTestMust(t, err)
	bound, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, n})
	astTestMust(t, err)
	return s, bound
}

// CFG_BASIC runs setup_unknown() whatever the logic: every plugin, in its
// registration order.
func TestSMTSetupBasicRegistersEveryPlugin(t *testing.T) {
	all := smtSetupAllPlugins()
	for _, logic := range []string{"QF_LIA", "QF_UF", "QF_BV", "QF_AUFBV", "QF_FP", "AUFLIA", ""} {
		s, bound := smtSetupPluginSolver(t, logic, false)
		astTestMust(t, s.Assert(bound))
		status, err := s.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal(logic, status)
		}
		if got := smtSetupPluginOrder(t, s.kernel); !reflect.DeepEqual(got, all) {
			t.Fatal("CFG_BASIC plugin order", logic, got)
		}
	}
}

// copy_plugins registers the source's theory set in source order before the
// destination's setup; that setup only appends missing profile plugins. The
// model checker's auxiliary context checks with CFG_LOGIC and an empty logic,
// so its profile is setup_unknown().
func TestSMTSetupAuxiliaryKeepsSourcePluginOrder(t *testing.T) {
	for _, tc := range []struct {
		logic        string
		source, want []int
	}{
		{"QF_LIA", []int{smtSetupArith}, smtSetupAllPlugins()},
		{"QF_AUFBV", []int{smtSetupBV, smtSetupArray}, []int{smtSetupBV, smtSetupArray, smtSetupArith, smtSetupDatatype, smtSetupDatalog, smtSetupSeq, smtSetupPB, smtSetupFPA}},
	} {
		s, _ := smtSetupPluginSolver(t, tc.logic, true)
		boolean, err := s.manager.MkBoolSort()
		astTestMust(t, err)
		p, err := s.manager.MkConst(astTestSymbol(t, s.manager, "plugin-set-p"), boolean)
		astTestMust(t, err)
		astTestMust(t, s.Assert(p))
		status, err := s.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal(tc.logic, status)
		}
		if got := smtSetupPluginOrder(t, s.kernel); !reflect.DeepEqual(got, tc.source) {
			t.Fatal("source plugin order", tc.logic, got)
		}
		a, err := s.newAuxiliarySolver()
		astTestMust(t, err)
		astTestMust(t, a.Assert(p))
		status, err = a.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("auxiliary", tc.logic, status)
		}
		if got := smtSetupPluginOrder(t, a.kernel); !reflect.DeepEqual(got, tc.want) {
			t.Fatal("auxiliary plugin order", tc.logic, got)
		}
		astTestMust(t, a.Close())
	}
}
