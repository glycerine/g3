package spacer_test

// Source: src/muz/spacer/spacer_manager.cpp state conversion, background,
// unique numbering, three solver pools, and PropSolver construction.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerManagerBuildsStateAndPropSolverServices(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	owner, err := spacer.NewManager(manager, nil, 2, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	spacerMust(t, err)
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	name, err := manager.Symbols.Intern("manager-state")
	spacerMust(t, err)
	prefix, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	spacerMust(t, err)
	current, err := owner.NewPredicate(prefix)
	spacerMust(t, err)
	old0, err := owner.OldPredicate(prefix, 0)
	spacerMust(t, err)
	old1, err := owner.OldPredicate(prefix, 1)
	spacerMust(t, err)
	if converted, err := owner.OldToNew(old0, 0); err != nil || converted != current {
		t.Fatal("old-to-new", converted, current, err)
	}
	if converted, err := owner.OldToOld(old0, 0, 1); err != nil || converted != old1 {
		t.Fatal("old-to-old", converted, old1, err)
	}
	currentAtom, err := manager.MkApp(current, nil)
	spacerMust(t, err)
	old0Atom, err := manager.MkApp(old0, nil)
	spacerMust(t, err)
	old1Atom, err := manager.MkApp(old1, nil)
	spacerMust(t, err)
	mixedOld, err := manager.MkAnd(old0Atom, old1Atom)
	spacerMust(t, err)
	if isNew, err := owner.IsNewFormula(currentAtom); err != nil || !isNew {
		t.Fatal("new formula classification", isNew, err)
	}
	if isOld, err := owner.IsOldFormula(old0Atom); err != nil || !isOld {
		t.Fatal("old formula classification", isOld, err)
	}
	if homogeneous, err := owner.IsHomogeneousFormula(mixedOld); err != nil || homogeneous {
		t.Fatal("mixed old formula classification", homogeneous, err)
	}
	indices, err := owner.CollectStateIndices(mixedOld)
	spacerMust(t, err)
	if len(indices) != 2 || indices[0] != 0 || indices[1] != 1 {
		t.Fatal("manager state indices", indices)
	}
	if different, err := owner.HaveDifferentStateKinds(old0, old1); err != nil || !different {
		t.Fatal("different state kinds", different, err)
	}
	filtered, err := owner.FilterOldAtoms([]ast.ASTHandle{currentAtom, old0Atom, old1Atom, mixedOld}, 0)
	spacerMust(t, err)
	if len(filtered) != 1 || filtered[0] != old0Atom {
		t.Fatal("manager old-atom filter", filtered)
	}
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	equality, err := manager.MkEq(truth, currentAtom)
	spacerMust(t, err)
	state, value, found, err := owner.TryStateAndValueFromAtom(equality)
	spacerMust(t, err)
	if !found || state != currentAtom || value != truth {
		t.Fatal("reversed state equality", state, value, found)
	}
	negated, err := manager.MkNot(currentAtom)
	spacerMust(t, err)
	state, value, found, err = owner.TryStateAndValueFromAtom(negated)
	spacerMust(t, err)
	if !found || state != currentAtom || !manager.IsFalse(value) {
		t.Fatal("negated state atom", state, value, found)
	}
	declaration, found, err := owner.TryStateDeclarationFromAtom(currentAtom)
	spacerMust(t, err)
	if !found || declaration != current {
		t.Fatal("state declaration", declaration, found)
	}
	skolem, err := spacer.MkZKConst(manager, 7, boolean)
	spacerMust(t, err)
	skolemFormula, err := manager.MkAnd(skolem, currentAtom)
	spacerMust(t, err)
	skolemValues, err := spacer.FindZKConstants(manager, skolemFormula)
	spacerMust(t, err)
	if len(skolemValues) != 1 || skolemValues[0] != skolem {
		t.Fatal("Skolem collection", skolemValues)
	}
	if has, err := spacer.HasZKConst(manager, currentAtom); err != nil || has {
		t.Fatal("ordinary state classified as Skolem-bearing", has, err)
	}
	modelCore, err := model.NewModelCore(manager)
	spacerMust(t, err)
	spacerMust(t, modelCore.RegisterConstant(old0, truth))
	modelText, err := owner.ModelString(modelCore)
	spacerMust(t, err)
	if !strings.Contains(modelText, "manager-state_0 -> true") {
		t.Fatal("manager model display", modelText)
	}
	spacerMust(t, modelCore.Close())
	if first, err := owner.Unique(); err != nil || first != 0 {
		t.Fatal("first unique", first, err)
	}
	if second, err := owner.Unique(); err != nil || second != 1 {
		t.Fatal("second unique", second, err)
	}
	prop, err := owner.NewPropSolver("manager-prop", spacer.PropSolverConfig{
		Itp: spacer.ItpSolverConfig{Params: globals.Context, Globals: globals},
	})
	spacerMust(t, err)
	p := spacerBool(t, manager, "manager-prop-p")
	notP, err := manager.MkNot(p)
	spacerMust(t, err)
	spacerMust(t, prop.Assert(notP))
	result, err := prop.CheckAssumptions([]ast.ASTHandle{p}, nil, spacer.PropCheckOptions{NeedCore: true})
	spacerMust(t, err)
	if result.Status != lbool.False || len(result.Core) != 1 {
		t.Fatal("manager PropSolver core", result.Status, len(result.Core))
	}
	spacerMust(t, result.Close(manager))
	spacerMust(t, prop.Close())
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}

func TestSpacerSkolemScanIncludesQuantifierAnnotationsAndSourceOrder(t *testing.T) {
	command, m, owner := lemmaOwnerFixture(t)
	defer func() { spacerMust(t, command.Close()) }()
	defer func() { spacerMust(t, owner.Close()) }()
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			spacerMust(t, m.DelRef(&pins[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		spacerMust(t, err)
		ref, err := m.NewRef(h)
		spacerMust(t, err)
		pins = append(pins, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	k0 := own(spacer.MkZKConst(m, 0, boolean))
	k1 := own(spacer.MkZKConst(m, 1, boolean))
	k2 := own(spacer.MkZKConst(m, 2, boolean))
	variable := own(m.MkVar(0, boolean))
	name, err := m.Symbols.Intern("skolem-annotation-body")
	spacerMust(t, err)
	property := own(m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	body := own(m.MkApp(property, []ast.ASTHandle{variable}))
	name, err = m.Symbols.Intern("skolem-annotation-term")
	spacerMust(t, err)
	declaration := own(m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{}))
	annotation := own(m.MkApp(declaration, []ast.ASTHandle{variable, k0}))
	pattern := own(m.MkPattern([]ast.ASTHandle{annotation}))
	for _, noPattern := range []bool{false, true} {
		spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body}
		if noPattern {
			spec.NoPatterns = []ast.ASTHandle{annotation}
		} else {
			spec.Patterns = []ast.ASTHandle{pattern}
		}
		quantifier := own(m.MkQuantifier(spec))
		found, err := spacer.HasZKConst(m, quantifier)
		spacerMust(t, err)
		if !found {
			t.Error("skolem in quantifier annotation was missed", noPattern)
		}
		constants, err := spacer.FindZKConstants(m, quantifier)
		spacerMust(t, err)
		if len(constants) != 1 || constants[0] != k0 {
			t.Error("quantifier annotation skolem collection", noPattern, constants)
		}
	}
	inner := own(m.MkAnd(k0, k1))
	root := own(m.MkAndArray([]ast.ASTHandle{k1, inner, k2}))
	constants, err := spacer.FindZKConstants(m, root)
	spacerMust(t, err)
	expected := []ast.ASTHandle{k1, k0, k2}
	if len(constants) != len(expected) {
		t.Fatal("shared skolem was repeated", constants)
	}
	for i := range expected {
		if constants[i] != expected[i] {
			t.Fatal("skolem scan does not follow source child order", constants)
		}
	}
}
