package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacySkolemProjectionConstructionFailureOwnership(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 2048 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := NewLegacyProjection(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, p.Close()) }()
			x := f.constant("skolem-failure-x", f.integer)
			one := f.own(m.ArithmeticIntegerNumeralText("1"))
			product := f.own(m.MkArithmeticMul(x, x))
			formula := f.own(m.MkArithmeticGT(product, one))
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			decl, err := m.GetDecl(x)
			fmMust(t, err)
			fmMust(t, md.Core().RegisterConstant(decl, one))
			mapping, err := ast.NewASTExprMapWithProofs(m, false)
			fmMust(t, err)
			defer func() { fmMust(t, mapping.Close()) }()
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			result, err := p.ProjectSkolem(md.Core(), []ast.ASTHandle{x}, formula, mapping)
			m.Budget().FailAfter(-1)
			if err == nil {
				remaining, cause := result.Variables()
				fmMust(t, cause)
				root, cause := result.Formula()
				fmMust(t, cause)
				occurs, cause := m.OccursExpr(x, root)
				fmMust(t, cause)
				if len(remaining) != 1 || remaining[0] != x || !occurs {
					t.Fatal("mapped nonlinear projection lost its residual owner", remaining, occurs)
				}
			}
			if result != nil {
				fmMust(t, result.Close())
			}
			if err == nil {
				succeeded = true
			} else {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					fmMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected projection failure", err)
				}
				failures++
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("failed projection kept result references", got, before)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing construction failure/success frontier", failures, succeeded)
	}
}
