package base_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func checkHornNameSymbolProof(t *testing.T, m *ast.ASTManager, rule *base.Rule, check bool) {
	t.Helper()
	proof, found, err := rule.Proof()
	ruleMust(t, err)
	if !found || proof.IsNull() {
		t.Fatal("typed HNF ingestion lost the clause proof")
	}
	formula, err := rule.ToFormula()
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&formula)) }()
	want, err := m.RefGet(formula)
	ruleMust(t, err)
	fact, err := m.GetProofFact(proof)
	ruleMust(t, err)
	if fact != want {
		t.Fatal("typed HNF clause proof proves a different formula")
	}
	if check {
		checker, err := proofs.NewChecker(m)
		ruleMust(t, err)
		valid, side, err := checker.Check(proof)
		ruleMust(t, err)
		ruleMust(t, checker.ReleaseSideConditions(side))
		if !valid {
			t.Fatal("typed HNF clause proof failed independent checking")
		}
	}
}

func hornNameSymbolAsserted(t *testing.T, m *ast.ASTManager, formula ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := m.MkProofAsserted(formula)
	ruleMust(t, err)
	holdRuleNameAST(t, m, proof)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	return proof
}

func TestHornNameSymbolsPreserveSplitIdentity(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		t.Run("proof="+strconv.FormatBool(withProof), func(t *testing.T) {
			f := newRuleSetFixture(t, "P", "Q", "H")
			body, err := f.manager.MkOr(f.app["P"], f.app["Q"])
			ruleMust(t, err)
			formula, err := f.manager.MkImplies(body, f.app["H"])
			ruleMust(t, err)
			holdRuleNameAST(t, f.manager, formula)
			var proof ast.ASTHandle
			if withProof {
				proof = hornNameSymbolAsserted(t, f.manager, formula)
			}
			for _, tc := range ruleNameSymbolCases(t, f.manager) {
				t.Run(tc.label, func(t *testing.T) {
					set, err := base.NewRuleSet(f.rules)
					ruleMust(t, err)
					t.Cleanup(func() { ruleMust(t, set.Close()) })
					if withProof {
						err = f.rules.AddFormulaWithProofAndSymbol(set, formula, proof, tc.symbol)
					} else {
						err = f.rules.AddFormulaWithSymbol(set, formula, tc.symbol)
					}
					ruleMust(t, err)
					if count, err := set.Len(); err != nil || count != 2 {
						t.Fatal("typed HNF input did not split into two clauses", count, err)
					}
					seen := make(map[ast.ASTHandle]bool)
					for i := 0; i < 2; i++ {
						rule, err := set.Rule(i)
						ruleMust(t, err)
						t.Cleanup(func() { ruleMust(t, rule.Close()) })
						checkRuleNameSymbol(t, rule, tc)
						head, err := rule.Head()
						ruleMust(t, err)
						size, err := rule.TailSize()
						ruleMust(t, err)
						if head != f.app["H"] || size != 1 {
							t.Fatal("split changed the head or tail shape", head, size)
						}
						tail, negative, err := rule.Tail(0)
						ruleMust(t, err)
						if negative || seen[tail] || (tail != f.app["P"] && tail != f.app["Q"]) {
							t.Fatal("split did not preserve both alternatives", tail, negative)
						}
						seen[tail] = true
						if withProof {
							checkHornNameSymbolProof(t, f.manager, rule, true)
						}
					}
					if mode, err := f.manager.ProofMode(); err != nil || mode != ast.ASTProofDisabled {
						t.Fatal("typed HNF ingestion changed the caller's proof mode", mode, err)
					}
				})
			}
		})
	}
}

func TestHornNameSymbolsSelectSourceFreshPrefixes(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		t.Run("proof="+strconv.FormatBool(withProof), func(t *testing.T) {
			f := newRuleSetFixture(t, "P", "Q", "S", "H")
			disjunction, err := f.manager.MkOr(f.app["P"], f.app["Q"])
			ruleMust(t, err)
			body, err := f.manager.MkAnd(disjunction, f.app["S"])
			ruleMust(t, err)
			formula, err := f.manager.MkImplies(body, f.app["H"])
			ruleMust(t, err)
			holdRuleNameAST(t, f.manager, formula)
			var proof ast.ASTHandle
			if withProof {
				proof = hornNameSymbolAsserted(t, f.manager, formula)
			}
			cases := append(ruleNameSymbolCases(t, f.manager), ruleNameSymbolCase{
				// Source AMD64 symbol(unsigned) accepts the full uint32 range;
				// symbol::str uses UNBOXINT's signed int spelling.
				"high-numeric", usymbol.Numerical(^uint32(0)), "k!-1", "k!-1",
			})
			for _, tc := range cases {
				t.Run(tc.label, func(t *testing.T) {
					set, err := base.NewRuleSet(f.rules)
					ruleMust(t, err)
					t.Cleanup(func() { ruleMust(t, set.Close()) })
					if withProof {
						err = f.rules.AddFormulaWithProofAndSymbol(set, formula, proof, tc.symbol)
					} else {
						err = f.rules.AddFormulaWithSymbol(set, formula, tc.symbol)
					}
					ruleMust(t, err)
					if count, err := set.Len(); err != nil || count != 3 {
						t.Fatal("typed HNF input did not introduce and split a definition", count, err)
					}
					var fresh ast.ASTHandle
					definitions := 0
					for i := 0; i < 3; i++ {
						rule, err := set.Rule(i)
						ruleMust(t, err)
						t.Cleanup(func() { ruleMust(t, rule.Close()) })
						checkRuleNameSymbol(t, rule, tc)
						head, err := rule.HeadDeclaration()
						ruleMust(t, err)
						if head != f.decl["H"] {
							if !fresh.IsNull() && head != fresh {
								t.Fatal("one definition received different fresh predicates")
							}
							fresh = head
							definitions++
						}
						if withProof {
							// Definition introduction has a source congruence proof
							// that the source checker rejects on the main clause.
							checkHornNameSymbolProof(t, f.manager, rule, false)
						}
					}
					if fresh.IsNull() || definitions != 2 {
						t.Fatal("missing split fresh definition", definitions)
					}
					name, err := f.manager.GetASTName(fresh)
					ruleMust(t, err)
					text, err := f.manager.Symbols.String(name)
					ruleMust(t, err)
					prefix := tc.text
					if tc.symbol.IsNull() {
						prefix = "P"
					}
					// hnf.cpp supplies a non-null empty suffix, so the source
					// fresh declaration has two separators before its counter.
					prefix += "!!"
					if !strings.HasPrefix(text, prefix) {
						t.Fatalf("fresh predicate %q, want prefix %q", text, prefix)
					}
					if _, err := strconv.ParseUint(strings.TrimPrefix(text, prefix), 10, 64); err != nil {
						t.Fatal("fresh predicate is missing its source counter", text, err)
					}
				})
			}
		})
	}
}

func TestHornNameSymbolsSurviveIngestionUnboundReduction(t *testing.T) {
	f := newUnboundFixture(t)
	m := f.manager
	arithmetic, err := m.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	x, err := m.MkVar(1, integer)
	ruleMust(t, err)
	y, err := m.MkVar(0, integer)
	ruleMust(t, err)
	head := f.predicate(t, "typed-ingest-head", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	tail := f.predicate(t, "typed-ingest-tail", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	tailDecl, err := m.GetDecl(tail)
	ruleMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	ruleMust(t, err)
	equality, err := m.MkEq(y, three)
	ruleMust(t, err)
	body, err := m.MkAnd(tail, equality)
	ruleMust(t, err)
	implication, err := m.MkImplies(body, head)
	ruleMust(t, err)
	formula, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer, integer}, Names: []usymbol.Symbol{usymbol.Numerical(0), usymbol.Numerical(1)}, Body: implication})
	ruleMust(t, err)
	holdRuleNameAST(t, m, formula)
	proof := hornNameSymbolAsserted(t, m, formula)
	for _, tc := range ruleNameSymbolCases(t, m) {
		for _, withProof := range []bool{false, true} {
			t.Run(tc.label+"/proof="+strconv.FormatBool(withProof), func(t *testing.T) {
				set, err := base.NewRuleSet(f.rules)
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, set.Close()) })
				if withProof {
					err = f.rules.AddFormulaWithProofAndSymbolAndUnboundFix(set, formula, proof, tc.symbol, f.lite, false)
				} else {
					err = f.rules.AddFormulaWithSymbolAndUnboundFix(set, formula, tc.symbol, f.lite, false)
				}
				ruleMust(t, err)
				if count, err := set.Len(); err != nil || count != 1 {
					t.Fatal("typed QE-lite ingestion rule count", count, err)
				}
				rule, err := set.Rule(0)
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, rule.Close()) })
				checkRuleNameSymbol(t, rule, tc)
				if size, err := rule.TailSize(); err != nil || size != 1 {
					t.Fatal("typed QE-lite ingestion did not eliminate the unbound equality", size, err)
				}
				got, err := rule.TailDeclaration(0)
				ruleMust(t, err)
				if got != tailDecl {
					t.Fatal("typed QE-lite ingestion changed the bound predicate")
				}
				if withProof {
					checkHornNameSymbolProof(t, m, rule, true)
				}
			})
		}
	}
}
