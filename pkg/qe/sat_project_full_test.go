package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSATFullProjectionScalarAndRefusal(t *testing.T) {
	for _, mode := range []string{"boolean", "integer", "bitvector", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			k := qsatKernelOwner(t, f)
			solver, err := k.solver()
			fmMust(t, err)
			c, err := newSATSolverContext(m, solver, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, c.Close()) }()
			rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
			fmMust(t, err)
			defer func() { fmMust(t, rw.Close()) }()
			q := f.constant("full-projection-q", f.boolean)
			var variable, literal ast.ASTHandle
			switch mode {
			case "boolean":
				variable = f.constant("full-projection-b", f.boolean)
				literal = variable
			case "integer":
				variable = f.constant("full-projection-i", f.integer)
				literal = f.own(m.MkEq(variable, f.number("3")))
			case "bitvector":
				sort := f.own(m.BitVectorSort(2))
				variable = f.constant("full-projection-bv", sort)
				literal = f.own(m.MkEq(variable, f.own(m.BitVectorNumeralUint64(1, 2))))
			default:
				sort := f.own(m.MkUninterpretedSort(symbol.Numerical(99021), nil))
				variable = f.constant("full-projection-u", sort)
				other := f.constant("full-projection-other", sort)
				literal = f.own(m.MkEq(variable, other))
			}
			formula := f.own(m.MkAnd(literal, q))
			fmMust(t, c.addVariable(variable))
			fmMust(t, c.initializeFormula(formula))
			before, err := m.RefGet(c.formula)
			fmMust(t, err)
			err = c.projectFull(0, rw, f.r.config)
			if mode == "unsupported" {
				if err == nil {
					t.Fatal("unsupported sort accepted")
				}
				// mk_exists rebinds m_fml before qelim fails; the
				// variable is not eliminated.
				_ = before
				if len(c.variables) != 1 {
					t.Fatal("failed projection changed context")
				}
				return
			}
			fmMust(t, err)
			after, err := m.RefGet(c.formula)
			fmMust(t, err)
			if after != q || len(c.variables) != 0 {
				t.Fatalf("projection = %v, want free Boolean condition", after)
			}
		})
	}
}
