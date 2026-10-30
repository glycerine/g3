package rel

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// CompileGuard translates the retained UDoc guard language into a DOC union.
func (r *UDocRelation) CompileGuard(manager *ast.ASTManager, expression ast.ASTHandle) (*UDocRelation, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if manager == nil || expression == (ast.ASTHandle{}) {
		return nil, ErrDOC
	}
	compiled, err := NewUDocRelation(r.manager, r.widths)
	if err != nil {
		return nil, err
	}
	initial, err := r.manager.AllocateX()
	if err == nil {
		err = compiled.elems.PushOwned(initial)
	}
	if err == nil {
		err = compiled.applyGuard(manager, expression)
	}
	if err != nil {
		_ = compiled.Close()
		return nil, err
	}
	return compiled, nil
}

// FilterGuard intersects the relation with the DOC union compiled from a guard.
func (r *UDocRelation) FilterGuard(manager *ast.ASTManager, expression ast.ASTHandle) (resultErr error) {
	if err := r.check(); err != nil {
		return err
	}
	compiled, err := r.CompileGuard(manager, expression)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, compiled.Close()) }()
	return r.elems.Intersect(compiled.elems)
}

func (r *UDocRelation) applyGuard(m *ast.ASTManager, expression ast.ASTHandle) error {
	if r.elems.Empty() {
		return nil
	}
	if m.IsTrue(expression) {
		return nil
	}
	if m.IsFalse(expression) {
		return r.elems.Reset()
	}
	kind, err := m.GetKind(expression)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		index, e := m.GetVarIndex(expression)
		if e != nil {
			return e
		}
		if index >= uint32(len(r.widths)) || r.widths[index] != 1 {
			return ErrDOC
		}
		sort, e := m.GetSort(expression)
		if e != nil {
			return e
		}
		boolean, e := m.MkBoolSort()
		if e != nil {
			return e
		}
		if sort != boolean {
			return ErrDOC
		}
		cube, e := r.manager.AllocateX()
		if e != nil {
			return e
		}
		if e = r.manager.Set(cube, r.offsets[index], TBit1); e != nil {
			return errors.Join(e, r.manager.Deallocate(cube))
		}
		e = r.elems.IntersectDoc(cube)
		return errors.Join(e, r.manager.Deallocate(cube))
	}
	if kind != ast.ASTApp {
		return ErrDOC
	}
	family, err := m.GetFamilyID(expression)
	if err != nil {
		return err
	}
	declKind, err := m.GetDeclKind(expression)
	if err != nil {
		return err
	}
	if family != m.BasicFamilyID() {
		return ErrDOC
	}
	switch declKind {
	case ast.ASTOpAnd:
		n, e := m.GetNumArgs(expression)
		if e != nil {
			return e
		}
		for i := uint32(0); i < n && !r.elems.Empty(); i++ {
			arg, e := m.GetArg(expression, i)
			if e != nil {
				return e
			}
			if e = r.applyGuard(m, arg); e != nil {
				return e
			}
		}
		return nil
	case ast.ASTOpNot:
		arg, e := m.GetArg(expression, 0)
		if e != nil {
			return e
		}
		temporary, e := r.guardUniverse()
		if e != nil {
			return e
		}
		if e = temporary.applyGuard(m, arg); e == nil {
			e = r.elems.Subtract(temporary.elems)
		}
		return errors.Join(e, temporary.Close())
	case ast.ASTOpOr:
		temporary, e := r.guardUniverse()
		if e != nil {
			return e
		}
		n, e := m.GetNumArgs(expression)
		for i := uint32(0); e == nil && i < n && !temporary.elems.Empty(); i++ {
			arg, readErr := m.GetArg(expression, i)
			if readErr != nil {
				e = readErr
				break
			}
			negated, makeErr := m.MkNot(arg)
			if makeErr != nil {
				e = makeErr
				break
			}
			e = temporary.applyGuard(m, negated)
		}
		if e == nil {
			e = r.elems.Subtract(temporary.elems)
		}
		return errors.Join(e, temporary.Close())
	case ast.ASTOpEq:
		left, e := m.GetArg(expression, 0)
		if e != nil {
			return e
		}
		right, e := m.GetArg(expression, 1)
		if e != nil {
			return e
		}
		return r.applyGroundBitVectorEquality(m, left, right)
	default:
		return ErrDOC
	}
}

func (r *UDocRelation) guardUniverse() (*UDocRelation, error) {
	temporary, err := NewUDocRelation(r.manager, r.widths)
	if err != nil {
		return nil, err
	}
	document, err := r.manager.AllocateX()
	if err == nil {
		err = temporary.elems.PushOwned(document)
	}
	if err != nil {
		_ = temporary.Close()
		return nil, err
	}
	return temporary, nil
}

func (r *UDocRelation) applyGroundBitVectorEquality(m *ast.ASTManager, left, right ast.ASTHandle) error {
	leftVar, leftLow, leftHigh, leftOK, err := udocGuardRange(m, left)
	if err != nil {
		return err
	}
	ground := right
	if !leftOK {
		rightVar, rightLow, rightHigh, rightOK, e := udocGuardRange(m, right)
		if e != nil {
			return e
		}
		if rightOK {
			leftVar, leftLow, leftHigh, leftOK = rightVar, rightLow, rightHigh, true
			ground = left
		}
	}
	if !leftOK {
		return ErrDOC
	}
	value, width, matched, err := m.BitVectorNumeralBigInt(ground)
	if err != nil {
		return err
	}
	if !matched {
		return ErrDOC
	}
	if leftVar >= uint32(len(r.widths)) || leftHigh < leftLow || leftHigh >= r.widths[leftVar] || width != leftHigh-leftLow+1 || value.Sign() < 0 || value.BitLen() > int(width) {
		return ErrDOC
	}
	cube, err := r.manager.AllocateX()
	if err != nil {
		return err
	}
	if err = r.manager.vectors.SetBigInt(cube.positive, new(big.Int).Set(value), r.offsets[leftVar]+leftHigh, r.offsets[leftVar]+leftLow); err != nil {
		return errors.Join(err, r.manager.Deallocate(cube))
	}
	err = r.elems.IntersectDoc(cube)
	return errors.Join(err, r.manager.Deallocate(cube))
}

func udocGuardRange(m *ast.ASTManager, expression ast.ASTHandle) (variable, low, high uint32, matched bool, err error) {
	kind, err := m.GetKind(expression)
	if err != nil {
		return
	}
	if kind == ast.ASTVar {
		variable, err = m.GetVarIndex(expression)
		if err != nil {
			return
		}
		sort, e := m.GetSort(expression)
		if e != nil {
			err = e
			return
		}
		width, e := m.BitVectorSize(sort)
		if e != nil {
			return 0, 0, 0, false, nil
		}
		return variable, 0, width - 1, true, nil
	}
	var base ast.ASTHandle
	var found bool
	found, err = m.BitVectorExtractValue(expression, &low, &high, &base)
	if err != nil || !found {
		return
	}
	kind, err = m.GetKind(base)
	if err != nil || kind != ast.ASTVar {
		return 0, 0, 0, false, err
	}
	variable, err = m.GetVarIndex(base)
	if err != nil {
		return
	}
	sort, err := m.GetSort(base)
	if err != nil {
		return
	}
	width, e := m.BitVectorSize(sort)
	if e != nil {
		return 0, 0, 0, false, nil
	}
	matched = low <= high && high < width
	return
}
