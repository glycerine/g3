package tactic

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func satConfigModuleParams(t *testing.T, r *SATTactic) *uparams.ParamsRef {
	t.Helper()
	name, err := r.services.Globals.Symbols.Intern("sat")
	satOK(t, err)
	params, err := r.services.Globals.Current.GetParams(name)
	satOK(t, err)
	return params
}

func TestSATConfigCurrentModuleSnapshotAndLocalPrecedenceThroughTactic(t *testing.T) {
	for _, test := range []struct {
		name          string
		moduleLimit   string
		localLimit    *uint32
		moduleDIMACS  string
		localDIMACS   *bool
		wantDecided   bool
		wantDIMACSOut bool
	}{
		{"module residual", "0", nil, "true", nil, false, true},
		{"local solve override", "0", func() *uint32 { v := ^uint32(0); return &v }(), "true", func() *bool { v := false; return &v }(), true, false},
		{"local residual override", "4294967295", func() *uint32 { v := uint32(0); return &v }(), "false", func() *bool { v := true; return &v }(), false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, r, output := satTacticFixture(t)
			module := satConfigModuleParams(t, r)
			key := "max_conflicts"
			moduleLimit := uint32(0)
			if test.moduleLimit != "0" {
				moduleLimit = ^uint32(0)
			}
			satOK(t, r.context.SetUIntName(module, &key, moduleLimit))
			key = "dimacs.display"
			satOK(t, r.context.SetBoolName(module, &key, test.moduleDIMACS == "true"))
			params, err := r.context.NewValue()
			satOK(t, err)
			defer func() { satOK(t, r.context.Del(&params)) }()
			if test.localLimit != nil {
				key := "max_conflicts"
				satOK(t, r.context.SetUIntName(&params, &key, *test.localLimit))
			}
			if test.localDIMACS != nil {
				key := "dimacs.display"
				satOK(t, r.context.SetBoolName(&params, &key, *test.localDIMACS))
			}
			satOK(t, r.UpdateParams(&params))
			p := goalTestAtom(t, m, "module-p")
			q := goalTestAtom(t, m, "module-q")
			g := goalTestNew(t, m, false, false)
			satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpOr, p, q)))
			var result tactic.TacticResult
			defer func() { satOK(t, result.Close()) }()
			satOK(t, r.Apply(g, &result))
			if result.DecidedSat() != test.wantDecided || result.DecidedUnsat() {
				t.Fatal("sat_config decision path", result.DecidedSat(), result.DecidedUnsat(), test.wantDecided)
			}
			hasDIMACS := strings.Contains(output.String(), "p cnf")
			if hasDIMACS != test.wantDIMACSOut {
				t.Fatal("sat_config dimacs path", hasDIMACS, test.wantDIMACSOut, output.String())
			}
		})
	}
}

func TestSATConfigModuleReadOccursForEachFreshTacticSolver(t *testing.T) {
	m, r, output := satTacticFixture(t)
	p := goalTestAtom(t, m, "fresh-config-p")
	q := goalTestAtom(t, m, "fresh-config-q")
	apply := func() (sat bool) {
		t.Helper()
		g := goalTestNew(t, m, false, false)
		satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpOr, p, q)))
		var result tactic.TacticResult
		defer func() { satOK(t, result.Close()) }()
		satOK(t, r.Apply(g, &result))
		return result.DecidedSat()
	}
	module := satConfigModuleParams(t, r)
	key := "max_conflicts"
	satOK(t, r.context.SetUIntName(module, &key, 0))
	key = "dimacs.display"
	satOK(t, r.context.SetBoolName(module, &key, true))
	if apply() {
		t.Fatal("first fresh solver ignored module conflict limit")
	}
	firstOutput := output.Len()
	if firstOutput == 0 {
		t.Fatal("first fresh solver ignored module DIMACS switch")
	}
	key = "max_conflicts"
	satOK(t, r.context.SetUIntName(module, &key, ^uint32(0)))
	key = "dimacs.display"
	satOK(t, r.context.SetBoolName(module, &key, false))
	if !apply() {
		t.Fatal("second fresh solver reused stale sat_config")
	}
	if output.Len() != firstOutput {
		t.Fatal("second solver reused stale DIMACS switch", firstOutput, output.Len())
	}
}

func TestSATConfigInvalidPoliciesFailAtFreshSolverConstruction(t *testing.T) {
	for _, test := range []struct {
		key, value, message string
	}{
		{"restart", "invalid-restart", "invalid restart strategy"},
		{"phase", "invalid-phase", "invalid phase selection strategy"},
		{"gc", "invalid-gc", "invalid gc strategy"},
	} {
		t.Run(test.key, func(t *testing.T) {
			m, r, _ := satTacticFixture(t)
			params, err := r.context.NewValue()
			satOK(t, err)
			defer func() { satOK(t, r.context.Del(&params)) }()
			symbol, err := m.Symbols.Intern(test.value)
			satOK(t, err)
			satOK(t, r.context.SetSymbolName(&params, &test.key, symbol))
			satOK(t, r.UpdateParams(&params))
			p := goalTestAtom(t, m, "invalid-policy-p")
			g := goalTestNew(t, m, false, false)
			satOK(t, g.Assert(p))
			before, err := g.Size()
			satOK(t, err)
			var result tactic.TacticResult
			defer func() { satOK(t, result.Close()) }()
			err = r.Apply(g, &result)
			var ex z3exception.DefaultException
			if !errors.As(err, &ex) || err.Error() != test.message {
				t.Fatal("sat_config policy exception", err)
			}
			after, sizeErr := g.Size()
			satOK(t, sizeErr)
			if before != after || len(result.Goals) != 0 || result.ModelConverter != nil {
				t.Fatal("solver-construction failure published tactic outputs", before, after, len(result.Goals), result.ModelConverter)
			}
			if r.application != nil || r.active {
				t.Fatal("failed fresh solver retained application")
			}
		})
	}
}

func TestSATConfigPhaseSelectionReachesActualDecisionModel(t *testing.T) {
	for _, test := range []struct {
		phase     string
		wantFirst bool
	}{
		{"always_true", true},
		{"always_false", false},
	} {
		t.Run(test.phase, func(t *testing.T) {
			m, r, _ := satTacticFixture(t)
			params, err := r.context.NewValue()
			satOK(t, err)
			defer func() { satOK(t, r.context.Del(&params)) }()
			phase, err := m.Symbols.Intern(test.phase)
			satOK(t, err)
			key := "phase"
			satOK(t, r.context.SetSymbolName(&params, &key, phase))
			key = "random_freq"
			satOK(t, r.context.SetDoubleName(&params, &key, 0))
			key = "elim_vars"
			satOK(t, r.context.SetBoolName(&params, &key, false))
			satOK(t, r.UpdateParams(&params))
			p := goalTestAtom(t, m, "phase-p")
			q := goalTestAtom(t, m, "phase-q")
			g := goalTestNew(t, m, false, false)
			satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpOr, p, q)))
			var result tactic.TacticResult
			defer func() { satOK(t, result.Close()) }()
			satOK(t, r.Apply(g, &result))
			if !result.DecidedSat() || result.ModelConverter == nil {
				t.Fatal("phase policy did not solve Boolean goal")
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
			if m.IsTrue(value) != test.wantFirst || m.IsFalse(value) == test.wantFirst {
				t.Fatal("phase selection model", test.phase, value, test.wantFirst)
			}
		})
	}
}
