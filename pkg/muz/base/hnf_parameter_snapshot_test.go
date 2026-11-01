package base_test

// Source: hnf::imp owns quantifier_hoister, whose bool_rewriter captures
// the rewriter module parameters when constructed and survives hnf::reset.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornNormalizerCapturesHoisterGlobalParameters(t *testing.T) {
	f := newRuleSetFixture(t, "Q", "R", "H")
	m := f.manager
	globals, err := f.context.GlobalParameters()
	ruleMust(t, err)
	ruleMust(t, globals.SourceSetName("rewriter.flat", "false"))
	normalizer, err := base.NewHornNormalizer(m, "snapshot")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, normalizer.Close()) })
	var pins []ast.ASTRef
	defer func() {
		for i := len(pins) - 1; i >= 0; i-- {
			ruleMust(t, m.DelRef(&pins[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		ruleMust(t, err)
		ref, err := m.NewRef(h)
		ruleMust(t, err)
		pins = append(pins, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	variable := own(m.MkVar(0, boolean))
	name, err := m.Symbols.Intern("parameter-snapshot-predicate")
	ruleMust(t, err)
	predicate := own(m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	atom := own(m.MkApp(predicate, []ast.ASTHandle{variable}))
	quantified := own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: atom}))
	inner := own(m.MkOr(quantified, f.app["Q"]))
	outer := own(m.MkOr(inner, f.app["R"]))
	formula := own(m.MkImplies(outer, f.app["H"]))
	// A collected outer prefix makes source hnf attach the pull-quant proof
	// after gate rewriting; otherwise its debug modus-ponens precondition fails.
	formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: formula}))
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	original := own(m.MkProofAsserted(formula))
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	check := func(n *base.HornNormalizer, expectedSplits int) {
		t.Helper()
		result, err := n.NormalizeWithProof(formula, original)
		ruleMust(t, err)
		defer func() { ruleMust(t, result.Close()) }()
		if len(result.Formulas) != 3 {
			t.Fatal("expected three Horn clauses", len(result.Formulas))
		}
		seen := make(map[ast.ASTHandle]bool)
		splits := 0
		var todo []ast.ASTHandle
		for _, ref := range result.Proofs {
			p, err := m.RefGet(ref)
			ruleMust(t, err)
			todo = append(todo, p)
		}
		for len(todo) != 0 {
			p := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if seen[p] {
				continue
			}
			seen[p] = true
			kind, err := m.GetDeclKind(p)
			ruleMust(t, err)
			if kind == ast.ASTPrDefAxiom {
				splits++
			}
			count, err := m.GetProofNumParents(p)
			ruleMust(t, err)
			for i := uint32(0); i < count; i++ {
				parent, err := m.GetProofParent(p, i)
				ruleMust(t, err)
				todo = append(todo, parent)
			}
		}
		if splits != expectedSplits {
			t.Fatalf("proof splits = %d, want %d from captured rewriter.flat", splits, expectedSplits)
		}
	}
	check(normalizer, 2)
	ruleMust(t, globals.SourceSetName("rewriter.flat", "true"))
	ruleMust(t, normalizer.Reset())
	check(normalizer, 2)
	newer, err := base.NewHornNormalizer(m, "updated-snapshot")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, newer.Close()) })
	check(newer, 1)
}
