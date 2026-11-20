package order

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *inference) read(h ast.ASTHandle, scope []binding, depth int) (int, error) {
	if err := t.visit(depth); err != nil {
		return 0, err
	}
	hd, err := t.m.NodeHeader(h)
	if err != nil {
		return 0, err
	}
	n := expression{domain: boolean}
	switch hd.Kind {
	case ast.ASTVar:
		index, err := t.m.GetVarIndex(h)
		if err != nil {
			return 0, err
		}
		if uint64(index) >= uint64(len(scope)) {
			return 0, inapplicable
		}
		v := scope[len(scope)-1-int(index)]
		n.op, n.value, n.domain = "var", v.name, v.domain
		// A source factory can construct an open or ill-sorted term.
		s, err := t.m.GetSort(h)
		if err != nil {
			return 0, err
		}
		expected := t.boolSort
		if v.domain != boolean {
			expected = t.domainSorts[v.domain]
		}
		if s != expected {
			return 0, inapplicable
		}
	case ast.ASTQuantifier:
		if err = t.quantifier(h, scope, depth, &n); err != nil {
			return 0, err
		}
	case ast.ASTApp:
		if hd.NumArgs > uint32(maxWork-t.visits) {
			return 0, inapplicable
		}
		n.args = make([]int, hd.NumArgs)
		for i := range n.args {
			a, err := t.m.GetArg(h, uint32(i))
			if err != nil {
				return 0, err
			}
			n.args[i], err = t.read(a, scope, depth+1)
			if err != nil {
				return 0, err
			}
		}
		if err = t.application(h, hd, &n); err != nil {
			return 0, err
		}
	default:
		return 0, inapplicable
	}
	id := len(t.terms)
	t.terms = append(t.terms, n)
	return id, nil
}

func (t *inference) quantifier(h ast.ASTHandle, scope []binding, depth int, n *expression) error {
	count, err := t.m.GetNumDeclarations(h)
	if err != nil {
		return err
	}
	if count == 0 || count > uint32(maxWork-t.visits) {
		return inapplicable
	}
	t.visits += int(count)
	n.binders = make([]binding, count)
	for i := range n.binders {
		s, err := t.m.GetDeclarationSort(h, uint32(i))
		if err != nil {
			return err
		}
		d, err := t.domain(s)
		if err != nil {
			return err
		}
		n.binders[i] = binding{name: t.nextBinder, domain: d}
		t.nextBinder++
	}
	b, err := t.m.GetQuantifierBody(h)
	if err != nil {
		return err
	}
	body, err := t.read(b, append(scope, n.binders...), depth+1)
	if err != nil {
		return err
	}
	if t.terms[body].domain != boolean {
		return inapplicable
	}
	forall, err := t.m.IsForall(h)
	if err != nil {
		return err
	}
	n.op = "exists"
	if forall {
		n.op = "forall"
	}
	n.args = []int{body}
	t.quantified = true
	return nil
}

// Values from the bit-vector declaration plugin (bv_decl_plugin.h).
const (
	bvNumeral = 0
	bvULE     = 22
	bvUGE     = 24
	bvULT     = 26
	bvUGT     = 28
)

func (t *inference) application(h ast.ASTHandle, hd ast.ASTNodeHeader, n *expression) error {
	switch hd.Family {
	case ast.NullFamilyID:
		id, err := t.declaration(hd.Decl)
		if err != nil {
			return err
		}
		d := &t.signatures[id]
		if len(n.args) != len(d.inputs) {
			return inapplicable
		}
		for i, a := range n.args {
			if err = t.unify(d.inputs[i], t.terms[a].domain); err != nil {
				return err
			}
		}
		n.op, n.value, n.domain = "fun", id, d.result
		return nil
	case t.m.BasicFamilyID():
		return t.booleanApp(hd.DeclKind, n)
	}
	if hd.DeclKind == bvNumeral && len(n.args) == 0 {
		s, err := t.m.GetSort(h)
		if err != nil {
			return err
		}
		n.domain, err = t.domain(s)
		if err != nil {
			return err
		}
		if n.domain == boolean || hd.Family != t.bvFamily {
			return inapplicable
		}
		v, ok := t.numerals[h]
		if !ok {
			value, matched, err := t.m.BitVectorNumeralText(h)
			if err != nil {
				return err
			}
			if !matched {
				return inapplicable
			}
			u, err := strconv.ParseUint(value, 10, int(t.widths[s]))
			if err != nil {
				return inapplicable
			}
			v = int(u)
			t.numerals[h] = v
		}
		n.op, n.value = "num", v
		return nil
	}
	if hd.Family != t.bvFamily || len(n.args) != 2 {
		return inapplicable
	}
	switch hd.DeclKind {
	case bvULE, bvUGE, bvULT, bvUGT:
		a, b := t.terms[n.args[0]].domain, t.terms[n.args[1]].domain
		if a == boolean {
			return inapplicable
		}
		n.op, n.value = "order", int(hd.DeclKind)
		return t.unify(a, b)
	}
	return inapplicable
}

func (t *inference) booleanApp(op int32, n *expression) error {
	size := len(n.args)
	switch op {
	case ast.ASTOpTrue, ast.ASTOpFalse:
		if size != 0 {
			return inapplicable
		}
		n.op = "true"
		if op == ast.ASTOpFalse {
			n.op = "false"
		}
	case ast.ASTOpEq, ast.ASTOpIff, ast.ASTOpDistinct:
		if size < 2 {
			return inapplicable
		}
		n.op = "="
		if op == ast.ASTOpDistinct {
			n.op = "distinct"
		}
		d := t.terms[n.args[0]].domain
		for _, a := range n.args[1:] {
			if err := t.unify(d, t.terms[a].domain); err != nil {
				return err
			}
		}
	case ast.ASTOpIte:
		if size != 3 || t.terms[n.args[0]].domain != boolean {
			return inapplicable
		}
		n.op = "ite"
		n.domain = t.terms[n.args[1]].domain
		return t.unify(n.domain, t.terms[n.args[2]].domain)
	default:
		switch op {
		case ast.ASTOpAnd:
			n.op = "and"
		case ast.ASTOpOr:
			n.op = "or"
		case ast.ASTOpImplies:
			n.op = "=>"
		case ast.ASTOpXor:
			n.op = "xor"
		case ast.ASTOpNot:
			n.op = "not"
		default:
			return inapplicable
		}
		if (op == ast.ASTOpNot && size != 1) || (op == ast.ASTOpImplies || op == ast.ASTOpXor) && size != 2 {
			return inapplicable
		}
		for _, a := range n.args {
			if t.terms[a].domain != boolean {
				return inapplicable
			}
		}
		if size == 0 {
			if op == ast.ASTOpAnd {
				n.op = "true"
			} else {
				n.op = "false"
			}
		}
	}
	return nil
}
