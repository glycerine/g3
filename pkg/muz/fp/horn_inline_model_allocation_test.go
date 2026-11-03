package fp

// Source dl_mk_rule_inliner.cpp installs the Horn model converter, which is
// replayed before dl_mk_quantifier_abstraction.cpp restores array predicates.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func newHornInlineArrayModelFixture(t *testing.T) (*fpRuleNameFixture, ast.ASTHandle) {
	t.Helper()
	f := newQueryVariablesOwnershipFixture(t)
	fragmentMust(t, f.e.SelectEngine("spacer"))
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.xform.slice", "false"))
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
	for _, key := range []string{"quantify_arrays", "instantiate_quantifiers"} {
		fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.xform."+key, "true"))
	}
	boolean := f.own(f.m.MkBoolSort())
	array := f.own(f.m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean))
	a := f.own(f.m.MkVar(0, array))
	predicate, application := f.predicate("qa-public-P", a)
	_, goal := f.predicate("qa-public-Goal")
	truth := f.own(f.m.MkTrue())
	family, err := f.m.ArrayFamilyID()
	fragmentMust(t, err)
	selected := f.own(f.m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, truth}))
	// P(a) holds exactly when a[true] does; Goal asks for its contradiction.
	seed := f.own(f.m.MkImplies(selected, application))
	fragmentMust(t, f.e.AddRule(seed, "seed"))
	tail := f.own(f.m.MkAndArgs([]ast.ASTHandle{application, f.own(f.m.MkNot(selected))}))
	step := f.own(f.m.MkImplies(tail, goal))
	fragmentMust(t, f.e.AddRule(step, "bad"))
	status, err := f.e.QueryExpression(goal)
	fragmentMust(t, err)
	if status != lbool.False || f.e.queryClone == nil || !f.e.queryReady {
		t.Fatal("array contradiction did not publish an unreachable query", status)
	}
	inline, quantified := 0, 0
	for _, conversion := range f.e.queryClone.spacerConversions {
		if conversion.kind == hornConvertInline {
			inline += len(conversion.inline)
		}
		if conversion.kind == hornConvertQuantifier {
			quantified += len(conversion.predicates)
		}
	}
	if inline == 0 || quantified == 0 || len(f.e.queryClone.spacerQuantified) == 0 {
		t.Fatal("public query did not install both model conversion stages", inline, quantified)
	}
	return f, predicate
}

func hornInlineArrayModelBody(t *testing.T, f *fpRuleNameFixture, md *model.Model, predicate ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	interpretation, err := md.Core().FunctionInterpretation(predicate)
	fragmentMust(t, err)
	if interpretation == nil || interpretation.Arity() != 1 {
		t.Fatal("original unary array predicate was not reconstructed")
	}
	body, err := interpretation.Else()
	fragmentMust(t, err)
	forall, err := f.m.IsForall(body)
	fragmentMust(t, err)
	if !forall {
		t.Fatal("array predicate model lost its quantified index")
	}
	return body
}

func TestHornInlineModelAllocationFailureAndRetry(t *testing.T) {
	failures, completed := 0, false
	for allowance := int64(0); allowance < 512 && !completed; allowance++ {
		if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f, predicate := newHornInlineArrayModelFixture(t)
			// Settle reusable pages and manager caches. Each export still makes
			// fresh model, substitution, and function-interpretation owners.
			warm, err := f.e.ExpressionModel()
			fragmentMust(t, err)
			hornInlineArrayModelBody(t, f, warm, predicate)
			fragmentMust(t, warm.Close())
			before := contextDisplayLiveCounts(f.m.Stats())
			clone, head, revision := f.e.queryClone, f.e.queryClonePredicate, f.e.queryCloneRevision
			checkUnchanged := func() {
				t.Helper()
				if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
					t.Fatal("inline model export leaked AST ownership", before, after)
				}
				if f.e.queryClone != clone || f.e.queryClonePredicate != head || f.e.queryCloneRevision != revision || !f.e.queryReady || f.e.lastResult != lbool.False {
					t.Fatal("model export changed the published query")
				}
			}
			f.m.Budget().FailAfter(allowance)
			md, cause := f.e.ExpressionModel()
			f.m.Budget().FailAfter(-1)
			if cause != nil {
				failures++
				if md != nil {
					fragmentMust(t, md.Close())
					t.Fatal("failed conversion published a partial model")
				}
				if !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("model conversion lost allocation failure", cause)
				}
				var cleanup memory.ConstructionFailure
				if errors.As(cause, &cleanup) {
					fragmentMust(t, cleanup.Cleanup())
				}
			} else {
				completed = true
				hornInlineArrayModelBody(t, f, md, predicate)
				fragmentMust(t, md.Close())
			}
			checkUnchanged()
			md, err = f.e.ExpressionModel()
			fragmentMust(t, err)
			hornInlineArrayModelBody(t, f, md, predicate)
			fragmentMust(t, md.Close())
			checkUnchanged()
			if completed {
				md, err = f.e.ExpressionModel()
				fragmentMust(t, err)
				defer func() { fragmentMust(t, md.Close()) }()
				checkHornInlineArrayModelValues(t, f, hornInlineArrayModelBody(t, f, md, predicate), predicate)
			}
		}) {
			break
		}
	}
	if failures == 0 || !completed {
		t.Fatal("model campaign missed failure or completion", failures, completed)
	}
}

func checkHornInlineArrayModelValues(t *testing.T, f *fpRuleNameFixture, body, predicate ast.ASTHandle) {
	t.Helper()
	array, err := f.m.GetDomain(predicate, 0)
	fragmentMust(t, err)
	truth, falsity := f.own(f.m.MkTrue()), f.own(f.m.MkFalse())
	allTrue, allFalse := f.own(f.m.MkConstArray(array, truth)), f.own(f.m.MkConstArray(array, falsity))
	family, err := f.m.ArrayFamilyID()
	fragmentMust(t, err)
	onlyTrue := f.own(f.m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{allFalse, truth, truth}))
	onlyFalse := f.own(f.m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{allTrue, truth, falsity}))
	boolean, err := f.m.GetSort(truth)
	fragmentMust(t, err)
	count, err := f.m.GetNumDeclarations(body)
	fragmentMust(t, err)
	indexSort, err := f.m.GetDeclarationSort(body, 0)
	fragmentMust(t, err)
	if count != 1 || indexSort != boolean {
		t.Fatal("unexpected universal array-index binder", count, indexSort)
	}
	exists, err := f.m.GetQuantifierBody(body)
	fragmentMust(t, err)
	kind, err := f.m.GetKind(exists)
	fragmentMust(t, err)
	if kind != ast.ASTQuantifier {
		t.Fatal("inline model lost its existential array witness")
	}
	forall, err := f.m.IsForall(exists)
	fragmentMust(t, err)
	count, err = f.m.GetNumDeclarations(exists)
	fragmentMust(t, err)
	witnessSort, err := f.m.GetDeclarationSort(exists, 0)
	fragmentMust(t, err)
	if forall || count != 1 || witnessSort != array {
		t.Fatal("unexpected existential array-witness binder", forall, count, witnessSort)
	}
	inner, err := f.m.GetQuantifierBody(exists)
	fragmentMust(t, err)
	reduced, err := f.m.NewRef(ast.ASTHandle{})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&reduced)) }()
	// The actual model is forall i:Bool, exists witness:Array Bool Bool.
	// All four arrays and both indices are finite: exhaust all 32 valuations
	// instead of asking a solver to discover quantified array witnesses.
	arrays := []ast.ASTHandle{allTrue, allFalse, onlyTrue, onlyFalse}
	for i, value := range arrays {
		allIndices := true
		for _, index := range []ast.ASTHandle{falsity, truth} {
			foundWitness := false
			for _, witness := range arrays {
				// Removing both binders leaves #0=witness, #1=index, #2=value;
				// RewriteWithBindings maps variable zero to the last argument.
				fragmentMust(t, f.e.guard.RewriteWithBindings(inner, []ast.ASTHandle{value, index, witness}, reduced))
				formula, err := f.m.RefGet(reduced)
				fragmentMust(t, err)
				if !f.m.IsTrue(formula) && !f.m.IsFalse(formula) {
					t.Fatal("fully instantiated array model did not reduce to a Boolean", i, formula)
				}
				foundWitness = foundWitness || f.m.IsTrue(formula)
			}
			allIndices = allIndices && foundWitness
		}
		if allIndices != (i%2 == 0) {
			t.Fatal("reconstructed predicate disagrees with a[true]", i, allIndices)
		}
	}
}
