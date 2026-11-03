package fp

// Source: dl_mk_bit_blast.cpp:92-107 owns arg/proj before inserting them
// into the model converter's substitution.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestBitBlastModelProjectionReferenceFailureAndRetry(t *testing.T) {
	for _, spare := range []int{0, 1} {
		t.Run(fmt.Sprint(spare), func(t *testing.T) {
			testBitBlastModelProjectionReferenceFailure(t, spare)
		})
	}
}

func testBitBlastModelProjectionReferenceFailure(t *testing.T, spare int) {
	t.Helper()
	f := newQueryVariablesOwnershipFixture(t)
	m := f.m
	boolean := f.own(m.MkBoolSort())
	bits := f.own(m.BitVectorSort(2))
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		fragmentMust(t, err)
		return f.own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
	}
	original := declare("model-projection-original", []ast.ASTHandle{bits})
	derived := declare("model-projection-derived", []ast.ASTHandle{boolean, boolean})
	// Prime only the persistent bit2bool declarations with a different input.
	// The converter's #0 BV argument and projections remain cold.
	other := f.own(m.MkVar(7, bits))
	for bit := uint32(0); bit < 2; bit++ {
		projection, err := m.MkBitVectorBit2Bool(bit, other)
		fragmentMust(t, err)
		ref, err := m.NewRef(projection)
		fragmentMust(t, err)
		fragmentMust(t, m.DelRef(&ref))
	}
	body := f.own(m.MkVar(0, boolean))
	source, err := model.NewModel(m)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, source.Close()) })
	interpretation, err := model.NewFunctionInterpretation(m, 2)
	fragmentMust(t, err)
	fragmentMust(t, interpretation.SetElse(body))
	fragmentMust(t, source.Core().RegisterFunction(derived, interpretation))
	var pressure []ast.ASTRef
	t.Cleanup(func() { fragmentMust(t, closeASTRefs(m, pressure)) })
	m.Budget().FailAfter(0)
	for {
		ref, cause := m.NewRef(ast.ASTHandle{})
		if cause != nil {
			m.Budget().FailAfter(-1)
			if !errors.Is(cause, arena.ErrInjectedFailure) || ref != (ast.ASTRef{}) {
				t.Fatal("reference page frontier", ref, cause)
			}
			break
		}
		pressure = append(pressure, ref)
	}
	for i := 0; i < spare; i++ {
		last := len(pressure) - 1
		fragmentMust(t, m.DelRef(&pressure[last]))
		pressure = pressure[:last]
	}
	before := contextDisplayLiveCounts(m.Stats())
	m.Budget().FailAfter(0)
	cause := f.e.expandSpacerBitBlastedFunction(source.Core(), original, derived)
	m.Budget().FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) {
		t.Fatal("converter did not hit the cold reference frontier", cause)
	}
	if after := contextDisplayLiveCounts(m.Stats()); after != before {
		t.Fatal("failed bit-blast model conversion lost AST owners", before, after)
	}
	still, err := source.Core().FunctionInterpretation(derived)
	fragmentMust(t, err)
	if still != interpretation {
		t.Fatal("failed conversion changed its source interpretation")
	}
	if fresh, err := source.Core().FunctionInterpretation(original); err != nil || fresh != nil {
		t.Fatal("failed conversion published its original interpretation", fresh, err)
	}
	fragmentMust(t, f.e.expandSpacerBitBlastedFunction(source.Core(), original, derived))
	rebuilt, err := source.Core().FunctionInterpretation(original)
	fragmentMust(t, err)
	actual, err := rebuilt.Else()
	fragmentMust(t, err)
	argument := f.own(m.MkVar(0, bits))
	want := f.own(m.MkBitVectorBit2Bool(0, argument))
	if actual != want {
		t.Fatal("retry changed the original predicate's bit meaning", actual, want)
	}
	if stale, err := source.Core().FunctionInterpretation(derived); err != nil || stale != nil {
		t.Fatal("retry did not remove the derived predicate", stale, err)
	}
}
