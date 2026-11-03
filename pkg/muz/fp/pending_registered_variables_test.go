package fp

// Source: dl_rule.cpp rule_manager::mk_rule binds registered constants before
// normalization; dl_context.cpp invokes it again when pending rules advance.

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPendingNormalizationBindsRegisteredVariables(t *testing.T) {
	for _, open := range []bool{false, true} {
		for _, displayFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("open=%v/display=%v", open, displayFirst), func(t *testing.T) {
				m, globals := newPDRLifecycleServices(t)
				e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
				fragmentMust(t, err)
				var roots []ast.ASTRef
				t.Cleanup(func() {
					fragmentMust(t, e.Close())
					fragmentMust(t, closeASTRefs(m, roots))
				})
				fragmentMust(t, e.SelectEngine("datalog"))
				own := func(h ast.ASTHandle, cause error) ast.ASTHandle {
					t.Helper()
					fragmentMust(t, cause)
					ref, err := m.NewRef(h)
					fragmentMust(t, err)
					roots = append(roots, ref)
					return h
				}
				boolean := own(m.MkBoolSort())
				xName, err := m.Symbols.Intern("pending-registered-x")
				fragmentMust(t, err)
				x := own(m.MkConst(xName, boolean))
				fragmentMust(t, e.RegisterVariable(x))
				pName, err := m.Symbols.Intern("pending-registered-P")
				fragmentMust(t, err)
				p := own(m.MkFuncDecl(pName, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(p))
				first := x
				if open {
					// Source expr_abstract leaves existing de Bruijn indices
					// unchanged. Reserve index zero for the one constant that
					// Bind will abstract, keeping this original variable free.
					first = own(m.MkVar(1, boolean))
				}
				original := own(m.MkApp(p, []ast.ASTHandle{first, x}))
				free, err := m.HasFreeVars(original)
				fragmentMust(t, err)
				if free != open {
					t.Fatal("registered-variable fixture has the wrong raw free-variable phase", free, open)
				}
				const name, bound = "registered-pending", uint32(19)
				fragmentMust(t, e.AddRuleWithBound(original, name, bound))
				checkArchive := func(wantPending bool) {
					t.Helper()
					if displayFirst && open {
						if len(e.rawRules) != 0 {
							t.Fatal("open display promotion did not remove its original raw entry")
						}
						return
					}
					if len(e.rawRules) != 1 {
						t.Fatal("normalization changed the raw source archive length", len(e.rawRules))
					}
					raw := e.rawRules[0]
					h, err := m.RefGet(raw.formula)
					fragmentMust(t, err)
					if h != original || raw.name != name || raw.bound != bound || raw.pending != wantPending {
						t.Fatal("normalization changed original raw formula/name/bound or phase", h, raw.name, raw.bound, raw.pending)
					}
				}
				checkHead := func() {
					t.Helper()
					count, err := e.set.Len()
					fragmentMust(t, err)
					if count != 1 {
						t.Fatal("registered-variable normalization lost or duplicated its rule", count)
					}
					rule, err := e.set.Rule(0)
					fragmentMust(t, err)
					defer func() { fragmentMust(t, rule.Close()) }()
					head, err := rule.Head()
					fragmentMust(t, err)
					indices := [2]uint32{}
					for i := range indices {
						argument, err := m.GetArg(head, uint32(i))
						fragmentMust(t, err)
						kind, err := m.GetKind(argument)
						fragmentMust(t, err)
						if kind != ast.ASTVar {
							t.Fatal("pending normalization left a registered constant in the rule head", i, kind)
						}
						indices[i], err = m.GetVarIndex(argument)
						fragmentMust(t, err)
					}
					if open && (indices[0] == indices[1] || indices[0] > 1 || indices[1] > 1) || !open && indices != [2]uint32{} {
						t.Fatal("binding captured the existing free variable or split repeated registered variables", open, indices)
					}
				}
				if displayFirst {
					var output bytes.Buffer
					fragmentMust(t, e.WriteContextSMT2(&output, nil, nil))
					forms, err := e.RuleFormulas()
					fragmentMust(t, err)
					if len(forms) != 1 {
						t.Fatal("display changed the source formula count", len(forms))
					}
					formula, err := m.RefGet(forms[0])
					fragmentMust(t, err)
					if open == (formula == original) {
						t.Fatal("display promoted a closed input or failed to promote an open input", open)
					}
					fragmentMust(t, closeASTRefs(m, forms))
					checkArchive(true)
					if open {
						checkHead()
					}
				}
				status, err := e.QueryPredicate(p)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("query flush did not produce the universally bound fact", status)
				}
				checkHead()
				checkArchive(false)
				// A second query must consume the already bound clauses and keep
				// the same original archive rather than bind them twice.
				status, err = e.QueryPredicate(p)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("repeated query changed registered-variable semantics", status)
				}
				checkHead()
				checkArchive(false)
			})
		}
	}
}
