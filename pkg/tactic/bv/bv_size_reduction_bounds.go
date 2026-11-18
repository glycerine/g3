package bv

// Source: src/tactic/bv/bv_size_reduction_tactic.cpp signed bounds and reduction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type bvSizeSubstitution struct{ value ast.ASTExprSubstitution }
type bvSizeReductionImplementation struct {
	family       ast.FamilyID
	lower, upper map[ast.ASTHandle]*unumeral.Rational // rationals of the manager's NumeralService().Manager
	// m_signed_lowers and m_signed_uppers table orders.
	lowerOrder, upperOrder *ast.ObjMap
	keys                   map[ast.ASTHandle]bool
	held, scratch          []ast.ASTHandle
	replacer               *rewriter.DefaultExprReplacer
	out                    ast.ASTRef
	substitutions          []*bvSizeSubstitution
	active                 *bvSizeSubstitution
	extension              *tactic.ExtensionModelConverter
	filter                 *tactic.FilterModelConverter
	pending                []tactic.ModelConverter
	ready                  bool
}

// bvSizeSigned is bv_util::norm(v, width, true) into out.
func bvSizeSigned(q *unumeral.RationalManager, v *unumeral.Rational, width uint32, out *unumeral.Rational) (resultErr error) {
	two, modulus, half := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&two), q.Del(&modulus), q.Del(&half)) }()
	if err := errors.Join(q.SetInt32(&two, 2), q.Power(&two, width, &modulus), q.Power(&two, width-1, &half)); err != nil {
		return err
	}
	if err := q.Mod(v, &modulus, out); err != nil {
		return err
	}
	if c, err := q.Compare(out, &half); err != nil {
		return err
	} else if c >= 0 {
		return q.Sub(out, &modulus, out)
	}
	return nil
}

// bvSizeNumBits is rational::get_num_bits() of a nonnegative integer.
func bvSizeNumBits(q *unumeral.RationalManager, v *unumeral.Rational) (uint32, error) {
	zero, err := q.IsZero(v)
	if err != nil || zero {
		return 1, err
	}
	return q.Bitsize(v)
}

func (r *bvSizeReductionImplementation) release(q *unumeral.RationalManager) (err error) {
	for _, table := range []map[ast.ASTHandle]*unumeral.Rational{r.lower, r.upper} {
		for h, v := range table {
			if e := q.Del(v); e != nil {
				err = errors.Join(err, e)
				continue
			}
			delete(table, h)
		}
	}
	return err
}
func (t *BVSizeReductionTactic) retain(r *bvSizeReductionImplementation, h ast.ASTHandle, scratch bool) error {
	if err := t.manager.IncRef(h); err != nil {
		return err
	}
	if scratch {
		r.scratch = append(r.scratch, h)
	} else {
		r.held = append(r.held, h)
	}
	return nil
}
func (t *BVSizeReductionTactic) releaseScratch(r *bvSizeReductionImplementation, limit int) error {
	for len(r.scratch) > limit {
		i := len(r.scratch) - 1
		if err := t.manager.DecRef(r.scratch[i]); err != nil {
			return err
		}
		r.scratch = r.scratch[:i]
	}
	return nil
}
func (t *BVSizeReductionTactic) updateBound(r *bvSizeReductionImplementation, h ast.ASTHandle, v *unumeral.Rational, lower bool) error {
	if !r.keys[h] {
		if err := t.retain(r, h, false); err != nil {
			return err
		}
		r.keys[h] = true
	}
	table, order := r.upper, r.upperOrder
	if lower {
		table, order = r.lower, r.lowerOrder
	}
	// update_signed_{lower,upper}: insert_if_not_there2, then improve.
	if _, err := order.InsertIfNotThere(h); err != nil {
		return err
	}
	q := t.manager.NumeralService().Manager
	old, found := table[h]
	if !found {
		old = new(unumeral.Rational)
		*old = unumeral.NewRational()
		table[h] = old
		return q.Copy(old, v)
	}
	c, err := q.Compare(old, v)
	if err != nil {
		return err
	}
	if lower && c < 0 || !lower && c > 0 {
		return q.Copy(old, v)
	}
	return nil
}
func (t *BVSizeReductionTactic) uninterpretedConstant(h ast.ASTHandle) (bool, error) {
	kind, err := t.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := t.manager.GetFamilyID(h)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	n, err := t.manager.GetNumArgs(h)
	return n == 0, err
}
func (t *BVSizeReductionTactic) collectBounds(r *bvSizeReductionImplementation, g *tactic.Goal) (resultErr error) {
	// numeral val;
	q := t.manager.NumeralService().Manager
	value, normalized := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&value), q.Del(&normalized)) }()
	n, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		f, err := g.Form(i)
		if err != nil {
			return err
		}
		negated, arg, err := t.manager.IsNot(f)
		if err != nil {
			return err
		}
		if negated {
			f = arg
		}
		isSLE, err := t.manager.IsBitVectorOperation(f, 23)
		if err != nil {
			return err
		}
		if !isSLE {
			continue
		}
		lhs, err := t.manager.GetArg(f, 0)
		if err != nil {
			return err
		}
		rhs, err := t.manager.GetArg(f, 1)
		if err != nil {
			return err
		}
		width, err := t.manager.BitVectorSize(lhs)
		if err != nil {
			return err
		}
		leftConst, err := t.uninterpretedConstant(lhs)
		if err != nil {
			return err
		}
		rightConst, err := t.uninterpretedConstant(rhs)
		if err != nil {
			return err
		}
		var variable ast.ASTHandle
		var found bool
		lower := false
		if leftConst {
			_, found, err = t.manager.BitVectorNumeralRational(rhs, &value)
			if err != nil {
				return err
			}
			if found {
				variable = lhs
			}
		}
		if !found && rightConst {
			_, found, err = t.manager.BitVectorNumeralRational(lhs, &value)
			if err != nil {
				return err
			}
			if found {
				variable = rhs
				lower = true
			}
		}
		if !found {
			continue
		}
		if err = t.traceCollected(f, negated, lower); err != nil {
			return err
		}
		// val = m_util.norm(val, bv_sz, true);
		if err = bvSizeSigned(q, &value, width, &value); err != nil {
			return err
		}
		if negated {
			if lower {
				err = q.Dec(&value)
			} else {
				err = q.Inc(&value)
			}
			if err != nil {
				return err
			}
			if err = bvSizeSigned(q, &value, width, &normalized); err != nil {
				return err
			}
			if equal, err := q.Equal(&normalized, &value); err != nil {
				return err
			} else if !equal {
				continue
			} // Source ignores an infeasible extreme bound.
			lower = !lower
		}
		if err = t.updateBound(r, variable, &value, lower); err != nil {
			return err
		}
	}
	return nil
}

func (t *BVSizeReductionTactic) definition(r *bvSizeReductionImplementation, v ast.ASTHandle, l, u *unumeral.Rational) (def, fresh ast.ASTHandle, err error) {
	width, err := t.manager.BitVectorSize(v)
	if err != nil {
		return def, fresh, err
	}
	q := t.manager.NumeralService().Manager
	if equal, e := q.Equal(l, u); e != nil {
		return def, fresh, e
	} else if equal {
		def, err = t.manager.BitVectorNumeralFromRational(l, width)
		return def, fresh, err
	}
	var bits uint32
	var mode int
	lNegative, err := q.IsNeg(l)
	if err != nil {
		return def, fresh, err
	}
	if lNegative {
		// unsigned l_nb = (-l).get_num_bits();
		negL := unumeral.NewRational()
		err = errors.Join(q.Copy(&negL, l), q.Neg(&negL))
		var lowerBits uint32
		if err == nil {
			lowerBits, err = bvSizeNumBits(q, &negL)
		}
		if err = errors.Join(err, q.Del(&negL)); err != nil {
			return def, fresh, err
		}
		uNegative, e := q.IsNeg(u)
		if e != nil {
			return def, fresh, e
		}
		if uNegative {
			bits = lowerBits
			mode = -1
		} else {
			if bits, err = bvSizeNumBits(q, u); err != nil {
				return def, fresh, err
			}
			if lowerBits > bits {
				bits = lowerBits
			}
			bits++
			mode = 1
		}
	} else {
		if bits, err = bvSizeNumBits(q, u); err != nil {
			return def, fresh, err
		}
		mode = 0
	}
	if err = t.traceWidth(v, l, u, bits, mode); err != nil {
		return def, fresh, err
	}
	if bits >= width {
		return def, fresh, nil
	}
	sort, err := t.manager.BitVectorSort(bits)
	if err != nil {
		return def, fresh, err
	}
	fresh, err = t.manager.MkFreshConstNullable(nil, sort)
	if err != nil {
		return def, fresh, err
	}
	if err = t.retain(r, fresh, true); err != nil {
		return def, fresh, err
	}
	if mode == 1 {
		def, err = t.manager.MkBitVectorSignExtend(width-bits, fresh)
		return def, fresh, err
	}
	var prefix ast.ASTHandle
	if mode < 0 {
		prefix, err = t.manager.BitVectorNumeralAllOnes(width - bits)
	} else {
		prefix, err = t.manager.BitVectorNumeralUnsigned(0, width-bits)
	}
	if err != nil {
		return def, fresh, err
	}
	if err = t.retain(r, prefix, true); err != nil {
		return def, fresh, err
	}
	def, err = t.manager.MkBitVectorConcatPair(prefix, fresh)
	return def, fresh, err
}

func (t *BVSizeReductionTactic) releaseConverters(r *bvSizeReductionImplementation, extension, filter bool) error {
	var err error
	if filter && r.filter != nil {
		owner := r.filter
		consumed, e := owner.DecRefConsumed()
		err = errors.Join(err, e)
		if consumed {
			r.filter = nil
			if e != nil {
				r.pending = append(r.pending, owner)
			}
		}
	}
	if extension && r.extension != nil {
		owner := r.extension
		consumed, e := owner.DecRefConsumed()
		err = errors.Join(err, e)
		if consumed {
			r.extension = nil
			if e != nil {
				r.pending = append(r.pending, owner)
			}
		}
	}
	return err
}
func (t *BVSizeReductionTactic) releaseSubstitutions(r *bvSizeReductionImplementation) error {
	for i := len(r.substitutions) - 1; i >= 0; i-- {
		owner := r.substitutions[i]
		if owner == r.active {
			continue
		}
		if err := t.manager.DelExprSubstitution(&owner.value); err != nil {
			return err
		}
		r.substitutions = append(r.substitutions[:i], r.substitutions[i+1:]...)
	}
	return nil
}
func (t *BVSizeReductionTactic) reduce(g *tactic.Goal, result *tactic.TacticResult) (count uint32, completed bool, err error) {
	r := t.current
	m := t.manager
	report, err := tactic.NewTacticReport("bv-size-reduction", g)
	if err != nil {
		return 0, false, err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	if err = t.collectBounds(r, g); err != nil {
		return 0, false, err
	}
	value, err := m.NewExprSubstitution()
	if err != nil {
		return 0, false, err
	}
	sub := &bvSizeSubstitution{value: value}
	r.substitutions = append(r.substitutions, sub)
	defer func() { err = errors.Join(err, t.releaseSubstitutions(r)) }()
	models, err := g.ModelsEnabled()
	if err != nil {
		return 0, false, err
	}
	if len(r.lower) != 0 && len(r.upper) != 0 {
		if err = t.traceBounds(r); err != nil {
			return 0, false, err
		}
		// numeral l, u;
		q := m.NumeralService().Manager
		l, u := unumeral.NewRational(), unumeral.NewRational()
		defer func() { err = errors.Join(err, q.Del(&l), q.Del(&u)) }()
		for _, slot := range r.lowerOrder.Entries() {
			variable := slot.Key
			lo := r.lower[variable]
			width, e := m.BitVectorSize(variable)
			if e != nil {
				return 0, false, e
			}
			up, found := r.upper[variable]
			if !found {
				continue
			}
			if e = errors.Join(bvSizeSigned(q, lo, width, &l), bvSizeSigned(q, up, width, &u)); e != nil {
				return 0, false, e
			}
			if e = t.traceInterval(variable, &l, &u); e != nil {
				return 0, false, e
			}
			if c, e := q.Compare(&l, &u); e != nil {
				return 0, false, e
			} else if c > 0 {
				f, e := m.MkFalse()
				if e != nil {
					return 0, false, e
				}
				return 0, false, g.Assert(f)
			}
			start := len(r.scratch)
			def, fresh, e := t.definition(r, variable, &l, &u)
			if e != nil {
				return 0, false, e
			}
			if !def.IsNull() {
				if e = t.retain(r, def, true); e != nil {
					return 0, false, e
				}
				if e = m.ExprSubstitutionInsert(sub.value, variable, def, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
					return 0, false, e
				}
				if models {
					if r.extension == nil {
						r.extension, e = tactic.NewExtensionModel(m, t.services)
						if e != nil {
							return 0, false, e
						}
						if e = r.extension.IncRef(); e != nil {
							return 0, false, e
						}
					}
					decl, e := m.GetDecl(variable)
					if e != nil {
						return 0, false, e
					}
					if e = r.extension.Insert(decl, def); e != nil {
						return 0, false, e
					}
					if !fresh.IsNull() {
						if r.filter == nil {
							r.filter, e = tactic.NewFilterModel(m)
							if e != nil {
								return 0, false, e
							}
							if e = r.filter.IncRef(); e != nil {
								return 0, false, e
							}
							if e = r.filter.BindTrace(t.trace, t.globals); e != nil {
								return 0, false, e
							}
						}
						d, e := m.GetDecl(fresh)
						if e != nil {
							return 0, false, e
						}
						if e = r.filter.Insert(d); e != nil {
							return 0, false, e
						}
					}
				}
				count++
			}
			if e = t.releaseScratch(r, start); e != nil {
				return 0, false, e
			}
		}
	}
	empty, err := m.ExprSubstitutionEmpty(sub.value)
	if err != nil {
		return 0, false, err
	}
	if empty {
		return count, false, nil
	}
	if err = r.replacer.SetSubstitution(&sub.value); err != nil {
		return 0, false, err
	}
	r.active = sub
	if err = t.releaseSubstitutions(r); err != nil {
		return 0, false, err
	}
	n, err := g.Size()
	if err != nil {
		return 0, false, err
	}
	for i := uint32(0); i < n; i++ {
		bad, e := g.Inconsistent()
		if e != nil {
			return 0, false, e
		}
		if bad {
			// expr_ref new_f dies with the early return.
			return count, false, m.ResetRef(r.out)
		}
		f, e := g.Form(i)
		if e != nil {
			return 0, false, e
		}
		if e = r.replacer.Rewrite(f, r.out); e != nil {
			return 0, false, e
		}
		newF, e := m.RefGet(r.out)
		if e != nil {
			return 0, false, e
		}
		if e = g.Update(i, newF, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
			return 0, false, e
		}
	}
	var converter tactic.ModelConverter
	if r.extension != nil {
		converter = r.extension
	}
	if r.filter != nil {
		converter, err = tactic.ConcatModels(m.Budget(), r.filter, converter)
		if err != nil {
			return 0, false, err
		}
	}
	if err = result.SetModelConverter(converter); err != nil {
		if converter != nil && converter != r.filter && converter != r.extension {
			err = errors.Join(err, converter.Close())
		}
		return 0, false, err
	}
	if err = t.releaseConverters(r, true, true); err != nil {
		return 0, false, err
	}
	// new_f is scoped to the substitution block, after m_mc and m_fmc reset.
	if err = m.ResetRef(r.out); err != nil {
		return 0, false, err
	}
	return count, true, nil
}
