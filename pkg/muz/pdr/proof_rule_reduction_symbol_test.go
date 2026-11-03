package pdr

// Source: model_search::get_proof_trace in src/muz/pdr/pdr_context.cpp uses
// rule_unifier::apply, whose combined label is tgt.name().str()+";"+
// src.name().str() in src/muz/transforms/dl_mk_rule_inliner.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRProofRuleReductionCombinesSourceSymbolText(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	rules, err := base.NewRuleManager(m)
	closureMust(t, err)
	var roots []ast.ASTRef
	defer func() {
		closureMust(t, rules.Close())
		for i := len(roots) - 1; i >= 0; i-- {
			closureMust(t, m.DelRef(&roots[i]))
		}
		closureMust(t, transformer.Close())
		closureMust(t, owner.Close())
		closeBase()
	}()
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	var applications [3]ast.ASTHandle
	for i, spelling := range []string{"proof-symbol-P", "proof-symbol-Q", "proof-symbol-R"} {
		name, err := m.Symbols.Intern(spelling)
		closureMust(t, err)
		declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		closureMust(t, err)
		closureMust(t, rules.RegisterPredicate(declaration))
		applications[i], err = m.MkApp(declaration, nil)
		closureMust(t, err)
		root, err := m.NewRef(applications[i])
		closureMust(t, err)
		roots = append(roots, root)
	}
	empty, err := m.Symbols.Intern("")
	closureMust(t, err)
	spelling, err := m.Symbols.Intern("k!7")
	closureMust(t, err)
	cases := []struct {
		kind, text string
		name       symbol.Symbol
	}{
		{kind: "null", text: "<null>"},
		{kind: "empty", name: empty},
		{kind: "numerical", text: "k!7", name: symbol.Numerical(7)},
		{kind: "spelling", text: "k!7", name: spelling},
	}
	context := &Context{manager: m, owner: owner}
	for _, left := range cases {
		for _, right := range cases {
			t.Run(left.kind+"/"+right.kind, func(t *testing.T) {
				// Resolve Q <- P, R with P <- R: the pivot disappears,
				// duplicate R is merged, and the surviving target tail maps to 0.
				target, err := rules.NewRuleWithSymbol(applications[1], []ast.ASTHandle{applications[0], applications[2]}, nil, left.name)
				closureMust(t, err)
				defer func() { closureMust(t, target.Close()) }()
				source, err := rules.NewRuleWithSymbol(applications[0], []ast.ASTHandle{applications[2]}, nil, right.name)
				closureMust(t, err)
				defer func() { closureMust(t, source.Close()) }()
				var tailMap []int
				resolved, targetTerms, sourceTerms, temporary, err := context.resolveProofRules(rules, target, source, 0, &tailMap)
				defer func() {
					if resolved != nil {
						closureMust(t, resolved.Close())
					}
					for i := len(temporary) - 1; i >= 0; i-- {
						closureMust(t, m.DelRef(&temporary[i]))
					}
				}()
				closureMust(t, err)
				wantText := left.text + ";" + right.text
				wantName, err := m.Symbols.Intern(wantText)
				closureMust(t, err)
				gotName, err := resolved.NameSymbol()
				closureMust(t, err)
				gotText, err := resolved.NameText()
				closureMust(t, err)
				if gotName != wantName || gotName.IsNumerical() || gotName == (symbol.Symbol{}) || gotText != wantText {
					t.Fatalf("resolved label = %v / %q, want textual symbol %v / %q", gotName, gotText, wantName, wantText)
				}
				head, err := resolved.Head()
				closureMust(t, err)
				count, err := resolved.TailSize()
				closureMust(t, err)
				tail, negative, err := resolved.Tail(0)
				closureMust(t, err)
				if head != applications[1] || count != 1 || tail != applications[2] || negative {
					t.Fatal("proof resolution did not remove the pivot and duplicate tail")
				}
				if len(targetTerms) != 0 || len(sourceTerms) != 0 || len(tailMap) != 2 || tailMap[0] != -1 || tailMap[1] != 0 {
					t.Fatal("proof resolution changed nullary substitutions or surviving-tail mapping", targetTerms, sourceTerms, tailMap)
				}
				for i, input := range []*base.Rule{target, source} {
					got, err := input.NameSymbol()
					closureMust(t, err)
					want := []symbol.Symbol{left.name, right.name}[i]
					if got != want {
						t.Fatal("proof resolution changed an input label identity", got, want)
					}
				}
			})
		}
	}
}
