package transforms_test

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func newCompoundSliceFailureFixture(t *testing.T, inputIndex int) (*coneFixture, *transforms.SliceProofConverter, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("compound conversion fixture leaked bytes", budget.Used())
		}
	})
	f := newConeFixture(t)
	budget = f.manager.Budget()
	m := f.manager
	boolean, err := m.MkBoolSort()
	coneMust(t, err)
	zero, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	one, err := m.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	coneMust(t, err)
	integer, err := m.GetSort(zero)
	coneMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		coneMust(t, e)
		d, e := m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		coneMust(t, f.rules.RegisterPredicate(d))
		return d
	}
	p, goal := declare("slice-compound-P", []ast.ASTHandle{integer, integer}), declare("slice-compound-goal", nil)
	x, err := m.MkVar(0, integer)
	coneMust(t, err)
	y, err := m.MkVar(1, integer)
	coneMust(t, err)
	pApp, err := m.MkApp(p, []ast.ASTHandle{x, y})
	coneMust(t, err)
	goalApp, err := m.MkApp(goal, nil)
	coneMust(t, err)
	guard, err := m.MkEq(y, zero)
	coneMust(t, err)
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
	}{{pApp, nil}, {goalApp, []ast.ASTHandle{pApp, guard}}} {
		r, e := f.rules.NewRule(spec.head, spec.tails, nil, "")
		coneMust(t, e)
		coneMust(t, f.rules.MkRuleAssertedProof(r))
		coneMust(t, f.set.Add(r))
		coneMust(t, r.Close())
	}
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	plugin := transforms.NewRuleSlicePlugin(m, f.rules, 1)
	plugin.SetGenerateProofTrace(true)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	changed, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !changed {
		t.Fatal("expected slicing")
	}
	derived, found := plugin.DerivedPredicate(p)
	if !found {
		t.Fatal("missing sliced predicate")
	}
	converter := plugin.TakeProofConverter()
	coneMust(t, transformer.Close())
	if converter == nil {
		t.Fatal("missing converter")
	}
	t.Cleanup(func() { coneMust(t, converter.Close()) })
	rules, err := f.set.PredicateRules(derived)
	coneMust(t, err)
	if len(rules) != 1 {
		t.Fatal("expected one sliced fact")
	}
	proof, found, err := rules[0].Proof()
	coneMust(t, err)
	coneMust(t, rules[0].Close())
	if !found {
		t.Fatal("missing sliced fact proof")
	}
	argument := zero
	if inputIndex >= 0 {
		argument, err = m.MkVar(uint32(inputIndex), integer)
		coneMust(t, err)
	}
	compound, err := m.MkArithmeticAdd(argument, one)
	coneMust(t, err)
	expectedTerm := compound
	if inputIndex >= 0 {
		expectedTerm, err = m.MkArithmeticAdd(y, one)
		coneMust(t, err)
	}
	conclusion, err := m.MkApp(derived, []ast.ASTHandle{compound})
	coneMust(t, err)
	scope, err := ast.NewScopedProof(m)
	coneMust(t, err)
	node, err := m.MkProofHyperResolve([]ast.ASTHandle{proof}, conclusion, nil, [][]ast.ASTHandle{{compound}})
	coneMust(t, err)
	pin, err := m.NewRef(node)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, m.DelRef(&pin)) })
	coneMust(t, scope.Close())
	expectedBody, err := m.MkApp(p, []ast.ASTHandle{x, expectedTerm})
	coneMust(t, err)
	name, err := m.Symbols.Intern("original-unused-column")
	coneMust(t, err)
	sorts, names := []ast.ASTHandle{integer}, []symbol.Symbol{name}
	if inputIndex >= 0 {
		sorts = append(sorts, integer)
		names = append(names, symbol.Numerical(1))
	}
	expected, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: expectedBody})
	coneMust(t, err)
	expectedRef, err := m.NewRef(expected)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, m.DelRef(&expectedRef)) })
	return f, converter, node, expected
}

func TestRuleSliceCompoundHeadAllocationFailureAndRetry(t *testing.T) {
	testRuleSliceCompoundHeadAllocationFailureAndRetry(t, false)
}

func TestRuleSliceCompoundHeadNodeOwnershipFailureAndRetry(t *testing.T) {
	testRuleSliceCompoundHeadAllocationFailureAndRetry(t, true)
}

func testRuleSliceCompoundHeadAllocationFailureAndRetry(t *testing.T, countNodes bool) {
	for _, inputIndex := range []int{-1, 0, 4} {
		t.Run(fmt.Sprintf("variable=%d", inputIndex), func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 4096 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f, converter, input, expected := newCompoundSliceFailureFixture(t, inputIndex)
					m := f.manager
					if countNodes {
						// The basic plugin owns proof declarations after first use.
						// The separate cold campaign still exercises their allocation.
						// Prime only those caches for exact temporary-node accounting.
						warmCompoundSliceProofDeclarations(t, m)
					}
					before := m.Stats().LiveReferences
					nodesBefore := m.Stats().LiveNodes
					inputCount, err := m.GetRefCount(input)
					coneMust(t, err)
					mode, err := m.ProofMode()
					coneMust(t, err)
					m.Budget().FailAfter(stop)
					result, err := converter.Convert(input)
					m.Budget().FailAfter(-1)
					if err != nil {
						failures++
						var cleanup memory.ConstructionFailure
						if errors.As(err, &cleanup) {
							coneMust(t, cleanup.Cleanup())
						}
						var failure memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
							t.Fatal("unexpected conversion failure", stop, err)
						}
						if result != (ast.ASTRef{}) {
							t.Fatal("failed conversion published a proof")
						}
					} else {
						success = true
						coneMust(t, m.DelRef(&result))
					}
					if got := m.Stats().LiveReferences; got != before {
						t.Fatal("conversion leaked references", stop, got, before)
					}
					if got := m.Stats().LiveNodes; countNodes && got != nodesBefore {
						t.Fatal("conversion leaked unowned AST nodes", stop, got, nodesBefore, err)
					}
					if current, err := m.GetRefCount(input); err != nil || current != inputCount {
						t.Fatal("conversion changed caller proof ownership", current, inputCount, err)
					}
					if current, err := m.ProofMode(); err != nil || current != mode {
						t.Fatal("conversion changed proof mode", current, err)
					}
					result, err = converter.Convert(input)
					coneMust(t, err)
					defer func() {
						if result != (ast.ASTRef{}) {
							coneMust(t, m.DelRef(&result))
						}
					}()
					node, err := m.RefGet(result)
					coneMust(t, err)
					checker, err := proofs.NewChecker(m)
					coneMust(t, err)
					valid, side, err := checker.Check(node)
					coneMust(t, err)
					defer func() { coneMust(t, checker.ReleaseSideConditions(side)) }()
					if !valid {
						t.Fatal("converted proof is invalid")
					}
					fact, err := m.GetProofFact(node)
					coneMust(t, err)
					equal, err := m.MkEq(fact, expected)
					coneMust(t, err)
					equalRef, err := m.NewRef(equal)
					coneMust(t, err)
					defer func() {
						if equalRef != (ast.ASTRef{}) {
							coneMust(t, m.DelRef(&equalRef))
						}
					}()
					solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: f.globals.Context, Globals: f.globals})
					coneMust(t, err)
					defer func() {
						if solver != nil {
							coneMust(t, solver.Close())
						}
					}()
					for _, condition := range append(side, equal) {
						coneMust(t, solver.Push())
						checked := ast.ASTRef{}
						if inputIndex >= 0 {
							checked = groundCompoundSliceCondition(t, m, condition)
							condition, err = m.RefGet(checked)
							coneMust(t, err)
						}
						negated, err := m.MkNot(condition)
						coneMust(t, err)
						coneMust(t, solver.Assert(negated))
						status, err := solver.CheckSat(nil)
						coneMust(t, err)
						coneMust(t, solver.Pop(1))
						if checked != (ast.ASTRef{}) {
							coneMust(t, m.DelRef(&checked))
						}
						if status != lbool.False {
							t.Fatal("invalid proof side condition or original-column meaning", stop, status)
						}
					}
					coneMust(t, solver.Close())
					solver = nil
					coneMust(t, checker.ReleaseSideConditions(side))
					side = nil
					coneMust(t, m.DelRef(&equalRef))
					coneMust(t, m.DelRef(&result))
					if got := m.Stats().LiveReferences; got != before {
						t.Fatal("verified retry leaked references", got, before)
					}
				})
				if t.Failed() {
					break
				}
			}
			if !success || failures == 0 {
				t.Fatal("missing conversion failure/success frontier", failures, success)
			}
		})
	}
}

func warmCompoundSliceProofDeclarations(t *testing.T, m *ast.ASTManager) {
	t.Helper()
	scope, err := ast.NewScopedProof(m)
	coneMust(t, err)
	truth, err := m.MkTrue()
	coneMust(t, err)
	conjunction, err := m.MkAndArgs([]ast.ASTHandle{truth, truth})
	coneMust(t, err)
	conjunctionOwner, err := m.NewRef(conjunction)
	coneMust(t, err)
	asserted, err := m.MkProofAsserted(conjunction)
	coneMust(t, err)
	rewrite, err := m.MkProofRewrite(conjunction, truth)
	coneMust(t, err)
	translated, err := m.MkProofModusPonens(asserted, rewrite)
	coneMust(t, err)
	owner, err := m.NewRef(translated)
	coneMust(t, err)
	coneMust(t, m.DelRef(&owner))
	der, err := m.MkProofDER(conjunction, truth)
	coneMust(t, err)
	derOwner, err := m.NewRef(der)
	coneMust(t, err)
	symmetry, err := m.MkProofSymmetry(der)
	coneMust(t, err)
	symmetryOwner, err := m.NewRef(symmetry)
	coneMust(t, err)
	coneMust(t, m.DelRef(&symmetryOwner))
	coneMust(t, m.DelRef(&derOwner))
	coneMust(t, m.DelRef(&conjunctionOwner))
	coneMust(t, scope.Close())
}

// These symbolic fixtures produce forall a,b,c. c=b+1 => P(a,c).
// Eliminate the defining equality by its explicit witness, preserving a and b
// as independent arbitrary constants. This avoids asking SMT to discover a
// quantified equivalence while still checking every proof side condition.
func groundCompoundSliceCondition(t *testing.T, m *ast.ASTManager, condition ast.ASTHandle) ast.ASTRef {
	t.Helper()
	subst, err := rewriter.NewVarSubstitution(m, false)
	coneMust(t, err)
	defer func() { coneMust(t, subst.Close()) }()
	var roots []ast.ASTRef
	defer func() {
		for i := range roots {
			coneMust(t, m.DelRef(&roots[i]))
		}
	}()
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		r, e := m.NewRef(h)
		coneMust(t, e)
		roots = append(roots, r)
		return h
	}
	apply := func(h ast.ASTHandle, args []ast.ASTHandle) ast.ASTHandle {
		r, e := m.NewRef(ast.ASTHandle{})
		coneMust(t, e)
		roots = append(roots, r)
		coneMust(t, subst.Substitute(h, args, r))
		v, e := m.RefGet(r)
		coneMust(t, e)
		return v
	}
	var integer ast.ASTHandle
	normalize := func(q ast.ASTHandle) ast.ASTHandle {
		forall, e := m.IsForall(q)
		coneMust(t, e)
		if !forall {
			t.Fatal("expected universal formula")
		}
		n, e := m.GetNumDeclarations(q)
		coneMust(t, e)
		if n != 2 && n != 3 {
			t.Fatal("unexpected symbolic binder bank", n)
		}
		for i := uint32(0); i < n; i++ {
			s, e := m.GetDeclarationSort(q, i)
			coneMust(t, e)
			if integer == (ast.ASTHandle{}) {
				integer = s
			}
			if integer != s {
				t.Fatal("mixed witness sorts")
			}
		}
		body, e := m.GetQuantifierBody(q)
		coneMust(t, e)
		if n == 3 {
			op, e := m.GetDeclKind(body)
			coneMust(t, e)
			if op != ast.ASTOpImplies {
				t.Fatal("missing head definition")
			}
			guard, e := m.GetArg(body, 0)
			coneMust(t, e)
			op, e = m.GetDeclKind(guard)
			coneMust(t, e)
			if op != ast.ASTOpEq {
				t.Fatal("missing defining equality")
			}
			lhs, e := m.GetArg(guard, 0)
			coneMust(t, e)
			index, e := m.GetVarIndex(lhs)
			coneMust(t, e)
			if index != 2 {
				t.Fatal("unexpected eliminated binder", index)
			}
			rhs, e := m.GetArg(guard, 1)
			coneMust(t, e)
			// Validate the witness independently of converter substitution metadata.
			v1, e := m.MkVar(1, integer)
			coneMust(t, e)
			one, e := m.ArithmeticNumeralRat(big.NewRat(1, 1), true)
			coneMust(t, e)
			witness, e := m.MkArithmeticAdd(v1, one)
			coneMust(t, e)
			pin(witness)
			if rhs != witness {
				t.Fatal("unexpected compound witness")
			}
			head, e := m.GetArg(body, 1)
			coneMust(t, e)
			v0, e := m.MkVar(0, integer)
			coneMust(t, e)
			body = apply(head, []ast.ASTHandle{v0, v1, witness})
		}
		return body
	}
	kind, err := m.GetKind(condition)
	coneMust(t, err)
	var body ast.ASTHandle
	if kind == ast.ASTQuantifier {
		body = normalize(condition)
	} else {
		op, e := m.GetDeclKind(condition)
		coneMust(t, e)
		if op != ast.ASTOpEq && op != ast.ASTOpIff {
			t.Fatal("unexpected proof condition")
		}
		a, e := m.GetArg(condition, 0)
		coneMust(t, e)
		b, e := m.GetArg(condition, 1)
		coneMust(t, e)
		a = normalize(a)
		b = normalize(b)
		body, e = m.MkEq(a, b)
		coneMust(t, e)
		pin(body)
	}
	// All remaining variables must belong to the two independent binders.
	todo := []ast.ASTHandle{body}
	for len(todo) > 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		k, e := m.GetKind(h)
		coneMust(t, e)
		switch k {
		case ast.ASTVar:
			i, e := m.GetVarIndex(h)
			coneMust(t, e)
			if i >= 2 {
				t.Fatal("eliminated binder escaped", i)
			}
		case ast.ASTApp:
			n, e := m.GetNumArgs(h)
			coneMust(t, e)
			for i := uint32(0); i < n; i++ {
				a, e := m.GetArg(h, i)
				coneMust(t, e)
				todo = append(todo, a)
			}
		default:
			t.Fatal("unexpected expression in witness check", k)
		}
	}
	args := make([]ast.ASTHandle, 2)
	for i := range args {
		h, e := m.MkFreshConst("slice-independent", integer)
		coneMust(t, e)
		args[i] = pin(h)
	}
	grounded := apply(body, args)
	result, err := m.NewRef(grounded)
	coneMust(t, err)
	return result
}
