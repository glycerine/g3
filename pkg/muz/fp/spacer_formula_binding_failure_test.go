package fp

// Source: src/muz/transforms/dl_mk_slice.cpp:344-355. The substitution's
// expr_ref_vector owns every fresh original-column variable during expansion.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSlicedFormulaFailureReleasesTemporaryBindings(t *testing.T) {
	failed, completed := false, false
	for stop := int64(0); stop < 128 && !completed; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, _ := newPDRLifecycleServices(t)
			boolean, err := m.MkBoolSort()
			fragmentMust(t, err)
			name, err := m.Symbols.Intern("slice-binding-failure")
			fragmentMust(t, err)
			declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean, boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			declarationPin, err := m.NewRef(declaration)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, m.DelRef(&declarationPin)) }()
			x, err := m.MkVar(0, boolean)
			fragmentMust(t, err)
			pin, err := m.NewRef(x)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, m.DelRef(&pin)) }()
			before := m.Stats()
			convert := func() (ast.ASTRef, error) {
				source, err := m.NewRef(x)
				fragmentMust(t, err)
				return expandSlicedFormula(m, map[ast.ASTHandle][]bool{declaration: {true, false, true, false}}, declaration, source)
			}
			// Make the consumed input reference outside the allocation campaign.
			source, err := m.NewRef(x)
			fragmentMust(t, err)
			m.Budget().FailAfter(stop)
			result, cause := expandSlicedFormula(m, map[ast.ASTHandle][]bool{declaration: {true, false, true, false}}, declaration, source)
			m.Budget().FailAfter(-1)
			if cause != nil {
				failed = true
				if result != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("unexpected conversion failure", result, cause)
				}
				var cleanup memory.ConstructionFailure
				if errors.As(cause, &cleanup) {
					fragmentMust(t, cleanup.Cleanup())
				}
			} else {
				completed = true
				fragmentMust(t, m.DelRef(&result))
			}
			after := m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
				t.Fatal("conversion lost temporary variable owners", before.LiveNodes, after.LiveNodes, before.LiveReferences, after.LiveReferences, cause)
			}
			retry, err := convert()
			fragmentMust(t, err)
			h, err := m.RefGet(retry)
			fragmentMust(t, err)
			index, err := m.GetVarIndex(h)
			fragmentMust(t, err)
			if index != 1 {
				t.Fatal("retry did not restore the original argument column", index)
			}
			fragmentMust(t, m.DelRef(&retry))
			after = m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
				t.Fatal("retry lost temporary variable owners", before, after)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !failed || !completed {
		t.Fatal("allocation campaign did not cover refusal and completion", failed, completed)
	}
}
