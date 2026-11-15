package spacer_test

// Source: src/muz/spacer/spacer_unsat_core_learner.cpp proof marking and
// unsat_core_plugin_lemma lowest B-pure split extraction.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestUnsatCoreLearnerMarksProofAndExtractsLowestBPureSplit(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	p := spacerBool(t, manager, "new-core-b")
	q := spacerBool(t, manager, "new-core-outside-b")
	notP, err := manager.MkNot(p)
	spacerMust(t, err)
	bProof, err := manager.MkProofAsserted(p)
	spacerMust(t, err)
	aProof, err := manager.MkProofAsserted(notP)
	spacerMust(t, err)
	root, err := manager.MkProofUnitResolution([]ast.ASTHandle{bProof, aProof})
	spacerMust(t, err)
	rootRef, err := manager.NewRef(root)
	spacerMust(t, err)

	learner, err := spacer.NewUnsatCoreLearner(manager)
	spacerMust(t, err)
	plugin, err := spacer.NewUnsatCoreLemmaPlugin(learner)
	spacerMust(t, err)
	spacerMust(t, learner.RegisterPlugin(plugin))
	core, err := learner.ComputeUnsatCore(root, []ast.ASTHandle{p})
	spacerMust(t, err)
	if len(core) != 1 {
		t.Fatal("ordinary lemma plugin core size", len(core))
	}
	coreHandle, err := manager.RefGet(core[0])
	spacerMust(t, err)
	if coreHandle != p {
		t.Fatal("ordinary lemma plugin selected wrong split", coreHandle, p)
	}
	spacerMust(t, manager.DelRef(&core[0]))
	pure, err := learner.OnlyContainsSymbolsB(p)
	spacerMust(t, err)
	if !pure {
		t.Fatal("B assertion was not B-pure")
	}
	pure, err = learner.OnlyContainsSymbolsB(q)
	spacerMust(t, err)
	if pure {
		t.Fatal("foreign declaration was classified as B-pure")
	}
	spacerMust(t, learner.Close())
	if _, err = learner.ComputeUnsatCore(root, nil); !errors.Is(err, spacer.ErrUnsatCoreLearnerOwner) {
		t.Fatal("closed learner accepted proof", err)
	}
	if err = plugin.Finalize(); !errors.Is(err, spacer.ErrUnsatCoreLearnerOwner) {
		t.Fatal("closed owned plugin accepted operation", err)
	}
	spacerMust(t, manager.DelRef(&rootRef))
	spacerMust(t, command.Close())
}

func TestUnsatCoreMinCutPluginExtractsProofFrontier(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	p := spacerBool(t, manager, "min-cut-b")
	notP, err := manager.MkNot(p)
	spacerMust(t, err)
	bProof, err := manager.MkProofAsserted(p)
	spacerMust(t, err)
	aProof, err := manager.MkProofAsserted(notP)
	spacerMust(t, err)
	root, err := manager.MkProofUnitResolution([]ast.ASTHandle{bProof, aProof})
	spacerMust(t, err)
	rootRef, err := manager.NewRef(root)
	spacerMust(t, err)

	learner, err := spacer.NewUnsatCoreLearner(manager)
	spacerMust(t, err)
	plugin, err := spacer.NewUnsatCoreMinCutPlugin(learner)
	spacerMust(t, err)
	spacerMust(t, learner.RegisterPlugin(plugin))
	core, err := learner.ComputeUnsatCore(root, []ast.ASTHandle{p})
	spacerMust(t, err)
	if len(core) != 1 {
		t.Fatal("min-cut core size", len(core))
	}
	handle, err := manager.RefGet(core[0])
	spacerMust(t, err)
	if handle != p {
		t.Fatal("min-cut selected wrong proof frontier", handle, p)
	}
	spacerMust(t, manager.DelRef(&core[0]))
	spacerMust(t, learner.Close())
	if err = plugin.Finalize(); !errors.Is(err, spacer.ErrUnsatCoreLearnerOwner) {
		t.Fatal("closed min-cut plugin accepted finalization", err)
	}
	spacerMust(t, manager.DelRef(&rootRef))
	spacerMust(t, command.Close())
}

func TestUnsatCoreFarkasPluginClosesArithmeticMixedStep(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	spacerMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	spacerMust(t, err)
	xName, err := manager.Symbols.Intern("new-core-farkas-x")
	spacerMust(t, err)
	x, err := manager.MkConst(xName, integer)
	spacerMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	spacerMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	spacerMust(t, err)
	le, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{x, zero})
	spacerMust(t, err)
	ge, err := manager.MkFamilyApp(arithmetic, 3, nil, []ast.ASTHandle{x, one})
	spacerMust(t, err)
	bProof, err := manager.MkProofAsserted(le)
	spacerMust(t, err)
	aProof, err := manager.MkProofAsserted(ge)
	spacerMust(t, err)
	falsity, err := manager.MkFalse()
	spacerMust(t, err)
	farkasName, err := manager.Symbols.Intern("farkas")
	spacerMust(t, err)
	farkasParameter, err := manager.NewParameterSymbol(farkasName)
	spacerMust(t, err)
	coefficientValue, err := manager.NumeralService().NewValue()
	spacerMust(t, err)
	spacerMust(t, manager.NumeralService().SetString(&coefficientValue, "1"))
	coefficient0, err := manager.NewParameterRational(&coefficientValue)
	spacerMust(t, err)
	coefficient1, err := manager.NewParameterRational(&coefficientValue)
	spacerMust(t, err)
	spacerMust(t, manager.NumeralService().Del(&coefficientValue))
	root, err := manager.MkProofTheoryLemma(arithmetic, falsity, []ast.ASTHandle{bProof, aProof}, []ast.ASTParameter{farkasParameter, coefficient0, coefficient1})
	spacerMust(t, err)
	spacerMust(t, manager.DelParameter(&coefficient1))
	spacerMust(t, manager.DelParameter(&coefficient0))
	spacerMust(t, manager.DelParameter(&farkasParameter))
	rootRef, err := manager.NewRef(root)
	spacerMust(t, err)

	for _, test := range []struct {
		name             string
		optimized        bool
		useConstantFromA bool
	}{
		{name: "a-constant", useConstantFromA: true},
		{name: "b-constant", useConstantFromA: false},
		{name: "optimized", optimized: true},
	} {
		learner, err := spacer.NewUnsatCoreLearner(manager)
		spacerMust(t, err)
		if test.optimized {
			farkasPlugin, err := spacer.NewUnsatCoreOptimizedFarkasPlugin(learner, globals.Context, globals)
			spacerMust(t, err)
			spacerMust(t, learner.RegisterPlugin(farkasPlugin))
		} else {
			farkasPlugin, err := spacer.NewUnsatCoreFarkasPlugin(learner, globals.Context, globals, true, test.useConstantFromA)
			spacerMust(t, err)
			spacerMust(t, learner.RegisterPlugin(farkasPlugin))
		}
		fallback, err := spacer.NewUnsatCoreLemmaPlugin(learner)
		spacerMust(t, err)
		spacerMust(t, learner.RegisterPlugin(fallback))
		core, err := learner.ComputeUnsatCore(root, []ast.ASTHandle{le})
		spacerMust(t, err)
		if len(core) != 1 {
			t.Fatal("Farkas plugin core size", test.name, len(core))
		}
		handle, err := manager.RefGet(core[0])
		spacerMust(t, err)
		if handle.IsNull() {
			t.Fatal("Farkas plugin returned null lemma", test.name)
		}
		spacerMust(t, manager.DelRef(&core[0]))
		spacerMust(t, learner.Close())
	}
	spacerMust(t, manager.DelRef(&rootRef))
	spacerMust(t, command.Close())
}

// AUDIT.md muz_spacer_02-F2: compute_linear_combination combines through a
// fresh smt::farkas_util, which partitions variable-disjoint constraints
// only when split literals are enabled (use-constant-from-A mode only).
func TestUnsatCoreFarkasPluginCombinesDisjointPremisesWithoutSplitting(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	spacerMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	spacerMust(t, err)
	constant := func(name string) ast.ASTHandle {
		symbol, err := manager.Symbols.Intern(name)
		spacerMust(t, err)
		value, err := manager.MkConst(symbol, integer)
		spacerMust(t, err)
		return value
	}
	x, y := constant("disjoint-farkas-x"), constant("disjoint-farkas-y")
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	spacerMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	spacerMust(t, err)
	xLE, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{x, zero})
	spacerMust(t, err)
	yLE, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{y, zero})
	spacerMust(t, err)
	xGE, err := manager.MkFamilyApp(arithmetic, 3, nil, []ast.ASTHandle{x, one})
	spacerMust(t, err)
	yGE, err := manager.MkFamilyApp(arithmetic, 3, nil, []ast.ASTHandle{y, one})
	spacerMust(t, err)
	facts := []ast.ASTHandle{xLE, yLE, xGE, yGE}
	premises := make([]ast.ASTHandle, len(facts))
	for i, fact := range facts {
		premises[i], err = manager.MkProofAsserted(fact)
		spacerMust(t, err)
	}
	falsity, err := manager.MkFalse()
	spacerMust(t, err)
	farkasName, err := manager.Symbols.Intern("farkas")
	spacerMust(t, err)
	parameters := []ast.ASTParameter{}
	farkasParameter, err := manager.NewParameterSymbol(farkasName)
	spacerMust(t, err)
	parameters = append(parameters, farkasParameter)
	coefficientValue, err := manager.NumeralService().NewValue()
	spacerMust(t, err)
	spacerMust(t, manager.NumeralService().SetString(&coefficientValue, "1"))
	for range facts {
		coefficient, err := manager.NewParameterRational(&coefficientValue)
		spacerMust(t, err)
		parameters = append(parameters, coefficient)
	}
	spacerMust(t, manager.NumeralService().Del(&coefficientValue))
	root, err := manager.MkProofTheoryLemma(arithmetic, falsity, premises, parameters)
	spacerMust(t, err)
	for i := range parameters {
		spacerMust(t, manager.DelParameter(&parameters[i]))
	}
	rootRef, err := manager.NewRef(root)
	spacerMust(t, err)
	// direct is farkas_util add+get over the literals the plugin selects.
	direct := func(literals []ast.ASTHandle, negate bool) (ast.ASTHandle, ast.ASTRef) {
		util, err := smtcore.NewSMTFarkasUtil(manager, globals.Context, globals)
		spacerMust(t, err)
		for _, literal := range literals {
			spacerMust(t, util.Add(big.NewRat(1, 1), literal))
		}
		out, err := manager.NewRef(ast.ASTHandle{})
		spacerMust(t, err)
		spacerMust(t, util.Get(out))
		spacerMust(t, util.Close())
		value, err := manager.RefGet(out)
		spacerMust(t, err)
		if negate {
			value, err = manager.MkNotSimplified(value)
			spacerMust(t, err)
		}
		return value, out
	}
	for _, useConstantFromA := range []bool{true, false} {
		learner, err := spacer.NewUnsatCoreLearner(manager)
		spacerMust(t, err)
		farkasPlugin, err := spacer.NewUnsatCoreFarkasPlugin(learner, globals.Context, globals, false, useConstantFromA)
		spacerMust(t, err)
		spacerMust(t, learner.RegisterPlugin(farkasPlugin))
		fallback, err := spacer.NewUnsatCoreLemmaPlugin(learner)
		spacerMust(t, err)
		spacerMust(t, learner.RegisterPlugin(fallback))
		core, err := learner.ComputeUnsatCore(root, []ast.ASTHandle{xLE, yLE})
		spacerMust(t, err)
		if len(core) != 1 {
			t.Fatal("Farkas plugin core size", useConstantFromA, len(core))
		}
		lemma, err := manager.RefGet(core[0])
		spacerMust(t, err)
		selected := []ast.ASTHandle{xGE, yGE}
		if !useConstantFromA {
			selected = []ast.ASTHandle{xLE, yLE}
		}
		want, wantRef := direct(selected, !useConstantFromA)
		if lemma != want {
			t.Fatal("Farkas plugin lemma differs from farkas_util", useConstantFromA)
		}
		family, err := manager.GetFamilyID(lemma)
		spacerMust(t, err)
		kind, err := manager.GetDeclKind(lemma)
		spacerMust(t, err)
		if family == manager.BasicFamilyID() && kind == ast.ASTOpOr {
			t.Fatal("Farkas plugin split an unsplit combination", useConstantFromA)
		}
		spacerMust(t, manager.DelRef(&wantRef))
		spacerMust(t, manager.DelRef(&core[0]))
		spacerMust(t, learner.Close())
	}
	spacerMust(t, manager.DelRef(&rootRef))
	spacerMust(t, command.Close())
}
