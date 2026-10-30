package spacer

// Source: src/muz/spacer/spacer_context.cpp pred_transformer::add_cover
// uses expr_substitution and mk_default_expr_replacer, not var_subst.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerCoverUsesSourceExpressionSubstitution(t *testing.T) {
	for _, quantified := range []bool{false, true} {
		name := "column-variable"
		if quantified {
			name = "variable-under-quantifier"
		}
		t.Run(name, func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				depthMust(t, err)
				ref, err := m.NewRef(h)
				depthMust(t, err)
				pins = append(pins, ref)
				return h
			}
			boolean := own(m.MkBoolSort())
			decl := func(name string) ast.ASTHandle {
				t.Helper()
				s, err := m.Symbols.Intern(name)
				depthMust(t, err)
				return own(m.MkFuncDecl(s, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
			}
			predicate := decl("cover-substitution-predicate")
			propertyDecl := decl("cover-substitution-property")
			p, err := NewRulePredicateTransformer(c.owner, predicate, PropSolverConfig{})
			depthMust(t, err)
			defer func() { depthMust(t, p.Close()) }()
			truth := own(m.MkTrue())
			falsehood := own(m.MkFalse())
			depthMust(t, p.InitializeSystem(falsehood, truth))
			variable := own(m.MkVar(0, boolean))
			property := own(m.MkApp(propertyDecl, []ast.ASTHandle{variable}))
			if quantified {
				property = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: property}))
			}
			depthMust(t, p.AddCover(-1, property))
			signature, err := p.Signature()
			depthMust(t, err)
			current, err := c.owner.OldToNew(signature[0], 0)
			depthMust(t, err)
			constant := own(m.MkApp(current, nil))
			expected := own(m.MkApp(propertyDecl, []ast.ASTHandle{constant}))
			lemmas, err := p.frames.FrameLemmas(InfinityLevel)
			depthMust(t, err)
			if len(lemmas) != 1 || lemmas[0] != expected {
				t.Fatal("cover did not use source expression-identity replacement", lemmas, expected)
			}
			cover, err := p.CoverDelta(-1)
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&cover)) }()
			actual, err := m.RefGet(cover)
			depthMust(t, err)
			expected = own(m.MkApp(propertyDecl, []ast.ASTHandle{variable}))
			if actual != expected {
				t.Fatal("cover column reconstruction", actual, expected)
			}
			otherProperty := own(m.MkApp(decl("cover-second-property"), []ast.ASTHandle{variable}))
			depthMust(t, p.AddCover(-1, otherProperty))
			frame, err := p.Formulas(InfinityLevel, false)
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&frame)) }()
			frameBody, err := m.RefGet(frame)
			depthMust(t, err)
			args, err := m.GetNumArgs(frameBody)
			depthMust(t, err)
			if args != 2 {
				t.Fatal("expected two frame properties", args)
			}
			withAxioms, err := p.Formulas(0, true)
			depthMust(t, err)
			axiomBody, err := m.RefGet(withAxioms)
			depthMust(t, err)
			if !m.IsFalse(axiomBody) {
				t.Fatal("initial false axiom was not absorbed", axiomBody)
			}
			depthMust(t, m.DelRef(&withAxioms))
			depthMust(t, p.AddCover(-1, falsehood))
			blocked, err := p.CoverDelta(-1)
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&blocked)) }()
			blockedBody, err := m.RefGet(blocked)
			depthMust(t, err)
			if !m.IsFalse(blockedBody) {
				t.Fatal("cover conjunction did not absorb false", blockedBody)
			}
			invariant, err := p.Formulas(InfinityLevel, false)
			depthMust(t, err)
			invariantBody, err := m.RefGet(invariant)
			depthMust(t, err)
			if !m.IsFalse(invariantBody) {
				t.Fatal("frame conjunction did not absorb false", invariantBody)
			}
			depthMust(t, m.DelRef(&invariant))
			depthMust(t, p.Close())
			// A prior formula snapshot owns its conjunction and signature
			// constants independently of later queries or transformer teardown.
			args, err = m.GetNumArgs(frameBody)
			depthMust(t, err)
			if args != 2 {
				t.Fatal("frame snapshot changed after transformer close", args)
			}

		})
	}
}
