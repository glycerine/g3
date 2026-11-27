package api

import "github.com/glycerine/g3/pkg/ast"

// Each bit represents one interpretation. Every uninterpreted predicate has
// a constant truth value and each uninterpreted sort has a single element.
// Known truth values hold for every assignment of bound variables.
type premiseTruth struct{ yes, no uint64 }

func (a premiseTruth) negate() premiseTruth { return premiseTruth{a.no, a.yes} }

func (a premiseTruth) and(b premiseTruth) premiseTruth {
	return premiseTruth{a.yes & b.yes, a.no | b.no}
}

func (a premiseTruth) or(b premiseTruth) premiseTruth {
	return premiseTruth{a.yes | b.yes, a.no & b.no}
}

type premiseModel struct {
	manager       *ast.ASTManager
	values        map[ast.ASTHandle]premiseTruth
	predicates    map[ast.ASTHandle]uint8
	block         int
	visits        int
	collapsedSort bool
}

// A model of the selected premises lets the refutation strategy decline
// this plan. The caller still checks the original assertions.
func hasPrivatePremiseModel(m *ast.ASTManager, roots []ast.ASTHandle) bool {
	if len(roots) == 0 || len(roots) > maxNarrowPremises {
		return false
	}
	v := premiseModel{manager: m, values: make(map[ast.ASTHandle]premiseTruth), predicates: make(map[ast.ASTHandle]uint8)}
	// Six predicates vary across the bits of a word. Each subsequent block
	// varies the assignment to the next six, keeping declarations consistent
	// across every root. Charge AST visits across blocks to bound failed plans.
	for ; ; v.block++ {
		possible := ^uint64(0)
		for i := len(roots) - 1; i >= 0 && possible != 0; i-- {
			if m.ResourceLimit().CancelFlag() {
				return false
			}
			possible &= v.eval(roots[i], 0).yes
		}
		if possible != 0 && (!v.collapsedSort || purePremiseTerms(m, roots)) {
			return true
		}
		n := len(v.predicates)
		if n <= 6 || v.block+1 >= 1<<(n-6) || v.visits >= 512 {
			return false
		}
		clear(v.values)
		v.collapsedSort = false
	}
}

// Interpreted functions such as model-value constants can distinguish
// elements of a user sort. Only collapse domains in the pure UF fragment.
func purePremiseTerms(m *ast.ASTManager, roots []ast.ASTHandle) bool {
	stack := append([]ast.ASTHandle(nil), roots...)
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		i := len(stack) - 1
		h := stack[i]
		stack = stack[:i]
		if seen[h] {
			continue
		}
		if len(seen) >= 512 {
			return false
		}
		seen[h] = true
		header, err := m.NodeHeader(h)
		if err != nil {
			return false
		}
		switch header.Kind {
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return false
			}
			stack = append(stack, body)
		case ast.ASTVar:
		case ast.ASTApp:
			if header.Family != ast.NullFamilyID && header.Family != m.BasicFamilyID() {
				return false
			}
			if int(header.NumArgs) > 512-len(stack) {
				return false
			}
			for j := uint32(0); j < header.NumArgs; j++ {
				child, err := m.GetArg(h, j)
				if err != nil {
					return false
				}
				stack = append(stack, child)
			}
		default:
			return false
		}
	}
	return true
}

func (v *premiseModel) eval(term ast.ASTHandle, depth int) premiseTruth {
	if result, ok := v.values[term]; ok {
		return result
	}
	if depth > 64 || v.visits >= 512 {
		return premiseTruth{}
	}
	v.visits++
	result := v.evaluateNode(term, depth)
	v.values[term] = result
	return result
}

func (v *premiseModel) predicate(decl ast.ASTHandle) premiseTruth {
	// Column i contains bit i of the interpretation index. Additional
	// predicates can remain unknown if the other operands decide a formula.
	columns := [...]uint64{0xaaaaaaaaaaaaaaaa, 0xcccccccccccccccc, 0xf0f0f0f0f0f0f0f0, 0xff00ff00ff00ff00, 0xffff0000ffff0000, 0xffffffff00000000}
	index, ok := v.predicates[decl]
	if !ok {
		if len(v.predicates) == 12 {
			return premiseTruth{}
		}
		index = uint8(len(v.predicates))
		v.predicates[decl] = index
	}
	var mask uint64
	if index < uint8(len(columns)) {
		mask = columns[index]
	} else if v.block&(1<<(index-uint8(len(columns)))) != 0 {
		mask = ^uint64(0)
	}
	return premiseTruth{mask, ^mask}
}

func (v *premiseModel) equality(term ast.ASTHandle, depth int) premiseTruth {
	m := v.manager
	left, err := m.GetArg(term, 0)
	if err != nil {
		return premiseTruth{}
	}
	right, err := m.GetArg(term, 1)
	if err != nil {
		return premiseTruth{}
	}
	if left == right {
		return premiseTruth{yes: ^uint64(0)}
	}
	sort, err := m.GetSort(left)
	if err != nil {
		return premiseTruth{}
	}
	singleton, err := m.IsUninterpretedSort(sort)
	if err != nil {
		return premiseTruth{}
	}
	if singleton {
		size, err := m.GetSortSize(sort)
		if err != nil || (size != ast.InfiniteSortSize() && size != ast.FiniteSortSize(1)) {
			return premiseTruth{}
		}
		v.collapsedSort = true
		return premiseTruth{yes: ^uint64(0)}
	}
	if boolean, err := m.HasBoolSort(left); err != nil || !boolean {
		return premiseTruth{}
	}
	a, b := v.eval(left, depth+1), v.eval(right, depth+1)
	return a.and(b).or(a.negate().and(b.negate()))
}

func (v *premiseModel) evaluateNode(term ast.ASTHandle, depth int) premiseTruth {
	const all = ^uint64(0)
	m := v.manager
	header, err := m.NodeHeader(term)
	if err != nil {
		return premiseTruth{}
	}
	if header.Kind == ast.ASTQuantifier {
		body, err := m.GetQuantifierBody(term)
		if err != nil {
			return premiseTruth{}
		}
		// Domains are nonempty. A uniformly true or false body gives the
		// same value under either a universal or an existential binder.
		return v.eval(body, depth+1)
	}
	if header.Kind != ast.ASTApp {
		return premiseTruth{}
	}
	if header.Family == ast.NullFamilyID {
		if boolean, err := m.HasBoolSort(term); err == nil && boolean {
			return v.predicate(header.Decl)
		}
		return premiseTruth{}
	}
	if header.Family != m.BasicFamilyID() {
		return premiseTruth{}
	}
	arg := func(i uint32) premiseTruth {
		child, err := m.GetArg(term, i)
		if err != nil {
			return premiseTruth{}
		}
		return v.eval(child, depth+1)
	}
	switch header.DeclKind {
	case ast.ASTOpTrue:
		return premiseTruth{yes: all}
	case ast.ASTOpFalse:
		return premiseTruth{no: all}
	case ast.ASTOpNot:
		return arg(0).negate()
	case ast.ASTOpImplies:
		return arg(0).negate().or(arg(1))
	case ast.ASTOpEq, ast.ASTOpIff:
		return v.equality(term, depth)
	case ast.ASTOpIte:
		c, t, f := arg(0), arg(1), arg(2)
		return premiseTruth{c.yes&t.yes | c.no&f.yes | t.yes&f.yes, c.yes&t.no | c.no&f.no | t.no&f.no}
	case ast.ASTOpAnd, ast.ASTOpOr:
		if header.NumArgs > 256 {
			return premiseTruth{}
		}
		if header.DeclKind == ast.ASTOpAnd {
			result := premiseTruth{yes: all}
			for i := uint32(0); i < header.NumArgs && result.no != all; i++ {
				result = result.and(arg(i))
			}
			return result
		}
		result := premiseTruth{no: all}
		for i := uint32(0); i < header.NumArgs && result.yes != all; i++ {
			result = result.or(arg(i))
		}
		return result
	}
	return premiseTruth{}
}
