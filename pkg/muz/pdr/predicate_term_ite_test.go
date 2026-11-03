package pdr

// Source: pred_transformer::init_rule in src/muz/pdr/pdr_context.cpp:609-617.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRTermITEHoistingPreservesVocabularyAndSemantics(t *testing.T) {
	for _, tc := range []struct {
		name     string
		terms    int
		useUTVPI bool
		convex   bool
		mode     uint32
	}{
		{"difference", 1, true, false, smtcore.SMTArithmeticDifferenceLogic},
		{"utvpi", 2, true, false, smtcore.SMTArithmeticUTVPI},
		{"general", 3, true, false, smtcore.SMTArithmeticSimplex},
		{"disabled", 1, false, false, smtcore.SMTArithmeticSimplex},
		{"convex", 1, true, true, smtcore.SMTArithmeticSimplex},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, oldOwner, oldTransformer, _, closeBase := closureFixture(t)
			config := oldOwner.contexts.config
			closureMust(t, oldTransformer.Close())
			closureMust(t, oldOwner.Close())
			defer closeBase()
			context, err := NewContext(m, nil, 4, ReachableNoCache, config)
			closureMust(t, err)
			defer func() { closureMust(t, context.Close()) }()
			closureMust(t, context.SetUseUTVPI(tc.useUTVPI))
			closureMust(t, context.SetUseConvexClosureGeneralizer(tc.convex))
			closureMust(t, context.SetGenerateProofTrace(true))
			var pins []ast.ASTRef
			defer func() { closureMust(t, closeASTRefs(m, pins)) }()
			own := func(h ast.ASTHandle, cause error) ast.ASTHandle {
				closureMust(t, cause)
				ref, cause := m.NewRef(h)
				closureMust(t, cause)
				pins = append(pins, ref)
				return h
			}
			boolean := own(m.MkBoolSort())
			arithmetic, err := m.ArithmeticFamilyID()
			closureMust(t, err)
			realSort := own(m.MkFamilySort(arithmetic, 0, nil))
			makeDecl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				symbol, cause := m.Symbols.Intern(name)
				closureMust(t, cause)
				return own(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
			}
			pred := makeDecl("ite-rule-p", []ast.ASTHandle{realSort, realSort, realSort, boolean})
			good, bad := makeDecl("ite-rule-good", nil), makeDecl("ite-rule-bad", nil)
			rules, err := base.NewRuleManager(m)
			closureMust(t, err)
			defer func() { closureMust(t, rules.Close()) }()
			for _, decl := range []ast.ASTHandle{pred, good, bad} {
				closureMust(t, rules.RegisterPredicate(decl))
			}
			set, err := base.NewRuleSet(rules)
			closureMust(t, err)
			defer func() { closureMust(t, set.Close()) }()
			vars := []ast.ASTHandle{own(m.MkVar(0, realSort)), own(m.MkVar(1, realSort)), own(m.MkVar(2, realSort)), own(m.MkVar(3, boolean))}
			zero, one, two := own(m.ArithmeticRealNumeralText("0")), own(m.ArithmeticRealNumeralText("1")), own(m.ArithmeticRealNumeralText("2"))
			constraint := func(args []ast.ASTHandle) ast.ASTHandle {
				sum := args[0]
				for i := 1; i < tc.terms; i++ {
					sum = own(m.MkArithmeticAdd(sum, args[i]))
				}
				choice := own(m.MkIte(args[3], one, two))
				return own(m.MkArithmeticLE(sum, choice))
			}
			add := func(head ast.ASTHandle, tail []ast.ASTHandle, name string) {
				rule, cause := rules.NewRule(head, tail, nil, name)
				closureMust(t, cause)
				closureMust(t, set.Add(rule))
				closureMust(t, rule.Close())
			}
			add(own(m.MkApp(pred, vars)), []ast.ASTHandle{constraint(vars)}, "term-ite-fact")
			truth, falsity := own(m.MkTrue()), own(m.MkFalse())
			add(own(m.MkApp(good, nil)), []ast.ASTHandle{own(m.MkApp(pred, []ast.ASTHandle{two, zero, zero, falsity}))}, "good-query")
			add(own(m.MkApp(bad, nil)), []ast.ASTHandle{own(m.MkApp(pred, []ast.ASTHandle{two, zero, zero, truth}))}, "bad-query")
			closureMust(t, context.Initialize(set))
			if mode := context.owner.contexts.arithmeticMode(); mode != tc.mode {
				t.Fatal("unexpected source arithmetic fragment", mode, tc.mode)
			}
			transformer := context.transformers[pred]
			transition := transformer.rules[0].transition
			features, err := ast.NewStaticFeatures(m)
			closureMust(t, err)
			closureMust(t, features.Collect(transition))
			termITEs := features.NumITETerms
			closureMust(t, features.Close())
			blast := tc.mode == smtcore.SMTArithmeticDifferenceLogic || tc.mode == smtcore.SMTArithmeticUTVPI
			if blast && termITEs != 0 || !blast && termITEs == 0 {
				t.Fatal("term-ITE hoisting crossed the source fragment boundary", tc.mode, termITEs)
			}
			allowed := make(map[ast.ASTHandle]bool)
			grounded := make([]ast.ASTHandle, len(transformer.signature))
			for i, decl := range transformer.signature {
				decl, err = context.owner.OldToNew(decl, 0)
				closureMust(t, err)
				allowed[decl] = true
				grounded[i] = own(m.MkApp(decl, nil))
			}
			seen := make(map[ast.ASTHandle]bool)
			stack := []ast.ASTHandle{transition}
			for len(stack) != 0 {
				h := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if seen[h] {
					continue
				}
				seen[h] = true
				kind, cause := m.GetKind(h)
				closureMust(t, cause)
				if kind != ast.ASTApp {
					t.Fatal("normalized transition is not ground", kind)
				}
				decl, cause := m.GetDecl(h)
				closureMust(t, cause)
				family, cause := m.GetFamilyID(decl)
				closureMust(t, cause)
				if family == ast.NullFamilyID && !allowed[decl] {
					t.Fatal("term-ITE normalization introduced an untracked declaration", decl)
				}
				args, cause := m.AppArgumentView(h)
				closureMust(t, cause)
				for i := uint32(0); i < args.Count(); i++ {
					arg, cause := args.At(i)
					closureMust(t, cause)
					stack = append(stack, arg)
				}
			}
			original := constraint(grounded)
			different := own(m.MkNot(own(m.MkEq(original, transition))))
			solver, err := smtcore.NewSMTSolver(m, nil, "", config)
			closureMust(t, err)
			defer func() { closureMust(t, solver.Close()) }()
			closureMust(t, solver.Push())
			closureMust(t, solver.Assert(different))
			status, err := solver.CheckSat(nil)
			closureMust(t, err)
			if status != lbool.False {
				t.Fatal("normalization changed the grounded rule", status)
			}
			closureMust(t, solver.Pop(1))
			status, err = context.Solve(good)
			closureMust(t, err)
			if status != lbool.True {
				t.Fatal("selected ITE branch should be reachable", status)
			}
			proof, err := context.Proof(good)
			closureMust(t, err)
			pins = append(pins, proof)
			proofHandle, err := m.RefGet(proof)
			closureMust(t, err)
			checker, err := proofs.NewChecker(m)
			closureMust(t, err)
			valid, conditions, err := checker.Check(proofHandle)
			closureMust(t, err)
			defer func() {
				for _, condition := range conditions {
					closureMust(t, m.DecRef(condition))
				}
			}()
			if !valid {
				t.Fatal("hoisted-rule proof failed validation")
			}
			checkValid := func(formula ast.ASTHandle) {
				used, cause := ast.NewASTUsedVars(m)
				closureMust(t, cause)
				closureMust(t, used.Process(formula))
				if count := used.MaxFoundVarIndexPlusOne(); count != 0 {
					sorts := make([]ast.ASTHandle, count)
					names := make([]symbol.Symbol, count)
					for i := uint32(0); i < count; i++ {
						sorts[count-1-i] = used.Contains(i)
						if sorts[count-1-i].IsNull() {
							sorts[count-1-i] = boolean
						}
						names[count-1-i] = symbol.Numerical(i)
					}
					formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
				}
				closureMust(t, solver.Push())
				closureMust(t, solver.Assert(own(m.MkNot(formula))))
				status, cause := solver.CheckSat(nil)
				closureMust(t, cause)
				closureMust(t, solver.Pop(1))
				if status != lbool.False {
					t.Fatal("proof condition or conclusion was not valid", status)
				}
			}
			for _, condition := range conditions {
				checkValid(condition)
			}
			fact, err := m.GetProofFact(proofHandle)
			closureMust(t, err)
			checkValid(own(m.MkEq(fact, own(m.MkApp(good, nil)))))
			status, err = context.Solve(bad)
			closureMust(t, err)
			if status != lbool.False {
				t.Fatal("unselected ITE branch should be unreachable", status)
			}
		})
	}
}
