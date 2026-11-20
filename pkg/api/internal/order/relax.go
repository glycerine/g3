// Package order relaxes quantified bit-vector constraints to many-sorted
// first-order logic with equality and unsigned strict total orders.
package order

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// A Boolean expression needs no abstract domain. Each bit-vector function
// position, binder and numeral occurrence starts in its own domain. Only
// positions that exchange values are unified.
const boolean = -1

const maxWork = 100000

type binding struct {
	name, domain int
}

type expression struct {
	op            string
	domain, value int
	args          []int
	binders       []binding
}

type signature struct {
	inputs []int
	result int
}

type inference struct {
	m                  *ast.ASTManager
	boolSort           ast.ASTHandle
	bvFamily           ast.FamilyID
	widths             map[ast.ASTHandle]uint32
	parents            []int
	domainSorts        []ast.ASTHandle
	declIDs            map[ast.ASTHandle]int
	signatures         []signature
	numerals           map[ast.ASTHandle]int
	containsFunction   map[ast.ASTHandle]bool
	terms              []expression
	assertions         []int
	nextBinder, visits int
	quantified         bool
}

var inapplicable = errors.New("outside the order abstraction")

// Relax returns an independent SMT-LIB problem. Every model of the source
// assertions lifts to a model of this problem. Give each inferred sort a copy
// of the unsigned integers of its original width and transport each function
// across the bijections for its argument and result positions. All equalities,
// unsigned comparisons, conditionals and quantifiers then retain their truth
// values. Only unsatisfiability of the returned problem establishes a result
// for the source assertions. A false applicability result requests fallback.
func Relax(m *ast.ASTManager, assertions []ast.ASTHandle) (Problem, bool, error) {
	var t inference
	if ok, err := t.infer(m, assertions); !ok || err != nil {
		return Problem{}, false, err
	}
	return t.render(), true, nil
}

func (t *inference) infer(m *ast.ASTManager, assertions []ast.ASTHandle) (bool, error) {
	// This is a cached header bit even when a ground assertion is a large DAG.
	quantified := false
	for _, h := range assertions {
		has, err := m.HasQuantifiers(h)
		if err != nil {
			return false, err
		}
		if has {
			quantified = true
			break
		}
	}
	if !quantified {
		return false, nil
	}
	b, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	*t = inference{
		m: m, boolSort: b, bvFamily: ast.NullFamilyID,
		widths: make(map[ast.ASTHandle]uint32), declIDs: make(map[ast.ASTHandle]int),
		numerals: make(map[ast.ASTHandle]int), containsFunction: make(map[ast.ASTHandle]bool),
	}
	for _, h := range assertions {
		if err := t.conjunct(h, 0); err != nil {
			if err == inapplicable {
				return false, nil
			}
			return false, err
		}
	}
	if !t.quantified || len(t.parents) == 0 || len(t.assertions) == 0 {
		return false, nil
	}
	return true, nil
}

// Bound the expanded syntax tree as a shared source DAG can have exponentially
// many occurrences. Different occurrences can need different abstract sorts.
func (t *inference) visit(depth int) error {
	t.visits++
	if depth > 256 || t.visits > maxWork || t.visits%256 == 0 && t.m.ResourceLimit().CancelFlag() {
		return inapplicable
	}
	return nil
}

func (t *inference) domain(sort ast.ASTHandle) (int, error) {
	if sort == t.boolSort {
		return boolean, nil
	}
	if _, ok := t.widths[sort]; !ok {
		bv, err := t.m.IsBitVectorSort(sort)
		if err != nil {
			return 0, err
		}
		if !bv {
			return 0, inapplicable
		}
		width, err := t.m.BitVectorSize(sort)
		if err != nil {
			return 0, err
		}
		if width == 0 || width > 16 {
			return 0, inapplicable
		}
		t.widths[sort] = width
		t.bvFamily, err = t.m.GetFamilyID(sort)
		if err != nil {
			return 0, err
		}
	}
	id := len(t.parents)
	t.parents = append(t.parents, id)
	t.domainSorts = append(t.domainSorts, sort)
	return id, nil
}

func (t *inference) root(i int) int {
	for i != boolean && t.parents[i] != i {
		t.parents[i] = t.parents[t.parents[i]]
		i = t.parents[i]
	}
	return i
}

func (t *inference) unify(a, b int) error {
	a, b = t.root(a), t.root(b)
	if a == b {
		return nil
	}
	if a == boolean || b == boolean || t.domainSorts[a] != t.domainSorts[b] {
		return inapplicable
	}
	if a > b {
		a, b = b, a
	}
	t.parents[b] = a
	return nil
}

func (t *inference) declaration(h ast.ASTHandle) (int, error) {
	if id, ok := t.declIDs[h]; ok {
		return id, nil
	}
	n, err := t.m.GetArity(h)
	if err != nil {
		return 0, err
	}
	if n > uint32(maxWork-t.visits) {
		return 0, inapplicable
	}
	t.visits += int(n)
	d := signature{inputs: make([]int, n)}
	for i := range d.inputs {
		s, err := t.m.GetDomain(h, uint32(i))
		if err != nil {
			return 0, err
		}
		d.inputs[i], err = t.domain(s)
		if err != nil {
			return 0, err
		}
	}
	s, err := t.m.GetRange(h)
	if err != nil {
		return 0, err
	}
	d.result, err = t.domain(s)
	if err != nil {
		return 0, err
	}
	id := len(t.signatures)
	t.declIDs[h] = id
	t.signatures = append(t.signatures, d)
	return id, nil
}

// Theory-only top-level conjuncts may be dropped as whole formulas. In
// particular, expanding bounds on a finite bit-vector sort can otherwise
// force the abstract solver to enumerate that same finite domain.
func (t *inference) conjunct(h ast.ASTHandle, depth int) error {
	if err := t.visit(depth); err != nil {
		return err
	}
	hd, err := t.m.NodeHeader(h)
	if err != nil {
		return err
	}
	if hd.Kind == ast.ASTApp && hd.Family == t.m.BasicFamilyID() && hd.DeclKind == ast.ASTOpAnd {
		for i := uint32(0); i < hd.NumArgs; i++ {
			a, err := t.m.GetArg(h, i)
			if err != nil {
				return err
			}
			if err = t.conjunct(a, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	keep, err := t.hasFunction(h, depth)
	if err != nil || !keep {
		return err
	}
	id, err := t.read(h, nil, depth)
	if err != nil {
		return err
	}
	if t.terms[id].domain != boolean {
		return inapplicable
	}
	t.assertions = append(t.assertions, id)
	return nil
}

func (t *inference) hasFunction(h ast.ASTHandle, depth int) (bool, error) {
	if v, ok := t.containsFunction[h]; ok {
		return v, nil
	}
	if err := t.visit(depth); err != nil {
		return false, err
	}
	hd, err := t.m.NodeHeader(h)
	if err != nil {
		return false, err
	}
	var found bool
	switch hd.Kind {
	case ast.ASTQuantifier:
		b, err := t.m.GetQuantifierBody(h)
		if err != nil {
			return false, err
		}
		found, err = t.hasFunction(b, depth+1)
		if err != nil {
			return false, err
		}
	case ast.ASTApp:
		found = hd.Family == ast.NullFamilyID
		for i := uint32(0); !found && i < hd.NumArgs; i++ {
			a, err := t.m.GetArg(h, i)
			if err != nil {
				return false, err
			}
			found, err = t.hasFunction(a, depth+1)
			if err != nil {
				return false, err
			}
		}
	}
	t.containsFunction[h] = found
	return found, nil
}
