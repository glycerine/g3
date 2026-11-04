package fp

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPendingFlushSourceProofMode(t *testing.T) {
	for _, trace := range []bool{false, true} {
		for _, initial := range []ast.ASTProofMode{ast.ASTProofDisabled, ast.ASTProofEnabled} {
			t.Run(fmt.Sprintf("trace=%t/initial=%v", trace, initial), func(t *testing.T) {
				f := newFPRuleNameFixture(t)
				fragmentMust(t, f.m.ToggleProofMode(initial))
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.generate_proof_trace", fmt.Sprint(trace)))
				_, fact := f.predicate("flush-proof-fact")
				fragmentMust(t, f.e.AddRule(fact, "flush-proof-fact"))
				fragmentMust(t, f.e.flushPendingRules())
				mode, err := f.m.ProofMode()
				fragmentMust(t, err)
				if mode != initial {
					t.Fatalf("manager proof mode after flush = %v, want %v", mode, initial)
				}
				count, err := f.e.set.Len()
				fragmentMust(t, err)
				if count != 1 {
					t.Fatalf("got %d rules, want 1", count)
				}
				rule, err := f.e.set.Rule(0)
				fragmentMust(t, err)
				defer func() { fragmentMust(t, rule.Close()) }()
				proof, present, err := rule.Proof()
				fragmentMust(t, err)
				if present != trace {
					t.Fatalf("proof present = %t, want %t", present, trace)
				}
				if present {
					conclusion, err := f.m.GetProofFact(proof)
					fragmentMust(t, err)
					if conclusion != fact {
						t.Fatalf("proof conclusion %v does not match asserted fact %v", conclusion, fact)
					}
				}
			})
		}
	}
}

func TestPendingFlushProofUsesBoundFormula(t *testing.T) {
	f := newFPRuleNameFixture(t)
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	boolean := f.own(f.m.MkBoolSort())
	variable := f.own(f.m.MkVar(0, boolean))
	_, open := f.predicate("flush-proof-variable", variable)
	fragmentMust(t, f.e.AddRule(open, "flush-proof-variable"))
	fragmentMust(t, f.e.flushPendingRules())
	rule, err := f.e.set.Rule(0)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, rule.Close()) }()
	proof, present, err := rule.Proof()
	fragmentMust(t, err)
	if !present {
		t.Fatal("bound rule has no proof")
	}
	conclusion, err := f.m.GetProofFact(proof)
	fragmentMust(t, err)
	formula, err := rule.ToFormula()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&formula)) }()
	want, err := f.m.RefGet(formula)
	fragmentMust(t, err)
	if conclusion != want {
		t.Fatalf("proof conclusion %v does not match bound rule %v", conclusion, want)
	}
	if conclusion == open {
		t.Fatal("proof still asserts the open input")
	}
}

// Source mk_horn_rule uses symmetry(DER) when converting an open Horn
// formula to the universally quantified rule formula.
func TestHornRuleProofUsesDERForUniversalClosure(t *testing.T) {
	f := newFPRuleNameFixture(t)
	fragmentMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	boolean := f.own(f.m.MkBoolSort())
	variable := f.own(f.m.MkVar(0, boolean))
	_, open := f.predicate("proof-der", variable)
	asserted := f.own(f.m.MkProofAsserted(open))
	rule, err := f.e.rules.NewRuleFromFormulaWithProof(open, asserted, "der")
	fragmentMust(t, err)
	defer func() { fragmentMust(t, rule.Close()) }()
	proof, found, err := rule.Proof()
	fragmentMust(t, err)
	if !found {
		t.Fatal("missing rule proof")
	}
	isMP, err := f.m.IsProofKind(proof, ast.ASTPrModusPonens)
	fragmentMust(t, err)
	if !isMP {
		t.Fatal("expected modus ponens for closure")
	}
	symmetry, err := f.m.GetProofParent(proof, 1)
	fragmentMust(t, err)
	isSymmetry, err := f.m.IsProofKind(symmetry, ast.ASTPrSymmetry)
	fragmentMust(t, err)
	if !isSymmetry {
		t.Fatal("expected symmetric DER proof")
	}
	der, err := f.m.GetProofParent(symmetry, 0)
	fragmentMust(t, err)
	isDER, err := f.m.IsProofKind(der, ast.ASTPrDER)
	fragmentMust(t, err)
	if !isDER {
		t.Fatal("expected DER proof")
	}
}

func TestPendingFlushReassertsAfterBindingRegisteredVariable(t *testing.T) {
	f := newFPRuleNameFixture(t)
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	boolean := f.own(f.m.MkBoolSort())
	name, err := f.m.Symbols.Intern("proof-bound-constant")
	fragmentMust(t, err)
	variable := f.own(f.m.MkConst(name, boolean))
	fragmentMust(t, f.e.RegisterVariable(variable))
	_, original := f.predicate("proof-bound-predicate", variable)
	boundRef, err := f.e.binder.Bind(original, true)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&boundRef)) }()
	bound, err := f.m.RefGet(boundRef)
	fragmentMust(t, err)
	if bound == original {
		t.Fatal("fixture did not bind the registered variable")
	}
	fragmentMust(t, f.e.AddRule(original, "proof-bound"))
	fragmentMust(t, f.e.flushPendingRules())
	rule, err := f.e.set.Rule(0)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, rule.Close()) }()
	proof, found, err := rule.Proof()
	fragmentMust(t, err)
	if !found {
		t.Fatal("bound rule has no proof")
	}
	work := []ast.ASTHandle{proof}
	assertedBound := false
	for len(work) > 0 {
		p := work[len(work)-1]
		work = work[:len(work)-1]
		asserted, err := f.m.IsProofKind(p, ast.ASTPrAsserted)
		fragmentMust(t, err)
		if asserted {
			fact, err := f.m.GetProofFact(p)
			fragmentMust(t, err)
			if fact == original {
				t.Fatal("proof still asserts the unbound input")
			}
			if fact == bound {
				assertedBound = true
			}
		}
		n, err := f.m.GetProofNumParents(p)
		fragmentMust(t, err)
		for i := uint32(0); i < n; i++ {
			parent, err := f.m.GetProofParent(p, i)
			fragmentMust(t, err)
			work = append(work, parent)
		}
	}
	if !assertedBound {
		t.Fatal("proof does not assert the bound input")
	}
}
