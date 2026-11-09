package pdr_test

// Source: src/muz/pdr/pdr_prop_solver.cpp level guards, safe proxies,
// subset-core recovery, temporary formulas and model acquisition.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func pdrPropFixture(t *testing.T, maximum uint32) (*pdr.PropSolver, func()) {
	t.Helper()
	command, manager, contexts := pdrContextFixture(t, maximum)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	solver, err := pdr.NewPropSolver(manager, contexts, truth, "pdr-test")
	pdrMust(t, err)
	return solver, func() {
		pdrMust(t, solver.Close())
		pdrMust(t, contexts.Close())
		pdrMust(t, command.Close())
	}
}

func TestPropSolverLevelFramesCoresAndModels(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 1)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	solver, err := pdr.NewPropSolver(manager, contexts, truth, "frames")
	pdrMust(t, err)
	p := pdrBool(t, manager, "frame-p")
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	pdrMust(t, solver.AddLevelFormula(p, 0))

	result, err := solver.CheckAtLevel(0, []ast.ASTHandle{notP}, true, false)
	pdrMust(t, err)
	if result.Status != lbool.False || !result.AssumesLevel || len(result.Core) != 1 || result.Core[0] != notP {
		t.Fatal("active level result", result.Status, result.AssumesLevel, result.Core)
	}
	result, err = solver.CheckAtLevel(1, []ast.ASTHandle{notP}, false, true)
	pdrMust(t, err)
	if result.Status != lbool.True || result.Model == nil {
		t.Fatal("inactive level model", result.Status, result.Model)
	}
	pdrMust(t, result.Model.DecRef())
	levels, err := solver.LevelCount()
	pdrMust(t, err)
	if levels != 2 {
		t.Fatal("ensured frame count", levels)
	}

	pdrMust(t, solver.Close())
	pdrMust(t, contexts.Close())
	pdrMust(t, command.Close())
}

func TestPropSolverSafeProxyCoreAndTemporaryFormulaRollback(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 0)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	solver, err := pdr.NewPropSolver(manager, contexts, truth, "safe")
	pdrMust(t, err)
	p := pdrBool(t, manager, "safe-p")
	q := pdrBool(t, manager, "safe-q")
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	notQ, err := manager.MkNot(q)
	pdrMust(t, err)
	pdrMust(t, solver.AddFormula(notP))
	pdrMust(t, solver.AddFormula(notQ))
	disjunction, err := manager.MkOr(p, q)
	pdrMust(t, err)
	result, err := solver.CheckAssumptions([]ast.ASTHandle{disjunction}, true, false)
	pdrMust(t, err)
	if result.Status != lbool.False || len(result.Core) != 1 || result.Core[0] != disjunction {
		t.Fatal("proxy subset core", result.Status, result.Core)
	}

	r := pdrBool(t, manager, "safe-r")
	notR, err := manager.MkNot(r)
	pdrMust(t, err)
	result, err = solver.CheckAssumptionsAndFormula(nil, r, false, false)
	pdrMust(t, err)
	if result.Status != lbool.True {
		t.Fatal("temporary formula", result.Status)
	}
	result, err = solver.CheckAssumptions([]ast.ASTHandle{notR}, false, false)
	pdrMust(t, err)
	if result.Status != lbool.True {
		t.Fatal("temporary formula survived delayed scope", result.Status)
	}

	pdrMust(t, solver.Close())
	if _, err = solver.LevelCount(); !errors.Is(err, pdr.ErrPropSolverOwner) {
		t.Fatal("closed prop solver", err)
	}
	pdrMust(t, contexts.Close())
	pdrMust(t, command.Close())
}

func TestPropSolverExpandsArithmeticEqualityForSubsetCore(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 0)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	solver, err := pdr.NewPropSolver(manager, contexts, truth, "arithmetic-core")
	pdrMust(t, err)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	pdrMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	pdrMust(t, err)
	x, err := manager.MkFreshConst("arithmetic-core-x", integer)
	pdrMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	pdrMust(t, err)
	greaterEqual, err := manager.MkArithmeticGE(x, zero)
	pdrMust(t, err)
	less, err := manager.MkNot(greaterEqual)
	pdrMust(t, err)
	pdrMust(t, solver.AddFormula(less))
	equality, err := manager.MkEq(x, zero)
	pdrMust(t, err)

	result, err := solver.CheckAssumptions([]ast.ASTHandle{equality}, true, false)
	pdrMust(t, err)
	if result.Status != lbool.False || len(result.Core) != 1 || result.Core[0] != greaterEqual {
		t.Fatal("expanded arithmetic subset core", result.Status, result.Core, greaterEqual)
	}

	pdrMust(t, solver.Close())
	pdrMust(t, contexts.Close())
	pdrMust(t, command.Close())
}

func TestPropSolverExtractsFarkasTheoryCore(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 0)
	pdrMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	solver, err := pdr.NewPropSolver(manager, contexts, truth, "farkas-core")
	pdrMust(t, err)
	pdrMust(t, solver.SetUseFarkas(true))
	if enabled, err := solver.UseFarkas(); err != nil || !enabled {
		t.Fatal("Farkas core mode was not retained", enabled, err)
	}
	arithmetic, err := manager.RegisterArithmeticPlugin()
	pdrMust(t, err)
	realSort, err := manager.MkFamilySort(arithmetic, 0, nil)
	pdrMust(t, err)
	x, err := manager.MkFreshConst("farkas-core-x", realSort)
	pdrMust(t, err)
	y, err := manager.MkFreshConst("farkas-core-y", realSort)
	pdrMust(t, err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	pdrMust(t, err)
	minusOne, err := manager.ArithmeticRealNumeralText("-1")
	pdrMust(t, err)
	sum, err := manager.MkArithmeticAdd(x, y)
	pdrMust(t, err)
	lessEqual, err := manager.MkArithmeticLE(sum, minusOne)
	pdrMust(t, err)
	xNonnegative, err := manager.MkArithmeticGE(x, zero)
	pdrMust(t, err)
	yNonnegative, err := manager.MkArithmeticGE(y, zero)
	pdrMust(t, err)
	pdrMust(t, solver.AddFormula(lessEqual))

	result, err := solver.CheckAssumptions([]ast.ASTHandle{xNonnegative, yNonnegative}, true, false)
	pdrMust(t, err)
	if result.Status != lbool.False || len(result.Core) == 0 {
		t.Fatal("Farkas proof did not produce an arithmetic core", result.Status, result.Core)
	}
	if !result.FarkasCore {
		t.Fatal("Farkas mode silently fell back to the subset core")
	}
	for _, lemma := range result.Core {
		if _, err = manager.GetKind(lemma); err != nil {
			t.Fatal("Farkas core did not retain its result", err)
		}
	}
	result, err = solver.CheckAssumptionsWithConsequences([]ast.ASTHandle{xNonnegative, yNonnegative}, false)
	pdrMust(t, err)
	if result.Status != lbool.False || !result.FarkasCore || len(result.Consequences) == 0 {
		t.Fatal("Farkas consequence forwarding", result.Status, result.FarkasCore, result.Consequences)
	}
	for _, consequence := range result.Consequences {
		if _, err = manager.GetKind(consequence); err != nil {
			t.Fatal("Farkas consequence was not retained", err)
		}
	}
	pdrMust(t, solver.SetSubsetBasedCore(true))
	result, err = solver.CheckAssumptions([]ast.ASTHandle{xNonnegative, yNonnegative}, true, false)
	pdrMust(t, err)
	if result.Status != lbool.False || result.FarkasCore || len(result.Core) != 2 {
		t.Fatal("forced subset-core mode", result.Status, result.FarkasCore, result.Core)
	}
	result, err = solver.CheckAssumptions([]ast.ASTHandle{xNonnegative, yNonnegative}, true, false)
	pdrMust(t, err)
	if result.Status != lbool.False || !result.FarkasCore {
		t.Fatal("subset-core override was not one-shot", result.Status, result.FarkasCore)
	}

	pdrMust(t, solver.Close())
	pdrMust(t, contexts.Close())
	pdrMust(t, command.Close())
}

func TestPropSolverFarkasCoreRestoresSafeProxy(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 0)
	pdrMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	solver, err := pdr.NewPropSolver(manager, contexts, truth, "farkas-proxy")
	pdrMust(t, err)
	pdrMust(t, solver.SetUseFarkas(true))
	p := pdrBool(t, manager, "farkas-proxy-p")
	q := pdrBool(t, manager, "farkas-proxy-q")
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	notQ, err := manager.MkNot(q)
	pdrMust(t, err)
	disjunction, err := manager.MkOr(p, q)
	pdrMust(t, err)
	pdrMust(t, solver.AddFormula(notP))
	pdrMust(t, solver.AddFormula(notQ))
	result, err := solver.CheckAssumptionsWithConsequences([]ast.ASTHandle{disjunction}, false)
	pdrMust(t, err)
	// safe_assumptions::elim_proxies replaces the proxy by true without
	// simplifying, so the B-set definition (or (not proxy) p q) becomes
	// (or (not true) (or p q)). Consequences are the learner's raw B facts.
	if result.Status != lbool.False || !result.FarkasCore || len(result.Core) != 1 {
		t.Fatal("Farkas proxy elimination", result.Status, result.FarkasCore, result.Core)
	}
	count, err := manager.GetNumArgs(result.Core[0])
	pdrMust(t, err)
	if count != 2 {
		t.Fatal("Farkas core is not the eliminated definition", count)
	}
	first, err := manager.GetArg(result.Core[0], 0)
	pdrMust(t, err)
	negated, inner, err := manager.IsNot(first)
	pdrMust(t, err)
	second, err := manager.GetArg(result.Core[0], 1)
	pdrMust(t, err)
	if !negated || !manager.IsTrue(inner) || second != disjunction {
		t.Fatal("Farkas core is not (or (not true) (or p q))")
	}
	if len(result.Consequences) != 2 {
		t.Fatal("Farkas consequences", result.Consequences)
	}
	pdrMust(t, solver.Close())
	pdrMust(t, contexts.Close())
	pdrMust(t, command.Close())
}

func TestPropSolverExpandsDatatypeEqualityForSubsetCore(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 0)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	solver, err := pdr.NewPropSolver(manager, contexts, truth, "datatype-core")
	pdrMust(t, err)
	name, err := manager.Symbols.Intern("PDRBox")
	pdrMust(t, err)
	constructorName, err := manager.Symbols.Intern("pdr-box")
	pdrMust(t, err)
	recognizerName, err := manager.Symbols.Intern("pdr-is-box")
	pdrMust(t, err)
	accessorName, err := manager.Symbols.Intern("pdr-unbox")
	pdrMust(t, err)
	otherName, err := manager.Symbols.Intern("pdr-other")
	pdrMust(t, err)
	otherRecognizerName, err := manager.Symbols.Intern("pdr-is-other")
	pdrMust(t, err)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	sorts, err := manager.MkDatatypes([]ast.DatatypeDefinition{{
		Name: name,
		Constructors: []ast.DatatypeConstructorDefinition{{
			Name:       constructorName,
			Recognizer: recognizerName,
			Accessors:  []ast.DatatypeAccessorDefinition{{Name: accessorName, Range: boolean}},
		}, {Name: otherName, Recognizer: otherRecognizerName}},
	}})
	pdrMust(t, err)
	constructors, err := manager.DatatypeConstructors(sorts[0])
	pdrMust(t, err)
	term, err := manager.MkFreshConst("datatype-core-value", sorts[0])
	pdrMust(t, err)
	value, err := manager.MkApp(constructors[0], []ast.ASTHandle{truth})
	pdrMust(t, err)
	recognizer, err := manager.DatatypeConstructorRecognizer(constructors[0])
	pdrMust(t, err)
	recognized, err := manager.MkApp(recognizer, []ast.ASTHandle{term})
	pdrMust(t, err)
	notRecognized, err := manager.MkNot(recognized)
	pdrMust(t, err)
	pdrMust(t, solver.AddFormula(notRecognized))
	equality, err := manager.MkEq(term, value)
	pdrMust(t, err)

	result, err := solver.CheckAssumptions([]ast.ASTHandle{equality}, true, false)
	pdrMust(t, err)
	if result.Status != lbool.False || len(result.Core) != 1 {
		t.Fatal("expanded datatype subset core", result.Status, result.Core)
	}
	coreDeclaration, err := manager.GetDecl(result.Core[0])
	pdrMust(t, err)
	coreConstructor, matched, err := manager.DatatypeRecognizerConstructor(coreDeclaration)
	pdrMust(t, err)
	coreTerm, err := manager.GetArg(result.Core[0], 0)
	pdrMust(t, err)
	if !matched || coreConstructor != constructors[0] || coreTerm != term {
		t.Fatal("expanded datatype recognizer", matched, coreConstructor, coreTerm)
	}

	pdrMust(t, solver.Close())
	pdrMust(t, contexts.Close())
	pdrMust(t, command.Close())
}

func TestPropSolverExpandsBitVectorEqualityForSubsetCore(t *testing.T) {
	for _, test := range []struct {
		name  string
		width uint32
		bit   uint32
		set   bool
	}{
		{name: "set", width: 3, bit: 0, set: true},
		{name: "clear", width: 3, bit: 1, set: false},
		{name: "wide-clear", width: 65, bit: 64, set: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			command, manager, contexts := pdrContextFixture(t, 0)
			truth, err := manager.MkTrue()
			pdrMust(t, err)
			solver, err := pdr.NewPropSolver(manager, contexts, truth, "bit-vector-core")
			pdrMust(t, err)
			value, err := manager.BitVectorNumeralUint64(5, test.width)
			pdrMust(t, err)
			sort, err := manager.GetSort(value)
			pdrMust(t, err)
			term, err := manager.MkFreshConst("bit-vector-core-value", sort)
			pdrMust(t, err)
			one, err := manager.BitVectorNumeralUint64(1, 1)
			pdrMust(t, err)
			extract, err := manager.MkBitVectorExtract(test.bit, test.bit, term)
			pdrMust(t, err)
			bitEquality, err := manager.MkEq(one, extract)
			pdrMust(t, err)
			background := bitEquality
			if test.set {
				background, err = manager.MkNot(bitEquality)
			}
			pdrMust(t, err)
			pdrMust(t, solver.AddFormula(background))
			equality, err := manager.MkEq(term, value)
			pdrMust(t, err)

			result, err := solver.CheckAssumptions([]ast.ASTHandle{equality}, true, false)
			pdrMust(t, err)
			if result.Status != lbool.False || len(result.Core) != 1 {
				t.Fatal("expanded bit-vector subset core", result.Status, result.Core)
			}
			core := result.Core[0]
			negated, argument, err := manager.IsNot(core)
			pdrMust(t, err)
			if negated {
				core = argument
			}
			if negated == test.set {
				t.Fatal("expanded bit-vector polarity", negated, test.set)
			}
			kind, err := manager.GetDeclKind(core)
			pdrMust(t, err)
			coreOne, err := manager.GetArg(core, 0)
			pdrMust(t, err)
			coreExtract, err := manager.GetArg(core, 1)
			pdrMust(t, err)
			high, err := manager.BitVectorExtractHigh(coreExtract)
			pdrMust(t, err)
			low, err := manager.BitVectorExtractLow(coreExtract)
			pdrMust(t, err)
			coreTerm, err := manager.GetArg(coreExtract, 0)
			pdrMust(t, err)
			if kind != ast.ASTOpEq || coreOne != one || high != test.bit || low != test.bit || coreTerm != term {
				t.Fatal("expanded bit-vector literal", kind, coreOne, high, low, coreTerm)
			}

			pdrMust(t, solver.Close())
			pdrMust(t, contexts.Close())
			pdrMust(t, command.Close())
		})
	}
}
