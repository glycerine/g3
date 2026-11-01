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

func TestLegacyModelSubstitutionConstructionFailureOwnershipAndRetry(t *testing.T) {
	stops := make([]int64, 64)
	for i := range stops {
		stops[i] = int64(i)
	}
	for stop := int64(64); stop <= 4096; stop *= 2 {
		stops = append(stops, stop)
	}
	succeeded, failures := false, 0
	for _, stop := range stops {
		if succeeded {
			break
		}
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := NewLegacyProjection(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, p.Close()) }()
			b := f.constant("substitution-b", f.boolean)
			c := f.constant("substitution-c", f.boolean)
			truth := f.own(m.MkTrue())
			formula := f.own(m.MkOr(b, c))
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			decl, err := m.GetDecl(b)
			fmMust(t, err)
			fmMust(t, md.Core().RegisterConstant(decl, truth))
			fmMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			before := m.Stats().LiveReferences
			bCount, err := m.GetRefCount(b)
			fmMust(t, err)
			m.Budget().FailAfter(stop)
			result, err := p.substituteModelValues(md.Core(), formula, []ast.ASTHandle{b})
			m.Budget().FailAfter(-1)
			if result != (ast.ASTRef{}) {
				fmMust(t, m.DelRef(&result))
			}
			if err != nil {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					fmMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected substitution failure", err)
				}
				failures++
			} else {
				succeeded = true
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("substitution kept temporary reference slots", got, before)
			}
			count, err := m.GetRefCount(b)
			fmMust(t, err)
			if count != bCount {
				t.Fatal("substitution kept a replacement-source reference", count, bCount)
			}
			if mode, err := m.ProofMode(); err != nil || mode != ast.ASTProofEnabled {
				t.Fatal("substitution changed proof mode", mode, err)
			}
			retry, err := p.substituteModelValues(md.Core(), formula, []ast.ASTHandle{b})
			fmMust(t, err)
			root, err := m.RefGet(retry)
			fmMust(t, err)
			op, err := m.GetDeclKind(root)
			fmMust(t, err)
			left, err := m.GetArg(root, 0)
			fmMust(t, err)
			right, err := m.GetArg(root, 1)
			fmMust(t, err)
			if op != ast.ASTOpOr || left != truth || right != c {
				t.Fatal("substitution must preserve the unsimplified source Boolean gate")
			}
			fmMust(t, m.DelRef(&retry))
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("retry leaked temporary references", got, before)
			}
			cDecl, err := m.GetDecl(c)
			fmMust(t, err)
			completed, err := md.Core().HasConstantInterpretation(cDecl)
			fmMust(t, err)
			if completed {
				t.Fatal("substitution completed an unrelated model constant")
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing substitution construction failure/success frontier", failures, succeeded)
	}
}

func TestLegacyModelSubstitutionCompletesOnlyProjectedBoolean(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	b := f.constant("completion-b", f.boolean)
	c := f.constant("completion-c", f.boolean)
	formula := f.own(m.MkOr(b, c))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	result, err := p.substituteModelValues(md.Core(), formula, []ast.ASTHandle{b})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	root, err := m.RefGet(result)
	fmMust(t, err)
	op, err := m.GetDeclKind(root)
	fmMust(t, err)
	left, err := m.GetArg(root, 0)
	fmMust(t, err)
	right, err := m.GetArg(root, 1)
	fmMust(t, err)
	// basic_decl_plugin::get_some_value chooses false; expr_safe_replace
	// leaves the Boolean gate for the driver's subsequent theory rewrite.
	if op != ast.ASTOpOr || !m.IsFalse(left) || right != c {
		t.Fatal("completed Boolean substitution did not preserve source policy")
	}
	for _, variable := range []ast.ASTHandle{b, c} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		completed, err := md.Core().HasConstantInterpretation(decl)
		fmMust(t, err)
		if completed != (variable == b) {
			t.Fatal("model completion affected the wrong constant", variable, completed)
		}
	}
}
