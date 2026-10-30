package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestRuleManagerAddsProofsForDisjunctiveHornSplit(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R")
	disjunction, err := f.manager.MkOr(f.app["Q"], f.app["R"])
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(disjunction, f.app["P"])
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "split-proof"); err != nil {
		t.Fatal(err)
	}
	if mode, e := f.manager.ProofMode(); e != nil || mode != ast.ASTProofDisabled {
		t.Fatal("proof mode was not restored", mode, e)
	}
	if count, e := set.Len(); e != nil || count != 2 {
		t.Fatal("normalized rule count", count, e)
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	for i := uint32(0); i < 2; i++ {
		rule, e := set.Rule(int(i))
		ruleMust(t, e)
		fact, e := rule.ToFormula()
		ruleMust(t, e)
		defer func() { ruleMust(t, f.manager.DelRef(&fact)) }()
		factHandle, e := f.manager.RefGet(fact)
		ruleMust(t, e)
		translated, found, e := rule.Proof()
		ruleMust(t, e)
		if !found {
			t.Fatal("normalized Horn rule lost its proof", i)
		}
		proved, e := f.manager.GetProofFact(translated)
		ruleMust(t, e)
		if proved != factHandle {
			t.Fatal("proof fact does not match normalized rule", i, proved, factHandle)
		}
		valid, side, e := checker.Check(translated)
		ruleMust(t, e)
		if !valid {
			t.Fatal("normalized Horn proof failed proof checking", i)
		}
		ruleMust(t, checker.ReleaseSideConditions(side))
		ruleMust(t, rule.Close())
	}
}

func TestRuleManagerRemovesLabelsAndPreservesHNFProofs(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R")
	disjunction, err := f.manager.MkOr(f.app["Q"], f.app["R"])
	ruleMust(t, err)
	body, err := f.manager.MkImplies(disjunction, f.app["P"])
	ruleMust(t, err)
	label, err := f.manager.Symbols.Intern("proof-label")
	ruleMust(t, err)
	formula, err := f.manager.MkLabel(true, []usymbol.Symbol{label}, body)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "labelled-split"); err != nil {
		t.Fatal(err)
	}
	if count, e := set.Len(); e != nil || count != 2 {
		t.Fatal("labelled split rule count", count, e)
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	for i := 0; i < 2; i++ {
		rule, e := set.Rule(i)
		ruleMust(t, e)
		translated, found, e := rule.Proof()
		ruleMust(t, e)
		if !found {
			t.Fatal("labelled Horn rule lost its proof", i)
		}
		valid, side, e := checker.Check(translated)
		ruleMust(t, e)
		if !valid {
			t.Fatal("labelled Horn proof failed proof checking", i)
		}
		ruleMust(t, checker.ReleaseSideConditions(side))
		ruleMust(t, rule.Close())
	}
}

func TestRuleManagerAddsProofForAlreadyHornFormula(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	formula, err := f.manager.MkImplies(f.app["Q"], f.app["P"])
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "already-horn-proof"); err != nil {
		t.Fatal(err)
	}
	if count, e := set.Len(); e != nil || count != 1 {
		t.Fatal("already-Horn rule count", count, e)
	}
	rule, err := set.Rule(0)
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	translated, found, err := rule.Proof()
	ruleMust(t, err)
	if !found {
		t.Fatal("already-Horn rule lost its proof")
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	valid, side, err := checker.Check(translated)
	ruleMust(t, err)
	if !valid {
		t.Fatal("already-Horn proof failed proof checking")
	}
	ruleMust(t, checker.ReleaseSideConditions(side))
}

func TestRuleManagerRejectsMismatchedHNFProofWithoutPublishing(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	formula, err := f.manager.MkImplies(f.app["Q"], f.app["P"])
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(f.app["Q"])
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "mismatched-proof"); err == nil {
		t.Fatal("mismatched proof fact was accepted")
	}
	if count, e := set.Len(); e != nil || count != 0 {
		t.Fatal("mismatched proof published rules", count, e)
	}
	if mode, e := f.manager.ProofMode(); e != nil || mode != ast.ASTProofDisabled {
		t.Fatal("mismatched proof changed proof mode", mode, e)
	}
}

func TestRuleManagerAddsProofsForHNFDefinitionIntroduction(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R", "S", "H")
	firstDisjunction, err := f.manager.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	secondDisjunction, err := f.manager.MkOr(f.app["R"], f.app["S"])
	ruleMust(t, err)
	body, err := f.manager.MkAnd(firstDisjunction, secondDisjunction)
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(body, f.app["H"])
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "definition-proof"); err != nil {
		t.Fatal(err)
	}
	if count, e := set.Len(); e != nil || count != 5 {
		t.Fatal("normalized HNF rule count", count, e)
	}
	if mode, e := f.manager.ProofMode(); e != nil || mode != ast.ASTProofDisabled {
		t.Fatal("proof mode was not restored", mode, e)
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	for i := 0; i < 5; i++ {
		rule, e := set.Rule(i)
		ruleMust(t, e)
		fact, e := rule.ToFormula()
		ruleMust(t, e)
		defer func() { ruleMust(t, f.manager.DelRef(&fact)) }()
		factHandle, e := f.manager.RefGet(fact)
		ruleMust(t, e)
		translated, found, e := rule.Proof()
		ruleMust(t, e)
		if !found {
			t.Fatal("HNF definition rule lost its proof", i)
		}
		proved, e := f.manager.GetProofFact(translated)
		ruleMust(t, e)
		if proved != factHandle {
			t.Fatal("HNF definition proof fact does not match rule", i, proved, factHandle)
		}
		valid, side, e := checker.Check(translated)
		ruleMust(t, e)
		// Source HNF constructs one congruence over the implication using
		// literal definitions inside its conjunction. The source checker only
		// accepts immediate argument rewrites, so it rejects the main clause.
		if valid != (i != 0) {
			t.Fatal("source HNF/checker congruence behavior differs", i, valid)
		}
		ruleMust(t, checker.ReleaseSideConditions(side))
		ruleMust(t, rule.Close())
	}
}

func TestRuleManagerAddsProofsForQuantifiedDisjunctiveHornSplit(t *testing.T) {
	f := newRuleSetFixture(t)
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	arithmetic, err := f.manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := f.manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	for _, name := range []string{"P", "Q", "R"} {
		symbol, e := f.manager.Symbols.Intern("quantified-split-" + name)
		ruleMust(t, e)
		f.decl[name], e = f.manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		ruleMust(t, f.rules.RegisterPredicate(f.decl[name]))
	}
	x, err := f.manager.MkVar(0, integer)
	ruleMust(t, err)
	applications := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Q", "R"} {
		applications[name], err = f.manager.MkApp(f.decl[name], []ast.ASTHandle{x})
		ruleMust(t, err)
	}
	disjunction, err := f.manager.MkOr(applications["Q"], applications["R"])
	ruleMust(t, err)
	body, err := f.manager.MkImplies(disjunction, applications["P"])
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("x")
	ruleMust(t, err)
	formula, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{name}, Body: body})
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "quantified-split-proof"); err != nil {
		t.Fatal(err)
	}
	if count, e := set.Len(); e != nil || count != 2 {
		t.Fatal("normalized quantified rule count", count, e)
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	for i := 0; i < 2; i++ {
		rule, e := set.Rule(i)
		ruleMust(t, e)
		translated, found, e := rule.Proof()
		ruleMust(t, e)
		if !found {
			t.Fatal("quantified normalized Horn rule lost its proof", i)
		}
		valid, side, e := checker.Check(translated)
		ruleMust(t, e)
		if !valid {
			t.Fatal("quantified normalized Horn proof failed proof checking", i)
		}
		ruleMust(t, checker.ReleaseSideConditions(side))
		ruleMust(t, rule.Close())
	}
}

func TestRuleManagerCarriesHNFProofsThroughSplitAndDefinitionWorklist(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R", "S", "H")
	m := f.manager
	inner, err := m.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	branch, err := m.MkAnd(inner, f.app["R"])
	ruleMust(t, err)
	outer, err := m.MkOr(branch, f.app["S"])
	ruleMust(t, err)
	formula, err := m.MkImplies(outer, f.app["H"])
	ruleMust(t, err)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	original, err := m.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, set.Close()) }()
	ruleMust(t, f.rules.AddFormulaWithProof(set, formula, original, "split-and-definition"))
	count, err := set.Len()
	ruleMust(t, err)
	if count != 4 {
		t.Fatal("expected two main clauses and two definitions", count)
	}
	checker, err := proofs.NewChecker(m)
	ruleMust(t, err)
	kinds := make(map[int32]bool)
	for i := 0; i < count; i++ {
		rule, err := set.Rule(i)
		ruleMust(t, err)
		p, found, err := rule.Proof()
		ruleMust(t, err)
		if !found {
			t.Fatal("worklist clause lost proof", i)
		}
		valid, side, err := checker.Check(p)
		ruleMust(t, err)
		// The second emitted main clause changes an inner conjunction;
		// preserve the C++ checker's rejection of its nested congruence.
		if valid != (i != 1) {
			t.Fatal("source worklist/checker congruence behavior differs", i, valid)
		}
		ruleMust(t, checker.ReleaseSideConditions(side))
		todo := []ast.ASTHandle{p}
		seen := make(map[ast.ASTHandle]bool)
		for len(todo) != 0 {
			node := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if seen[node] {
				continue
			}
			seen[node] = true
			kind, err := m.GetDeclKind(node)
			ruleMust(t, err)
			kinds[kind] = true
			parents, err := m.GetProofNumParents(node)
			ruleMust(t, err)
			for j := uint32(0); j < parents; j++ {
				parent, err := m.GetProofParent(node, j)
				ruleMust(t, err)
				todo = append(todo, parent)
			}
		}
		ruleMust(t, rule.Close())
	}
	for _, kind := range []int32{ast.ASTPrAndElim, ast.ASTPrDefIntro, ast.ASTPrApplyDef, ast.ASTPrMonotonicity} {
		if !kinds[kind] {
			t.Fatal("missing source proof stage", kind)
		}
	}
}

func TestHornProofNormalizerPreservesAlreadyHornBeforeCancellation(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	m := f.manager
	formula, err := m.MkImplies(f.app["Q"], f.app["P"])
	ruleMust(t, err)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	original, err := m.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	normalizer, err := base.NewHornNormalizer(m, "horn-fast-path")
	ruleMust(t, err)
	m.Limit.Cancel()
	defer m.Limit.ResetCancel()
	result, err := normalizer.NormalizeWithProof(formula, original)
	ruleMust(t, err)
	defer func() { ruleMust(t, result.Close()) }()
	if len(result.Formulas) != 1 || len(result.Proofs) != 1 {
		t.Fatal("already-Horn input was lost to the worklist checkpoint")
	}
	actual, err := m.RefGet(result.Formulas[0])
	ruleMust(t, err)
	proof, err := m.RefGet(result.Proofs[0])
	ruleMust(t, err)
	if actual != formula || proof != original {
		t.Fatal("already-Horn fast path changed formula or proof")
	}
}

func TestHornProofNormalizerCarriesQuantifiedBodyDefinition(t *testing.T) {
	for _, outer := range []bool{false, true} {
		name := "unprefixed"
		if outer {
			name = "universal-prefix"
		}
		t.Run(name, func(t *testing.T) {
			f := newRuleSetFixture(t, "H")
			m := f.manager
			boolean, err := m.MkBoolSort()
			ruleMust(t, err)
			variable, err := m.MkVar(0, boolean)
			ruleMust(t, err)
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
			atom := func(name string) ast.ASTHandle {
				s, err := m.Symbols.Intern(name)
				ruleMust(t, err)
				d := own(m.MkFuncDecl(s, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
				return own(m.MkApp(d, []ast.ASTHandle{variable}))
			}
			body := own(m.MkAnd(atom("proof-quant-P"), atom("proof-quant-Q")))
			quantified := own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body}))
			formula := own(m.MkImplies(quantified, f.app["H"]))
			if outer {
				formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(1)}, Body: formula}))
			}
			ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			original := own(m.MkProofAsserted(formula))
			ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
			normalizer, err := base.NewHornNormalizer(m, "proof-quant-body")
			ruleMust(t, err)
			result, err := normalizer.NormalizeWithProof(formula, original)
			ruleMust(t, err)
			defer func() { ruleMust(t, result.Close()) }()
			if len(result.Formulas) != 2 || len(result.Proofs) != 2 || len(result.FreshPredicates) != 1 {
				t.Fatal("quantified definition result sizes", len(result.Formulas), len(result.Proofs), len(result.FreshPredicates))
			}
			checker, err := proofs.NewChecker(m)
			ruleMust(t, err)
			kinds := make(map[int32]bool)
			for i, ref := range result.Proofs {
				p, err := m.RefGet(ref)
				ruleMust(t, err)
				fact, err := m.GetProofFact(p)
				ruleMust(t, err)
				expected, err := m.RefGet(result.Formulas[i])
				ruleMust(t, err)
				if fact != expected {
					t.Fatal("quantified proof fact does not match output", i)
				}
				valid, side, err := checker.Check(p)
				ruleMust(t, err)
				// Source NNFNeg proves (~old_quantifier) ~ new_quantifier;
				// the enclosing congruence expects old_quantifier ~ new_quantifier.
				// Preserve the source checker's rejection of this main proof.
				if valid != (i != 0) {
					t.Fatal("source quantified-body/checker behavior differs", i, valid)
				}
				ruleMust(t, checker.ReleaseSideConditions(side))
				todo := []ast.ASTHandle{p}
				for len(todo) != 0 {
					node := todo[len(todo)-1]
					todo = todo[:len(todo)-1]
					kind, err := m.GetDeclKind(node)
					ruleMust(t, err)
					kinds[kind] = true
					if kind == ast.ASTPrNNFNeg {
						fact, err := m.GetProofFact(node)
						ruleMust(t, err)
						left, err := m.GetArg(fact, 0)
						ruleMust(t, err)
						negative, originalQuantifier, err := m.IsNot(left)
						ruleMust(t, err)
						if !negative || originalQuantifier != quantified {
							t.Fatal("source NNFNeg fact shape", left)
						}
					}
					count, err := m.GetProofNumParents(node)
					ruleMust(t, err)
					for j := uint32(0); j < count; j++ {
						parent, err := m.GetProofParent(node, j)
						ruleMust(t, err)
						todo = append(todo, parent)
					}
				}
			}
			if !kinds[ast.ASTPrNNFNeg] || !kinds[ast.ASTPrApplyDef] || !kinds[ast.ASTPrDefIntro] {
				t.Fatal("missing quantified definition proof stages", kinds)
			}
			if outer && !kinds[ast.ASTPrQuantIntro] {
				t.Fatal("missing quantified congruence lifting")
			}
			mode, err := m.ProofMode()
			ruleMust(t, err)
			if mode != ast.ASTProofDisabled {
				t.Fatal("proof mode was not restored")
			}
		})
	}
}

func TestHornProofNormalizerMemoizesDefinitionProofAcrossOccurrences(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "H")
	m := f.manager
	disjunction, err := m.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	body, err := m.MkAnd(disjunction, disjunction)
	ruleMust(t, err)
	formula, err := m.MkImplies(body, f.app["H"])
	ruleMust(t, err)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	original, err := m.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	normalizer, err := base.NewHornNormalizer(m, "proof-memo")
	ruleMust(t, err)
	result, err := normalizer.NormalizeWithProof(formula, original)
	ruleMust(t, err)
	defer func() { ruleMust(t, result.Close()) }()
	if len(result.FreshPredicates) != 1 || len(result.Formulas) != 3 {
		t.Fatal("repeated disjunction introduced duplicate definitions", len(result.FreshPredicates), len(result.Formulas))
	}
	main, err := m.RefGet(result.Proofs[0])
	ruleMust(t, err)
	todo := []ast.ASTHandle{main}
	found := false
	for len(todo) != 0 {
		node := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		kind, err := m.GetDeclKind(node)
		ruleMust(t, err)
		count, err := m.GetProofNumParents(node)
		ruleMust(t, err)
		if kind == ast.ASTPrMonotonicity {
			if count != 2 {
				t.Fatal("congruence did not preserve both occurrences", count)
			}
			first, err := m.GetProofParent(node, 0)
			ruleMust(t, err)
			second, err := m.GetProofParent(node, 1)
			ruleMust(t, err)
			if first != second {
				t.Fatal("definition proof was rebuilt instead of memoized")
			}
			kind, err := m.GetDeclKind(first)
			ruleMust(t, err)
			if kind != ast.ASTPrApplyDef {
				t.Fatal("memoized proof is not source apply-def", kind)
			}
			found = true
		}
		for i := uint32(0); i < count; i++ {
			p, err := m.GetProofParent(node, i)
			ruleMust(t, err)
			todo = append(todo, p)
		}
	}
	if !found {
		t.Fatal("missing congruence for repeated definitions")
	}
}

func TestHornNormalizerDefinitionNegationMatchesSource(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		for _, implication := range []bool{false, true} {
			name := "negated-conjunction"
			if implication {
				name = "implication-with-negated-premise"
			}
			if withProof {
				name += "/proof"
			}
			t.Run(name, func(t *testing.T) {
				f := newRuleSetFixture(t, "P", "Q", "R", "H")
				m := f.manager
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
				var expression ast.ASTHandle
				if implication {
					expression = own(m.MkImplies(own(m.MkNot(f.app["P"])), f.app["Q"]))
				} else {
					expression = own(m.MkNot(own(m.MkAnd(f.app["P"], f.app["Q"]))))
				}
				formula := own(m.MkImplies(own(m.MkAnd(expression, f.app["R"])), f.app["H"]))
				normalizer, err := base.NewHornNormalizer(m, "proof-negation")
				ruleMust(t, err)
				var result *base.HornNormalization
				if withProof {
					ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					p := own(m.MkProofAsserted(formula))
					ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
					result, err = normalizer.NormalizeWithProof(formula, p)
				} else {
					result, err = normalizer.Normalize(formula)
				}
				ruleMust(t, err)
				defer func() { ruleMust(t, result.Close()) }()
				if len(result.FreshPredicates) != 1 || len(result.Formulas) != 3 {
					t.Fatal("definition sizes", len(result.FreshPredicates), len(result.Formulas))
				}
				expected := []ast.ASTHandle{f.app["P"], f.app["Q"]}
				if !implication {
					expected[0] = own(m.MkNot(expected[0]))
					expected[1] = own(m.MkNot(expected[1]))
				}
				seen := make(map[ast.ASTHandle]bool)
				for _, ref := range result.Formulas[1:] {
					formula, err := m.RefGet(ref)
					ruleMust(t, err)
					body, err := m.GetArg(formula, 0)
					ruleMust(t, err)
					seen[body] = true
				}
				if !seen[expected[0]] || !seen[expected[1]] {
					t.Fatal("definition negation differs from source", seen, expected)
				}
			})
		}
	}
}
