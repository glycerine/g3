package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogCombinesInterpretedTailsWithoutStrengthening(t *testing.T) {
	for _, kinds := range [][]string{
		{"bound_relation", "interval_relation"},
		{"interval_relation", "bound_relation"},
	} {
		t.Run(kinds[0], func(t *testing.T) {
			f := newDirectFactFixture(t)
			m, e := f.manager, f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			var predicates []boundIntervalDatalogPredicate
			declare := func(name string, arity int) ast.ASTHandle {
				symbol, cause := m.Symbols.Intern("bound-interval-conjunction-" + name)
				must(cause)
				sorts := make([]ast.ASTHandle, arity)
				for i := range sorts {
					sorts[i] = integer
				}
				declaration, cause := m.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
				must(cause)
				must(e.RegisterPredicate(declaration))
				predicates = append(predicates, boundIntervalDatalogPredicate{declaration: declaration, kinds: kinds})
				return declaration
			}
			p, r, repeatedSource, missing := declare("P", 2), declare("R", 2), declare("RepeatedSource", 2), declare("Missing", 2)
			direct, projected, joined := declare("Direct", 2), declare("Projected", 1), declare("Joined", 2)
			joinedProjected := declare("JoinedProjected", 1)
			unbound, unboundProjected := declare("Unbound", 2), declare("UnboundProjected", 1)
			repeated, single, empty := declare("Repeated", 1), declare("Single", 1), declare("Empty", 1)
			activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, predicates)
			must(err)
			if !selected {
				t.Fatal("bound/interval product was not selected")
			}
			defer func() { must(activation.close()) }()
			var roots []ast.ASTRef
			defer func() {
				for i := len(roots) - 1; i >= 0; i-- {
					must(m.DelRef(&roots[i]))
				}
			}()
			hold := func(value ast.ASTHandle) ast.ASTHandle {
				root, cause := m.NewRef(value)
				must(cause)
				roots = append(roots, root)
				return value
			}
			numeral := func(value int64) ast.ASTHandle {
				result, cause := m.ArithmeticIntegerNumeralText(big.NewInt(value).String())
				must(cause)
				return hold(result)
			}
			addFact := func(declaration ast.ASTHandle, values ...int64) {
				arguments := make([]ast.ASTHandle, len(values))
				for i, value := range values {
					arguments[i] = numeral(value)
				}
				must(activation.addFact(declaration, arguments))
			}
			addFact(p, 2, 10)
			addFact(p, 8, 20)
			addFact(r, 10, 30)
			addFact(r, 20, 40)
			addFact(repeatedSource, 2, 4)
			addFact(repeatedSource, 8, 6)
			variables := make([]ast.ASTHandle, 4)
			for i := range variables {
				variables[i], err = m.MkVar(uint32(i), integer)
				must(err)
				hold(variables[i])
			}
			x, y, z, w := variables[0], variables[1], variables[2], variables[3]
			app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
				result, cause := m.MkApp(declaration, arguments)
				must(cause)
				return hold(result)
			}
			le := func(left, right ast.ASTHandle) ast.ASTHandle {
				result, cause := m.MkArithmeticLE(left, right)
				must(cause)
				return hold(result)
			}
			addRule := func(head ast.ASTHandle, positive []ast.ASTHandle, conditions ...ast.ASTHandle) {
				t.Helper()
				tails := append(append([]ast.ASTHandle(nil), positive...), conditions...)
				rule, cause := e.rules.NewRule(head, tails, nil, "bound-interval-interpreted-conjunction")
				must(cause)
				defer func() { must(rule.Close()) }()
				total, cause := rule.TailSize()
				must(cause)
				uninterpreted, cause := rule.UninterpretedTailSize()
				must(cause)
				if total-uninterpreted != len(conditions) {
					t.Fatal("fixture did not preserve separate interpreted tails", total, uninterpreted, len(conditions))
				}
				must(e.set.Add(rule))
			}
			pTail := app(p, x, y)
			upperX, upperY := le(x, numeral(5)), le(y, numeral(15))
			addRule(app(direct, x, y), []ast.ASTHandle{pTail}, upperX, upperY)
			addRule(app(projected, x), []ast.ASTHandle{pTail}, upperX, upperY)
			addRule(app(joined, z, x), []ast.ASTHandle{pTail, app(r, y, z)}, upperX, le(z, numeral(35)))
			addRule(app(joinedProjected, x), []ast.ASTHandle{pTail, app(r, y, z)}, upperY, le(z, numeral(35)))
			belowX, err := m.MkArithmeticLT(w, x)
			must(err)
			hold(belowX)
			addRule(app(unbound, w, x), []ast.ASTHandle{pTail}, belowX, le(w, numeral(0)))
			addRule(app(unboundProjected, x), []ast.ASTHandle{pTail}, belowX, le(w, numeral(0)))
			addRule(app(repeated, x), []ast.ASTHandle{app(repeatedSource, x, x)}, upperX, le(numeral(5), x))
			addRule(app(single, x), []ast.ASTHandle{pTail}, upperX)
			addRule(app(empty, x), []ast.ASTHandle{app(missing, x, y)}, upperX, upperY)
			contains := func(declaration ast.ASTHandle, values ...int64) bool {
				arguments := make([]*big.Rat, len(values))
				for i, value := range values {
					arguments[i] = big.NewRat(value, 1)
				}
				return activation.evaluated[declaration].ContainsFact(arguments)
			}
			check := func(maxX, maxY, maxZ int64) {
				t.Helper()
				status, cause := activation.query([]ast.ASTHandle{direct, joined, unbound, repeated})
				must(cause)
				if status != lbool.Undef {
					t.Fatal("nonempty coarse product status", status)
				}
				// The source supplies one And expression. Neither component
				// recognizes it, so separate guards must not tighten the hull.
				if !contains(direct, 2, 10) || !contains(direct, maxX, maxY) || contains(direct, 1, 10) || contains(direct, 2, maxY+1) {
					t.Fatal("separate interpreted tails changed the source conjunction hull")
				}
				if !contains(projected, maxX) || !contains(unboundProjected, maxX) || !contains(joinedProjected, maxX) || contains(projected, maxX+1) || contains(joinedProjected, maxX+1) {
					t.Fatal("conjunction/project lost source columns")
				}
				if !contains(joined, 30, 2) || !contains(joined, maxZ, maxX) || contains(joined, maxZ+1, 2) {
					t.Fatal("two-tail conjunction lost joined column mapping")
				}
				if !contains(unbound, -100, 2) || !contains(unbound, 100, maxX) || contains(unbound, 0, maxX+1) {
					t.Fatal("conjunction condition-only column was omitted or strengthened")
				}
				if !contains(repeated, 4) || !contains(repeated, 6) || contains(repeated, 3) || contains(repeated, 7) {
					t.Fatal("conjunction changed repeated-tail equality intersection")
				}
				if !contains(single, 2) || !contains(single, 5) || contains(single, 6) {
					t.Fatal("singleton interpreted tail lost its source filter")
				}
				if !activation.evaluated[empty].Empty() || !contains(p, maxX, maxY) || !contains(r, maxY, maxZ) {
					t.Fatal("conjunction changed emptiness or source ownership")
				}
			}
			check(8, 20, 40)
			check(8, 20, 40)
			answer, err := activation.relationFormula(unbound)
			must(err)
			defer func() { must(m.DelRef(&answer)) }()
			m.Limit.Cancel()
			_, cause := activation.query([]ast.ASTHandle{direct})
			m.Limit.ResetCancel()
			if cause == nil {
				t.Fatal("canceled conjunction query succeeded")
			}
			if _, cause = activation.relationFormula(direct); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
				t.Fatal("canceled conjunction query exposed a stale result", cause)
			}
			if _, cause = m.RefGet(answer); cause != nil {
				t.Fatal("cancellation invalidated a caller-owned formula", cause)
			}
			addFact(p, 10, 25)
			addFact(r, 25, 50)
			check(10, 25, 50)
		})
	}
}
