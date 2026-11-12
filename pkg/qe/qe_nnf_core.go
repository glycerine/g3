package qe

// Source: src/qe/qe.cpp class nnf: an explicit todo stack of (formula,
// polarity) frames, memo tables m_pos/m_neg and the result trail m_trail.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// errQENNFNotApp is nnf::get_nnf returning false: a relevant subterm is not
// an application (a quantifier), and the source abandons the conversion,
// leaving the formula unchanged.
var errQENNFNotApp = errors.New("qe nnf: relevant non-application")

type qeNNFFrame struct {
	expression ast.ASTHandle
	positive   bool
}

// lookup returns the memoized result or pushes the frame to visit.
func (n *QENNFNormalizer) lookup(expression ast.ASTHandle, positive bool) (ast.ASTHandle, bool) {
	if value, found := n.memo[qeNNFKey{expression, positive}]; found {
		return value, true
	}
	n.todo = append(n.todo, qeNNFFrame{expression, positive})
	return ast.ASTHandle{}, false
}

func (n *QENNFNormalizer) insert(expression ast.ASTHandle, positive bool, value ast.ASTHandle) error {
	n.memo[qeNNFKey{expression, positive}] = value
	return n.hold(value)
}

func (n *QENNFNormalizer) pop() { n.todo = n.todo[:len(n.todo)-1] }

// literal is p ? e : mk_not(m, e).
func (n *QENNFNormalizer) literal(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	if positive {
		return expression, nil
	}
	return n.manager.MkNotSimplified(expression)
}

// insertGate inserts the bool_rewriter result of a connective; the local
// expr_ref holding it ends after the trail took its reference.
func (n *QENNFNormalizer) insertGate(expression ast.ASTHandle, positive bool, operation int32, arguments ...ast.ASTHandle) error {
	ref, err := n.gateRef(operation, arguments...)
	if err != nil {
		return err
	}
	value, err := n.manager.RefGet(ref)
	if err == nil {
		err = n.insert(expression, positive, value)
	}
	return errors.Join(err, n.manager.DelRef(&ref))
}

// getNNF is nnf::get_nnf. ok is false when a relevant non-application was
// reached.
func (n *QENNFNormalizer) getNNF(formula ast.ASTHandle, p0 bool) (result ast.ASTHandle, ok bool, err error) {
	m := n.manager
	size := len(n.todo)
	if value, found := n.lookup(formula, p0); found {
		return value, true, nil
	}
	if err = n.hold(formula); err != nil {
		return ast.ASTHandle{}, false, err
	}
	// tmp: the get_nnf local receiving lift_ite's result.
	var tmp ast.ASTRef
	defer func() {
		if tmp != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&tmp))
		}
	}()
	for size < len(n.todo) {
		frame := n.todo[len(n.todo)-1]
		e, p := frame.expression, frame.positive
		relevant, e2 := n.relevance.IsRelevant(e)
		if e2 != nil {
			return ast.ASTHandle{}, false, e2
		}
		if !relevant {
			n.pop()
			value, e2 := n.literal(e, p)
			if e2 == nil {
				e2 = n.insert(e, p, value)
			}
			if e2 != nil {
				return ast.ASTHandle{}, false, e2
			}
			continue
		}
		kind, e2 := m.GetKind(e)
		if e2 != nil {
			return ast.ASTHandle{}, false, e2
		}
		if kind != ast.ASTApp {
			return ast.ASTHandle{}, false, nil
		}
		if _, found := n.memo[qeNNFKey{e, p}]; found {
			n.pop()
			continue
		}
		family, e2 := m.GetFamilyID(e)
		if e2 != nil {
			return ast.ASTHandle{}, false, e2
		}
		operation, e2 := m.GetDeclKind(e)
		if e2 != nil {
			return ast.ASTHandle{}, false, e2
		}
		basic := family == m.BasicFamilyID()
		switch {
		case basic && (operation == ast.ASTOpAnd || operation == ast.ASTOpOr):
			e2 = n.nnfAndOr(operation == ast.ASTOpAnd, e, p)
		case basic && operation == ast.ASTOpNot:
			e2 = n.nnfNot(e, p)
		case basic && operation == ast.ASTOpIte:
			e2 = n.nnfIte(e, p)
		case basic && operation == ast.ASTOpIff:
			e2 = n.nnfIff(e, p)
		case basic && operation == ast.ASTOpEq:
			var boolean bool
			boolean, e2 = n.booleanEquality(e)
			if e2 == nil {
				if boolean {
					e2 = n.nnfIff(e, p)
				} else {
					e2 = n.nnfLeaf(e, p, &tmp)
					if e2 == errQENNFNotApp {
						return ast.ASTHandle{}, false, nil
					}
				}
			}
		case basic && operation == ast.ASTOpXor:
			e2 = n.nnfIff(e, !p)
		case basic && operation == ast.ASTOpImplies:
			e2 = n.nnfImplies(e, p)
		default:
			e2 = n.nnfLeaf(e, p, &tmp)
			if e2 == errQENNFNotApp {
				return ast.ASTHandle{}, false, nil
			}
		}
		if e2 != nil {
			return ast.ASTHandle{}, false, e2
		}
	}
	value, found := n.lookup(formula, p0)
	if !found {
		n.pop()
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	return value, true, nil
}

// nnfLeaf is the m_lift_ite branch and the final literal branch.
func (n *QENNFNormalizer) nnfLeaf(e ast.ASTHandle, p bool, tmp *ast.ASTRef) error {
	m := n.manager
	lifted, found, err := n.liftITE(e)
	if err != nil {
		return err
	}
	if found {
		if *tmp == (ast.ASTRef{}) {
			if *tmp, err = m.NewRef(lifted); err != nil {
				return err
			}
		} else if err = m.SetRef(*tmp, lifted); err != nil {
			return err
		}
		value, ok, err := n.getNNF(lifted, p)
		if err != nil {
			return err
		}
		if !ok {
			return errQENNFNotApp
		}
		if err = m.SetRef(*tmp, value); err != nil {
			return err
		}
		n.pop()
		return n.insert(e, p, value)
	}
	n.pop()
	value, err := n.literal(e, p)
	if err != nil {
		return err
	}
	return n.insert(e, p, value)
}

func (n *QENNFNormalizer) args(e ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := n.manager.GetNumArgs(e)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, count)
	for i := range result {
		if result[i], err = n.manager.GetArg(e, uint32(i)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (n *QENNFNormalizer) nnfAndOr(conjunction bool, e ast.ASTHandle, p bool) error {
	arguments, err := n.args(e)
	if err != nil {
		return err
	}
	values := make([]ast.ASTHandle, 0, len(arguments))
	visited := true
	for _, argument := range arguments {
		if r, found := n.lookup(argument, p); found {
			values = append(values, r)
		} else {
			visited = false
		}
	}
	if !visited {
		return nil
	}
	n.pop()
	operation := int32(ast.ASTOpOr)
	if p == conjunction {
		operation = ast.ASTOpAnd
	}
	return n.insertGate(e, p, operation, values...)
}

func (n *QENNFNormalizer) nnfNot(e ast.ASTHandle, p bool) error {
	argument, err := n.manager.GetArg(e, 0)
	if err != nil {
		return err
	}
	if r, found := n.lookup(argument, !p); found {
		return n.insert(e, p, r)
	}
	return nil
}

func (n *QENNFNormalizer) nnfImplies(e ast.ASTHandle, p bool) error {
	arguments, err := n.args(e)
	if err != nil {
		return err
	}
	r1, found1 := n.lookup(arguments[0], !p)
	r2, found2 := n.lookup(arguments[1], p)
	if !found1 || !found2 {
		return nil
	}
	operation := int32(ast.ASTOpAnd)
	if p {
		operation = ast.ASTOpOr
	}
	return n.insertGate(e, p, operation, r1, r2)
}

// pairGate is the shape of nnf_iff and nnf_ite: an outer connective over
// two inner ones, whose local expr_refs end with the function.
func (n *QENNFNormalizer) pairGate(e ast.ASTHandle, p bool, inner, outer int32, a1, a2, b1, b2 ast.ASTHandle) (err error) {
	m := n.manager
	left, err := n.gateRef(inner, a1, a2)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&left)) }()
	right, err := n.gateRef(inner, b1, b2)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&right)) }()
	l, err := m.RefGet(left)
	if err != nil {
		return err
	}
	r, err := m.RefGet(right)
	if err != nil {
		return err
	}
	return n.insertGate(e, p, outer, l, r)
}

func (n *QENNFNormalizer) nnfIff(e ast.ASTHandle, p bool) error {
	arguments, err := n.args(e)
	if err != nil {
		return err
	}
	r1, f1 := n.lookup(arguments[0], true)
	r2, f2 := n.lookup(arguments[0], false)
	p1, f3 := n.lookup(arguments[1], true)
	p2, f4 := n.lookup(arguments[1], false)
	if !(f1 && f2 && f3 && f4) {
		return nil
	}
	n.pop()
	if p {
		return n.pairGate(e, p, ast.ASTOpAnd, ast.ASTOpOr, r1, p1, r2, p2)
	}
	return n.pairGate(e, p, ast.ASTOpOr, ast.ASTOpAnd, r1, p1, r2, p2)
}

func (n *QENNFNormalizer) nnfIte(e ast.ASTHandle, p bool) error {
	arguments, err := n.args(e)
	if err != nil {
		return err
	}
	r1, f1 := n.lookup(arguments[0], true)
	r2, f2 := n.lookup(arguments[0], false)
	th, f3 := n.lookup(arguments[1], p)
	el, f4 := n.lookup(arguments[2], p)
	if !(f1 && f2 && f3 && f4) {
		return nil
	}
	n.pop()
	return n.pairGate(e, p, ast.ASTOpAnd, ast.ASTOpOr, r1, th, r2, el)
}

func (n *QENNFNormalizer) booleanEquality(expression ast.ASTHandle) (bool, error) {
	argument, err := n.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	sort, err := n.manager.GetSort(argument)
	if err != nil {
		return false, err
	}
	boolean, err := n.manager.MkBoolSort()
	return sort == boolean, err
}
