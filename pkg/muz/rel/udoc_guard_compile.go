package rel

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

// CompileGuard translates the retained UDoc guard language into a DOC union.
func (r *UDocRelation) CompileGuard(manager *ast.ASTManager, expression ast.ASTHandle) (*UDocRelation, error) {
	return r.compileGuard(manager, expression, make([]bool, r.NumBits()))
}

func (r *UDocRelation) compileGuard(manager *ast.ASTManager, expression ast.ASTHandle, discarded []bool) (*UDocRelation, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if manager == nil || expression == (ast.ASTHandle{}) || len(discarded) != int(r.NumBits()) {
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
	classes := backtrack.NewDefaultUnionFind()
	for i := uint32(0); i < r.NumBits(); i++ {
		classes.MkVar()
	}
	if err == nil {
		err = compiled.applyGuard(manager, expression, classes, discarded)
	}
	if err != nil {
		_ = compiled.Close()
		return nil, err
	}
	return compiled, nil
}

// FilterGuardAndProject applies the DOC guard before projecting the requested
// columns, preserving the source's discarded-column equality merge behavior.
func (r *UDocRelation) FilterGuardAndProject(manager *ast.ASTManager, expression ast.ASTHandle, removed []uint32, destination *DocManager) (result *UDocRelation, resultErr error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if destination == nil || destination.closed {
		return nil, ErrDOC
	}
	discarded := make([]bool, r.NumBits())
	seen := make([]bool, len(r.widths))
	for _, column := range removed {
		if column >= uint32(len(r.widths)) || seen[column] {
			return nil, ErrDOC
		}
		seen[column] = true
		for bit := r.offsets[column]; bit < r.offsets[column+1]; bit++ {
			discarded[bit] = true
		}
	}
	compiled, err := r.compileGuard(manager, expression, discarded)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, compiled.Close()) }()
	filtered, err := r.Clone()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, filtered.Close()) }()
	if err = filtered.elems.Intersect(compiled.elems); err != nil {
		return nil, err
	}
	return filtered.Project(removed, destination)
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

func (r *UDocRelation) applyGuard(m *ast.ASTManager, expression ast.ASTHandle, classes *backtrack.UnionFind, discarded []bool) error {
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
			if e = r.applyGuard(m, arg, classes, discarded); e != nil {
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
		if e = temporary.applyGuard(m, arg, classes, make([]bool, len(discarded))); e == nil {
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
			e = temporary.applyGuard(m, negated, classes, discarded)
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
		return r.applyBitVectorEquality(m, left, right, classes, discarded)
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

func (r *UDocRelation) applyBitVectorEquality(m *ast.ASTManager, left, right ast.ASTHandle, classes *backtrack.UnionFind, discarded []bool) error {
	leftVar, leftLow, leftHigh, leftOK, err := udocGuardRange(m, left)
	if err != nil {
		return err
	}
	rightVar, rightLow, rightHigh, rightOK, err := udocGuardRange(m, right)
	if err != nil {
		return err
	}
	if leftOK && rightOK {
		length := leftHigh - leftLow + 1
		if rightHigh < rightLow || rightHigh-rightLow+1 != length || leftVar >= uint32(len(r.widths)) || rightVar >= uint32(len(r.widths)) {
			return ErrDOC
		}
		leftIndex := r.offsets[leftVar] + leftLow
		rightIndex := r.offsets[rightVar] + rightLow
		if leftIndex+length > r.NumBits() || rightIndex+length > r.NumBits() {
			return ErrDOC
		}
		for i := uint32(0); i < length; i++ {
			classes.Merge(leftIndex+i, rightIndex+i)
		}
		for i := 0; i < len(r.elems.values); {
			valid, e := r.manager.MergeRange(r.elems.values[i], leftIndex, length, classes, discarded)
			if e != nil {
				return e
			}
			if !valid {
				if e = r.elems.erase(i); e != nil {
					return e
				}
				continue
			}
			i++
		}
		return nil
	}
	ground := right
	if !leftOK {
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
