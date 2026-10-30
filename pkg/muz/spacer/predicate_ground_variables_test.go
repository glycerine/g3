package spacer

// Source: src/muz/spacer/spacer_context.cpp ground_free_vars uses
// expr_free_vars over quantifier bodies, excluding trigger annotations.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerRuleGroundingUsesBodyFreeVariables(t *testing.T) {
	for _, tc := range []struct {
		name                string
		noPattern, bodyFree bool
	}{
		{"pattern-only-free-variable", false, false},
		{"no-pattern-only-free-variable", true, false},
		{"body-and-pattern-free-variable", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			bound := own(m.MkVar(0, boolean))
			free := own(m.MkVar(1, boolean))
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				t.Helper()
				nameSymbol, err := m.Symbols.Intern(name)
				depthMust(t, err)
				return own(m.MkFuncDecl(nameSymbol, domain, boolean, ast.ASTFuncDeclInfo{}))
			}
			property := decl("grounding-body", []ast.ASTHandle{boolean})
			annotationDecl := decl("grounding-trigger", []ast.ASTHandle{boolean, boolean})
			body := own(m.MkApp(property, []ast.ASTHandle{bound}))
			if tc.bodyFree {
				body = own(m.MkAnd(body, own(m.MkApp(property, []ast.ASTHandle{free}))))
			}
			annotation := own(m.MkApp(annotationDecl, []ast.ASTHandle{bound, free}))
			spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}
			if tc.noPattern {
				spec.NoPatterns = []ast.ASTHandle{annotation}
			} else {
				spec.Patterns = []ast.ASTHandle{own(m.MkPattern([]ast.ASTHandle{annotation}))}
			}
			quantified := own(m.MkQuantifier(spec))
			head, err := f.rule.Head()
			depthMust(t, err)
			rule, err := f.manager.NewRule(head, []ast.ASTHandle{own(m.MkNot(quantified))}, nil, "grounding-quantified-initial")
			depthMust(t, err)
			defer func() { depthMust(t, rule.Close()) }()
			set, err := base.NewRuleSet(f.manager)
			depthMust(t, err)
			defer func() { depthMust(t, set.Close()) }()
			depthMust(t, set.Add(rule))
			depthMust(t, c.UpdateRules(set))
			auxiliaries, err := c.transformers[f.query].RuleAuxiliaries(rule)
			depthMust(t, err)
			expected := 0
			if tc.bodyFree {
				expected = 1
			}
			if len(auxiliaries) != expected {
				t.Fatalf("auxiliary count = %d, want %d from source body-only scan", len(auxiliaries), expected)
			}
		})
	}
}
