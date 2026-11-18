package arith

// Source: src/tactic/arith/eq2bv_tactic.cpp, generated finite-domain bounds.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type eq2bvBound struct {
	formula    ast.ASTRef
	dependency ast.ASTDependency
}
type eq2bvBounds struct {
	manager *ast.ASTManager
	entries []eq2bvBound
}

func (b *eq2bvBounds) Close() error {
	if b == nil || b.manager == nil {
		return nil
	}
	for len(b.entries) != 0 {
		i := len(b.entries) - 1
		entry := &b.entries[i]
		if entry.formula != (ast.ASTRef{}) {
			if err := b.manager.DelRef(&entry.formula); err != nil {
				return err
			}
		}
		if !entry.dependency.IsNull() {
			consumed, err := b.manager.DependencyDecConsumed(entry.dependency)
			if consumed {
				entry.dependency = ast.ASTDependency{}
			}
			if err != nil {
				return err
			}
			if !consumed {
				return ast.ErrASTPending
			}
		}
		b.entries = b.entries[:i]
	}
	b.manager = nil
	return nil
}
func eq2bvNumeral(m *ast.ASTManager, value *unumeral.Rational, width uint32) (h ast.ASTHandle, err error) {
	return m.BitVectorNumeralFromRational(value, width)
}
func eq2bvMakeBounds(plan *eq2bvReplacements, source *BoundManager) (result *eq2bvBounds, err error) {
	if plan == nil || plan.manager == nil || source == nil || source.Manager() != plan.manager {
		return nil, ast.ErrASTHandle
	}
	m := plan.manager
	result = &eq2bvBounds{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	// rational r;
	q := m.NumeralService().Manager
	value, max := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&value), q.Del(&max)) }()
	for _, entry := range plan.entries {
		fresh, e := m.RefGet(entry.fresh)
		if e != nil {
			return result, e
		}
		for _, lower := range []bool{true, false} {
			var strict, found bool
			var dep ast.ASTDependency
			if lower {
				strict, found, e = source.Lower(entry.original, &value)
			} else {
				strict, found, e = source.Upper(entry.original, &value)
			}
			if e != nil {
				return result, e
			}
			negative := false
			if found {
				if negative, e = q.IsNeg(&value); e != nil {
					return result, e
				}
			}
			if !found || negative {
				continue
			}
			if strict {
				return result, ast.ErrASTDomain
			}
			if lower {
				dep, e = source.LowerDependency(entry.original)
			} else {
				dep, e = source.UpperDependency(entry.original)
			}
			if e != nil {
				return result, e
			}
			n, e := eq2bvNumeral(m, &value, entry.width)
			if e != nil {
				return result, e
			}
			left, right := fresh, n
			if lower {
				left, right = n, fresh
			}
			formula, e := m.MkBitVectorULE(left, right)
			if e != nil {
				return result, e
			}
			if !lower && entry.negativeSentinel {
				boundRef, x := m.NewRef(formula)
				if x != nil {
					return result, x
				}
				// 2^width - 1
				x = q.SetInt32(&max, 2)
				if x == nil {
					x = q.Power(&max, entry.width, &max)
				}
				if x == nil {
					x = q.Dec(&max)
				}
				var sentinel ast.ASTHandle
				if x == nil {
					sentinel, x = eq2bvNumeral(m, &max, entry.width)
				}
				if x == nil {
					sentinel, x = m.MkEq(fresh, sentinel)
				}
				if x == nil {
					var sentinelRef ast.ASTRef
					sentinelRef, x = m.NewRef(sentinel)
					if x == nil {
						formula, x = m.MkOr(formula, sentinel)
						x = errors.Join(x, m.DelRef(&sentinelRef))
					}
				}
				x = errors.Join(x, m.DelRef(&boundRef))
				if x != nil {
					return result, x
				}
			}
			ref, e := m.NewRef(formula)
			if e != nil {
				return result, e
			}
			if !dep.IsNull() {
				if e = m.DependencyInc(dep); e != nil {
					return result, errors.Join(e, m.DelRef(&ref))
				}
			}
			result.entries = append(result.entries, eq2bvBound{formula: ref, dependency: dep})
		}
	}
	return result, nil
}

// eq2bvPinBoundDependencies keeps the bound dependencies of the plan's
// variables alive while the goal is rewritten: rewriting replaces the bound
// forms (and drops their dependencies) before the bound formulas are built,
// whereas the source bound_manager holds its own references. The returned
// function releases the pins.
func eq2bvPinBoundDependencies(plan *eq2bvReplacements, source *BoundManager) (release func() error, err error) {
	m := plan.manager
	var pinned []ast.ASTDependency
	release = func() error {
		var e error
		for len(pinned) != 0 {
			i := len(pinned) - 1
			e = errors.Join(e, m.DependencyDec(pinned[i]))
			pinned = pinned[:i]
		}
		return e
	}
	for _, entry := range plan.entries {
		for _, lower := range []bool{true, false} {
			var dep ast.ASTDependency
			var e error
			if lower {
				dep, e = source.LowerDependency(entry.original)
			} else {
				dep, e = source.UpperDependency(entry.original)
			}
			if e == nil && !dep.IsNull() {
				if e = m.DependencyInc(dep); e == nil {
					pinned = append(pinned, dep)
				}
			}
			if e != nil {
				return nil, errors.Join(e, release())
			}
		}
	}
	return release, nil
}
