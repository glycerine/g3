package transforms_test

// Source: src/muz/transforms/dl_mk_coalesce.cpp shared-body disjunction.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestRuleCoalesceMergesSharedBodyConstraints(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	one, err := f.manager.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	coneMust(t, err)
	two, err := f.manager.ArithmeticNumeralRat(big.NewRat(2, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("coalesce-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("P")
	q := declare("Q")
	variable, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	pVariable, err := f.manager.MkApp(p, []ast.ASTHandle{variable})
	coneMust(t, err)
	qVariable, err := f.manager.MkApp(q, []ast.ASTHandle{variable})
	coneMust(t, err)
	for _, value := range []ast.ASTHandle{zero, one, two} {
		equality, makeErr := f.manager.MkEq(variable, value)
		coneMust(t, makeErr)
		rule, makeErr := f.rules.NewRule(pVariable, []ast.ASTHandle{qVariable, equality}, nil, "branch")
		coneMust(t, makeErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	coneMust(t, f.set.SetOutputPredicate(p))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("coalesce fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleCoalescePlugin(f.manager, f.rules, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("coalesce plugin returned no replacement")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 1 {
		t.Fatalf("coalesced rule count %d, want 1", count)
	}
	rule, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	uninterpreted, err := rule.UninterpretedTailSize()
	coneMust(t, err)
	total, err := rule.TailSize()
	coneMust(t, err)
	formula, _, err := rule.Tail(1)
	coneMust(t, err)
	operation, err := f.manager.GetDeclKind(formula)
	coneMust(t, err)
	if uninterpreted != 1 || total != 2 || operation != ast.ASTOpOr {
		t.Fatal("coalesced body shape", uninterpreted, total, operation)
	}
}

func TestRuleCoalesceDerivesMergedProofFromBothClauses(t *testing.T) {
	f := newConeFixture(t)
	if err := f.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
		t.Fatal(err)
	}
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	one, err := f.manager.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("coalesce-proof-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p, q := declare("P"), declare("Q")
	variable, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	pVariable, err := f.manager.MkApp(p, []ast.ASTHandle{variable})
	coneMust(t, err)
	qVariable, err := f.manager.MkApp(q, []ast.ASTHandle{variable})
	coneMust(t, err)
	var sourceProof, targetProof ast.ASTHandle
	for index, value := range []ast.ASTHandle{zero, one} {
		condition, makeErr := f.manager.MkEq(variable, value)
		coneMust(t, makeErr)
		rule, makeErr := f.rules.NewRule(pVariable, []ast.ASTHandle{qVariable, condition}, nil, "coalesce-proof-branch")
		coneMust(t, makeErr)
		formula, makeErr := rule.ToFormula()
		coneMust(t, makeErr)
		fact, makeErr := f.manager.RefGet(formula)
		coneMust(t, makeErr)
		proof, makeErr := f.manager.MkProofAsserted(fact)
		coneMust(t, makeErr)
		coneMust(t, rule.SetProof(proof))
		if index == 1 {
			sourceProof = proof
		} else {
			targetProof = proof
		}
		coneMust(t, f.manager.DelRef(&formula))
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	coneMust(t, f.set.SetOutputPredicate(p))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("proof-carrying coalesce fixture rejected")
	}
	if err = f.manager.ToggleProofMode(ast.ASTProofDisabled); err != nil {
		t.Fatal(err)
	}
	plugin := transforms.NewRuleCoalescePluginWithProofTrace(f.manager, f.rules, 0, true)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	if count, countErr := result.Len(); countErr != nil || count != 1 {
		t.Fatalf("coalesced rule count %d, %v", count, countErr)
	}
	merged, err := result.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, merged.Close()) }()
	proof, found, err := merged.Proof()
	coneMust(t, err)
	if !found {
		t.Fatal("coalesced rule lost its proof")
	}
	isProof, err := f.manager.IsProof(proof)
	coneMust(t, err)
	if !isProof {
		t.Fatal("coalesced proof is not a proof AST")
	}
	kind, err := f.manager.GetDeclKind(proof)
	coneMust(t, err)
	if kind != ast.ASTPrModusPonens {
		t.Fatalf("coalesced proof kind %d, want modus ponens", kind)
	}
	if count, e := f.manager.GetProofNumParents(proof); e != nil || count != 2 {
		t.Fatal("merge proof parent count", count, e)
	}
	target, e := f.manager.GetProofParent(proof, 0)
	coneMust(t, e)
	implication, e := f.manager.GetProofParent(proof, 1)
	coneMust(t, e)
	source, e := f.manager.GetProofParent(implication, 0)
	coneMust(t, e)
	if source != sourceProof || target != targetProof {
		t.Fatal("merge proof must use both original clauses")
	}
	formula, err := merged.ToFormula()
	coneMust(t, err)
	formulaHandle, err := f.manager.RefGet(formula)
	coneMust(t, err)
	proofFact, err := f.manager.GetProofFact(proof)
	coneMust(t, err)
	equal, err := f.manager.AreEqual(proofFact, formulaHandle)
	coneMust(t, err)
	if !equal {
		t.Fatal("coalesced proof proves the wrong rule formula")
	}
	coneMust(t, f.manager.DelRef(&formula))
}

// AUDIT.md muz_transforms_01-F4: mk_coalesce walks begin_grouped_rules(),
// the m_head2rules obj_map table order, not first-appearance order.
func TestRuleCoalesceEmitsHeadsInGroupedOrder(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("coalesce-order-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	q := declare("Q")
	variable, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	qVariable, err := f.manager.MkApp(q, []ast.ASTHandle{variable})
	coneMust(t, err)
	var inserted []ast.ASTHandle
	for _, name := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		head := declare(name)
		inserted = append(inserted, head)
		application, makeErr := f.manager.MkApp(head, []ast.ASTHandle{variable})
		coneMust(t, makeErr)
		rule, makeErr := f.rules.NewRule(application, []ast.ASTHandle{qVariable}, nil, "r")
		coneMust(t, makeErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
		coneMust(t, f.set.SetOutputPredicate(head))
	}
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("coalesce fixture rejected")
	}
	grouped, err := f.set.GroupedHeads()
	coneMust(t, err)
	same := len(grouped) == len(inserted)
	for i := 0; same && i < len(grouped); i++ {
		same = grouped[i] == inserted[i]
	}
	if same {
		t.Fatal("fixture grouped order equals insertion order; it cannot detect the divergence")
	}
	plugin := transforms.NewRuleCoalescePlugin(f.manager, f.rules, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	count, err := result.Len()
	coneMust(t, err)
	if count != len(grouped) {
		t.Fatal("coalesced rule count", count, len(grouped))
	}
	for i := 0; i < count; i++ {
		rule, err := result.Rule(i)
		coneMust(t, err)
		head, err := rule.HeadDeclaration()
		coneMust(t, errors.Join(err, rule.Close()))
		if head != grouped[i] {
			t.Fatal("rule", i, "head is not in begin_grouped_rules order")
		}
	}
}

// AUDIT.md muz_transforms_01-F5: extract_conjs and merge_rules combine the
// guards with bool_rewriter::mk_and / mk_or, not raw applications.
func TestRuleCoalesceSimplifiesGuardWithBoolRewriter(t *testing.T) {
	for _, test := range []struct {
		name  string
		guard func(f *coneFixture, v ast.ASTHandle) [2][]ast.ASTHandle
		check func(t *testing.T, f *coneFixture, guard ast.ASTHandle)
	}{
		{name: "empty-conjunction-absorbs", guard: func(f *coneFixture, v ast.ASTHandle) [2][]ast.ASTHandle {
			one, err := f.manager.ArithmeticNumeralRat(big.NewRat(1, 1), true)
			coneMust(t, err)
			bound, err := f.manager.MkArithmeticGT(v, one)
			coneMust(t, err)
			return [2][]ast.ASTHandle{nil, {bound}}
		}, check: func(t *testing.T, f *coneFixture, guard ast.ASTHandle) {
			if !f.manager.IsTrue(guard) {
				t.Fatal("bool_rewriter mk_or(true, c) must give true")
			}
		}},
		{name: "single-conjuncts", guard: func(f *coneFixture, v ast.ASTHandle) [2][]ast.ASTHandle {
			var eqs [2][]ast.ASTHandle
			for i := range eqs {
				value, err := f.manager.ArithmeticNumeralRat(big.NewRat(int64(i+1), 1), true)
				coneMust(t, err)
				eq, err := f.manager.MkEq(v, value)
				coneMust(t, err)
				eqs[i] = []ast.ASTHandle{eq}
			}
			return eqs
		}, check: func(t *testing.T, f *coneFixture, guard ast.ASTHandle) {
			operation, err := f.manager.GetDeclKind(guard)
			coneMust(t, err)
			if operation != ast.ASTOpOr {
				t.Fatal("merged guard is not a disjunction", operation)
			}
			n, err := f.manager.GetNumArgs(guard)
			coneMust(t, err)
			for i := uint32(0); i < n; i++ {
				arg, err := f.manager.GetArg(guard, i)
				coneMust(t, err)
				operation, err := f.manager.GetDeclKind(arg)
				coneMust(t, err)
				if operation != ast.ASTOpEq {
					t.Fatal("disjunct", i, "is not the single equality", operation)
				}
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newConeFixture(t)
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
			coneMust(t, err)
			integer, err := f.manager.GetSort(zero)
			coneMust(t, err)
			declare := func(name string) ast.ASTHandle {
				symbol, makeErr := f.manager.Symbols.Intern("coalesce-bwr-" + name)
				coneMust(t, makeErr)
				declaration, makeErr := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
				coneMust(t, makeErr)
				coneMust(t, f.rules.RegisterPredicate(declaration))
				return declaration
			}
			p := declare("P")
			q := declare("Q")
			// P(#0) :- Q(#1), guard(#1): every head/tail argument is a
			// distinct variable, so extract_conjs adds no equalities.
			headVariable, err := f.manager.MkVar(0, integer)
			coneMust(t, err)
			variable, err := f.manager.MkVar(1, integer)
			coneMust(t, err)
			pVariable, err := f.manager.MkApp(p, []ast.ASTHandle{headVariable})
			coneMust(t, err)
			qVariable, err := f.manager.MkApp(q, []ast.ASTHandle{variable})
			coneMust(t, err)
			for _, guard := range test.guard(f, variable) {
				rule, makeErr := f.rules.NewRule(pVariable, append([]ast.ASTHandle{qVariable}, guard...), nil, "branch")
				coneMust(t, makeErr)
				coneMust(t, f.set.Add(rule))
				coneMust(t, rule.Close())
			}
			coneMust(t, f.set.SetOutputPredicate(p))
			valid, err := f.set.Finalize()
			coneMust(t, err)
			if !valid {
				t.Fatal("coalesce fixture rejected")
			}
			plugin := transforms.NewRuleCoalescePlugin(f.manager, f.rules, 0)
			plugin.SetParamServices(f.globals.Context, f.globals)
			defer func() { coneMust(t, plugin.Close()) }()
			result, err := plugin.Transform(f.set)
			coneMust(t, err)
			defer func() { coneMust(t, result.Close()) }()
			count, err := result.Len()
			coneMust(t, err)
			if count != 1 {
				t.Fatal("coalesced rule count", count)
			}
			rule, err := result.Rule(0)
			coneMust(t, err)
			defer func() { coneMust(t, rule.Close()) }()
			total, err := rule.TailSize()
			coneMust(t, err)
			guard, _, err := rule.Tail(total - 1)
			coneMust(t, err)
			test.check(t, f, guard)
		})
	}
}
