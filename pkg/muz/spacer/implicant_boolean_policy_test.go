package spacer

// Source: src/muz/spacer/spacer_util.cpp:809-868, implicant_picker::process_app.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerImplicantBooleanPoliciesAndSemanticImplication(t *testing.T) {
	cases := []struct {
		name, form string
		values     [3]bool
		want       []string
	}{
		{"false-and-first", "not-and", [3]bool{false, false, true}, []string{"!p"}},
		{"false-and-second", "not-and", [3]bool{true, false, false}, []string{"!q"}},
		{"true-or-first", "or", [3]bool{true, true, true}, []string{"p"}},
		{"true-or-second", "or", [3]bool{false, true, true}, []string{"q"}},
		{"false-or-all", "eq-or", [3]bool{false, false, false}, []string{"!r", "!q", "!p"}},
		{"true-and-all", "eq-and", [3]bool{true, true, true}, []string{"r", "q", "p"}},
		{"false-compound-equality", "not-eq-and", [3]bool{true, true, false}, []string{"!r", "q", "p"}},
		{"identical-equality", "eq-self", [3]bool{true, false, false}, nil},
		{"distinct-Boolean-values", "distinct-values", [3]bool{true, false, false}, nil},
		{"top-and-flatten", "and", [3]bool{true, true, true}, []string{"r", "p", "q"}},
		{"false-implication-all", "eq-implies", [3]bool{true, false, false}, []string{"!r", "!q", "p"}},
		{"true-implication-consequent", "implies", [3]bool{false, true, true}, []string{"q"}},
		{"true-implication-antecedent", "implies", [3]bool{false, false, true}, []string{"!p"}},
		{"constant-equality", "eq", [3]bool{true, true, false}, []string{"eq"}},
		{"constant-disequality", "not-eq", [3]bool{true, false, false}, []string{"not-eq"}},
		{"true-xor", "xor", [3]bool{true, false, false}, []string{"!q", "p"}},
		{"false-xor", "not-xor", [3]bool{true, true, false}, []string{"q", "p"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, globals := newSpacerTestServices(t)
			config := smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals}
			owner, err := NewManager(m, nil, 1, config)
			depthMust(t, err)
			defer func() { depthMust(t, owner.Close()) }()
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				depthMust(t, err)
				ref, err := m.NewRef(h)
				depthMust(t, err)
				pins = append(pins, ref)
				return h
			}
			boolean := own(m.MkBoolSort())
			p := own(m.MkFreshConst("policy-p", boolean))
			q := own(m.MkFreshConst("policy-q", boolean))
			r := own(m.MkFreshConst("policy-r", boolean))
			truth, falsity := own(m.MkTrue()), own(m.MkFalse())
			and := own(m.MkAnd(p, q))
			or := own(m.MkOr(p, q))
			implies := own(m.MkImplies(p, q))
			xor := own(m.MkXor(p, q))
			eq := own(m.MkEq(p, q))
			and3 := own(m.MkAndArray([]ast.ASTHandle{p, q, r}))
			forms := map[string]ast.ASTHandle{
				"p": p, "q": q, "r": r, "!p": own(m.MkNot(p)), "!q": own(m.MkNot(q)), "!r": own(m.MkNot(r)),
				"and": and3, "not-and": own(m.MkNot(and3)), "or": own(m.MkOrArgs([]ast.ASTHandle{p, q, r})),
				"eq-or": own(m.MkEq(or, r)), "eq-and": own(m.MkEq(and, r)), "eq-implies": own(m.MkEq(implies, r)),
				"implies": implies, "eq": eq, "not-eq": own(m.MkNot(eq)), "xor": xor, "not-xor": own(m.MkNot(xor)),
			}
			forms["not-eq-and"] = own(m.MkNot(forms["eq-and"]))
			forms["eq-self"] = own(m.MkEq(p, p))
			valueEquality := own(m.MkEq(truth, falsity))
			forms["distinct-values"] = own(m.MkNot(valueEquality))
			md, err := model.NewModel(m)
			depthMust(t, err)
			defer func() { depthMust(t, md.Close()) }()
			for i, variable := range []ast.ASTHandle{p, q, r} {
				value := falsity
				if tc.values[i] {
					value = truth
				}
				decl, err := m.GetDecl(variable)
				depthMust(t, err)
				depthMust(t, md.Core().RegisterConstant(decl, value))
			}
			before := m.Stats().LiveReferences
			refs, err := computeImplicantLiterals(m, md.Core(), owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{forms[tc.form]})
			depthMust(t, err)
			if len(refs) != len(tc.want) {
				depthMust(t, releaseNormalizeRefs(m, refs))
				t.Fatal("source literal count", len(refs), len(tc.want))
			}
			literals := make([]ast.ASTHandle, len(refs))
			for i, ref := range refs {
				literals[i], err = m.RefGet(ref)
				depthMust(t, err)
				if literals[i] != forms[tc.want[i]] {
					depthMust(t, releaseNormalizeRefs(m, refs))
					t.Fatal("source literal order", i, tc.want[i])
				}
			}
			depthMust(t, releaseNormalizeRefs(m, refs))
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("temporary literal references", got, before)
			}
			// Discharge cube => formula independently of the supplied model.
			backend, err := smtcore.NewSMTSolver(m, nil, "QF_UF", config)
			depthMust(t, err)
			defer func() { depthMust(t, backend.Close()) }()
			cube := own(m.MkAndArray(literals))
			negated := own(m.MkNot(forms[tc.form]))
			depthMust(t, backend.Assert(cube))
			depthMust(t, backend.Push())
			depthMust(t, backend.Assert(negated))
			status, err := backend.CheckSat(nil)
			depthMust(t, err)
			if status != lbool.False {
				t.Fatal("selected cube does not imply formula", status)
			}
			depthMust(t, backend.Pop(1))
			for i, name := range []string{"p", "q", "r"} {
				if !tc.values[i] {
					name = "!" + name
				}
				depthMust(t, backend.Assert(forms[name]))
			}
			status, err = backend.CheckSat(nil)
			depthMust(t, err)
			if status != lbool.True {
				t.Fatal("cube is incompatible with supplied model", status)
			}
		})
	}
}
