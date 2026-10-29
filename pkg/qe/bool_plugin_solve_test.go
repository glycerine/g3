package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type boolPluginFixture struct {
	*fmFixture
	plugin   *BoolPlugin
	context  *SolverContext
	contains *ast.ContainsApp
	x, q     ast.ASTHandle
}

func newBoolPluginFixture(t *testing.T) *boolPluginFixture {
	f := &boolPluginFixture{fmFixture: newFMFixture(t)}
	f.x = f.constant("bool-solve-x", f.boolean)
	f.q = f.constant("bool-solve-q", f.boolean)
	var err error
	f.contains, err = ast.NewContainsApp(f.r.manager, f.x)
	fmMust(t, err)
	f.context, err = NewSolverContext(f.r.manager, []ast.ASTHandle{f.x})
	fmMust(t, err)
	f.plugin, err = NewBoolPlugin(f.context)
	fmMust(t, err)
	t.Cleanup(func() {
		fmMust(t, f.plugin.Close())
		fmMust(t, f.context.Close())
		fmMust(t, f.contains.Close())
	})
	return f
}

func TestBoolPluginSolveUnitsInSourceOrder(t *testing.T) {
	for _, truth := range []bool{true, false} {
		t.Run(map[bool]string{true: "positive", false: "negative"}[truth], func(t *testing.T) {
			f := newBoolPluginFixture(t)
			unit := f.x
			if !truth {
				unit = f.own(f.r.manager.MkNot(f.x))
			}
			formula := f.own(f.r.manager.MkAnd(unit, f.q))
			conjs, err := NewConjunctionEnum(f.r.manager, formula)
			fmMust(t, err)
			defer func() { fmMust(t, conjs.Close()) }()
			atoms, err := NewBoolAtomIndex(f.r.manager, []ast.ASTHandle{f.x}, nil)
			fmMust(t, err)
			defer func() { fmMust(t, atoms.Close()) }()
			result, solved, err := f.plugin.Solve(f.contains, formula, conjs, atoms)
			fmMust(t, err)
			if !solved || result == nil {
				t.Fatal("unit was not solved")
			}
			defer func() { fmMust(t, result.Close()) }()
			h, err := f.r.manager.RefGet(result.Formula)
			fmMust(t, err)
			if found, cause := f.contains.Check(h); cause != nil || found {
				t.Fatal("unit result retained variable", found, cause)
			}
			definition, err := f.r.manager.RefGet(result.Definition)
			fmMust(t, err)
			if f.r.manager.IsTrue(definition) != truth {
				t.Fatal("unit definition polarity")
			}
			copyOwner := *result
			if err = copyOwner.Close(); err == nil {
				t.Fatal("copied elimination owner accepted")
			}
		})
	}
}

func TestBoolAtomIndexCopyCloseAndReuseGuards(t *testing.T) {
	f := newBoolPluginFixture(t)
	atoms, err := NewBoolAtomIndex(f.r.manager, []ast.ASTHandle{f.x, f.x}, nil)
	fmMust(t, err)
	copyOwner := *atoms
	if _, err = copyOwner.Positive(f.x); err == nil {
		t.Fatal("copied atom index accepted")
	}
	fmMust(t, atoms.Close())
	fmMust(t, atoms.Close())
	if _, err = atoms.Positive(f.x); err == nil {
		t.Fatal("closed atom index accepted")
	}
}

func TestBoolPluginPolarizedSolveWeightAndRejection(t *testing.T) {
	f := newBoolPluginFixture(t)
	formula := f.own(f.r.manager.MkOr(f.x, f.q))
	conjs, err := NewConjunctionEnum(f.r.manager, formula)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	atoms, err := NewBoolAtomIndex(f.r.manager, []ast.ASTHandle{f.x, f.x}, []ast.ASTHandle{f.q})
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	if weight, cause := f.plugin.Weight(f.contains, atoms); cause != nil || weight != 0 {
		t.Fatal("one-polarity weight", weight, cause)
	}
	result, solved, err := f.plugin.Solve(f.contains, formula, conjs, atoms)
	fmMust(t, err)
	if !solved {
		t.Fatal("positive occurrence was not solved")
	}
	fmMust(t, result.Close())

	both, err := NewBoolAtomIndex(f.r.manager, []ast.ASTHandle{f.x}, []ast.ASTHandle{f.x})
	fmMust(t, err)
	defer func() { fmMust(t, both.Close()) }()
	if weight, cause := f.plugin.Weight(f.contains, both); cause != nil || weight != 3 {
		t.Fatal("mixed-polarity weight", weight, cause)
	}
	result, solved, err = f.plugin.Solve(f.contains, formula, conjs, both)
	fmMust(t, err)
	if solved || result != nil {
		t.Fatal("mixed polarity accepted")
	}

	foreign := f.own(f.r.manager.MkEq(f.x, f.q))
	blocked, err := NewBoolAtomIndex(f.r.manager, []ast.ASTHandle{f.x, foreign}, nil)
	fmMust(t, err)
	defer func() { fmMust(t, blocked.Close()) }()
	result, solved, err = f.plugin.Solve(f.contains, foreign, conjs, blocked)
	fmMust(t, err)
	if solved || result != nil {
		t.Fatal("foreign atom occurrence accepted")
	}
}

func TestBoolPluginPolarizedAbsentVariableProducesFalseRealizer(t *testing.T) {
	f := newBoolPluginFixture(t)
	conjs, err := NewConjunctionEnum(f.r.manager, f.q)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	atoms, err := NewBoolAtomIndex(f.r.manager, []ast.ASTHandle{f.q}, nil)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	result, solved, err := f.plugin.Solve(f.contains, f.q, conjs, atoms)
	fmMust(t, err)
	if !solved {
		t.Fatal("absent variable was not discharged")
	}
	defer func() { fmMust(t, result.Close()) }()
	definition, err := f.r.manager.RefGet(result.Definition)
	fmMust(t, err)
	if !f.r.manager.IsFalse(definition) {
		t.Fatal("absent variable realizer is not false")
	}
}
