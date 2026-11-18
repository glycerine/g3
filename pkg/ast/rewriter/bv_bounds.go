package rewriter

// Source: src/ast/rewriter/bv_bounds.cpp; src/ast/rewriter/bv_bounds.h.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type bvBoundInterval struct {
	lo, hi  *unumeral.Rational
	negated bool
	value   ast.ASTHandle
}

type bvConversion uint8

const (
	bvConverted bvConversion = iota
	bvUnsat
	bvUndefined
)

// BVBounds is the source polynomial-time interval checker for bit-vector
// constraints. Native maps hold the values; the ObjMaps beside them hold the
// obj_map<app, ...> table layouts that the source iterates.
type BVBounds struct {
	self                       *BVBounds
	manager                    *ast.ASTManager
	lowers, uppers, singletons map[ast.ASTHandle]*unumeral.Rational
	negative                   map[ast.ASTHandle][]bvBoundInterval
	// m_unsigned_lowers, m_unsigned_uppers, m_singletons and
	// m_negative_intervals table orders.
	lowerOrder, upperOrder, singletonOrder, negativeOrder *ast.ObjMap
	okay, closed                                          bool
	num                                                   *pbNumbers
}

// bvBoundsKeys lists a table's keys in iteration order.
func bvBoundsKeys(o *ast.ObjMap) []ast.ASTHandle {
	entries := o.Entries()
	keys := make([]ast.ASTHandle, len(entries))
	for i := range entries {
		keys[i] = entries[i].Key
	}
	return keys
}

func NewBVBounds(m *ast.ASTManager) (*BVBounds, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.RegisterBitVectorPlugin(); err != nil {
		return nil, err
	}
	b := &BVBounds{manager: m, okay: true}
	b.self = b
	if err := b.Reset(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *BVBounds) open() error {
	if b == nil || b.self != b || b.closed || b.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

// The bounds' numerals (numeral = rational, of the AST manager's
// NumeralService().Manager) all live in b.num, released by Reset/Close: the
// source's bv_bounds is a short-lived local of one reduce_app_core. The
// helpers below return fresh rationals of b.num; operation failures are
// sticky in b.num.err and reported by the public entry points.

func (b *BVBounds) z(v int64) *unumeral.Rational {
	out := b.num.fresh()
	b.num.setInt(out, v)
	return out
}
func (b *BVBounds) cpy(v *unumeral.Rational) *unumeral.Rational {
	out := b.num.fresh()
	b.num.set(out, v)
	return out
}

// pow is rational::power_of_two(width).
func (b *BVBounds) pow(width uint32) *unumeral.Rational {
	out := b.z(2)
	b.num.fail(b.num.q.Power(out, width, out))
	return out
}

// max is power_of_two(width) - 1.
func (b *BVBounds) max(width uint32) *unumeral.Rational { return b.subR(b.pow(width), b.z(1)) }
func (b *BVBounds) addR(x, y *unumeral.Rational) *unumeral.Rational {
	out := b.num.fresh()
	b.num.add(out, x, y)
	return out
}
func (b *BVBounds) subR(x, y *unumeral.Rational) *unumeral.Rational {
	out := b.num.fresh()
	b.num.sub(out, x, y)
	return out
}
func (b *BVBounds) negR(x *unumeral.Rational) *unumeral.Rational {
	out := b.cpy(x)
	b.num.neg(out)
	return out
}
func (b *BVBounds) cmp(x, y *unumeral.Rational) int { return b.num.cmp(x, y) }

// numeralFailure reports a sticky numeral failure from a public entry point.
func (b *BVBounds) numeralFailure(err *error) {
	if b != nil && b.num != nil && b.num.err != nil && *err == nil {
		*err = b.num.err
	}
}

func (b *BVBounds) str(x *unumeral.Rational) string {
	text, err := b.num.q.String(x)
	b.num.fail(err)
	return text
}

func (b *BVBounds) debugInterval(width uint32, lo, hi *unumeral.Rational) error {
	if ast.ASTSourceDebug && (lo == nil || hi == nil || lo.IsNeg() || b.cmp(lo, hi) > 0 || b.cmp(hi, b.pow(width)) >= 0) {
		return ast.ErrASTDomain
	}
	return nil
}

func (b *BVBounds) Reset() (err error) {
	if b == nil {
		return nil
	}
	if b.num != nil {
		err = b.num.release()
	}
	b.num = newPBNumbers(b.manager)
	b.lowers = make(map[ast.ASTHandle]*unumeral.Rational)
	b.uppers = make(map[ast.ASTHandle]*unumeral.Rational)
	b.singletons = make(map[ast.ASTHandle]*unumeral.Rational)
	b.negative = make(map[ast.ASTHandle][]bvBoundInterval)
	b.lowerOrder, b.upperOrder = ast.NewObjMap(b.manager), ast.NewObjMap(b.manager)
	b.singletonOrder, b.negativeOrder = ast.NewObjMap(b.manager), ast.NewObjMap(b.manager)
	b.okay = true
	return err
}

func (b *BVBounds) Close() error {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	err := b.Reset()
	if err == nil {
		err = b.num.release()
	}
	b.closed = true
	return err
}

func (b *BVBounds) IsOkay() bool { return b != nil && b.self == b && !b.closed && b.okay }

// Singletons returns independent integer copies of the source singleton map.
func (b *BVBounds) Singletons() (map[ast.ASTHandle]*unumeral.Rational, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	out := make(map[ast.ASTHandle]*unumeral.Rational, len(b.singletons))
	for h, v := range b.singletons {
		out[h] = b.cpy(v)
	}
	return out, nil
}

func (b *BVBounds) isOperation(h ast.ASTHandle, family ast.FamilyID, kind int32) (bool, error) {
	k, err := b.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := b.manager.GetFamilyID(h)
	if err != nil || f != family {
		return false, err
	}
	op, err := b.manager.GetDeclKind(h)
	return op == kind, err
}

func (b *BVBounds) numeral(h ast.ASTHandle) (*unumeral.Rational, uint32, bool, error) {
	// m_bv_util.is_numeral(e, val, bv_sz)
	value := b.num.fresh()
	width, matched, err := b.manager.BitVectorNumeralRational(h, value)
	if err != nil || !matched {
		return nil, 0, false, err
	}
	return value, width, true, nil
}

func (b *BVBounds) toBound(h ast.ASTHandle) (bool, error) {
	k, err := b.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	yes, err := b.manager.IsBitVector(h)
	if err != nil || !yes {
		return false, err
	}
	if yes, err = b.manager.IsBitVectorOperation(h, 4); err != nil || yes { // OP_BADD
		return false, err
	}
	yes, err = b.manager.IsBitVectorNumeral(h)
	return !yes, err
}

func (b *BVBounds) constantAdd(width uint32, h ast.ASTHandle) (ast.ASTHandle, *unumeral.Rational, bool, error) {
	if ast.ASTSourceDebug {
		actual, err := b.manager.BitVectorSize(h)
		if err != nil {
			return ast.ASTHandle{}, nil, false, err
		}
		if actual != width {
			return ast.ASTHandle{}, nil, false, ast.ErrASTDomain
		}
	}
	var left, right ast.ASTHandle
	yes, err := b.manager.BitVectorAddOperands(h, &left, &right)
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	if !yes {
		return h, b.z(0), true, nil
	}
	if bound, e := b.toBound(left); e != nil {
		return ast.ASTHandle{}, nil, false, e
	} else if bound {
		if value, w, found, e := b.numeral(right); e != nil {
			return ast.ASTHandle{}, nil, false, e
		} else if found && w == width {
			return left, value, true, nil
		}
	}
	if bound, e := b.toBound(right); e != nil {
		return ast.ASTHandle{}, nil, false, e
	} else if bound {
		if value, w, found, e := b.numeral(left); e != nil {
			return ast.ASTHandle{}, nil, false, e
		} else if found && w == width {
			return right, value, true, nil
		}
	}
	return ast.ASTHandle{}, nil, false, nil
}

func (b *BVBounds) record(value ast.ASTHandle, lo, hi *unumeral.Rational, negated bool, intervals *[]bvBoundInterval) (bvConversion, error) {
	marker := "["
	if negated {
		marker = "~["
	}
	if err := b.traceValue("record", 25, "record0 ", value, ":"+marker+b.str(lo)+";"+b.str(hi)+"]"); err != nil {
		return bvUndefined, err
	}
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return bvUndefined, err
	}
	if err = b.debugInterval(width, lo, hi); err != nil {
		return bvUndefined, err
	}
	minimum := b.z(0)
	maximum := b.max(width)
	if current, found := b.lowers[value]; found {
		minimum = b.cpy(current)
	}
	if current, found := b.uppers[value]; found {
		maximum = b.cpy(current)
	}
	lo, hi = b.cpy(lo), b.cpy(hi)
	loMin := b.cmp(lo, minimum) <= 0
	hiMax := b.cmp(hi, maximum) >= 0
	if negated {
		if loMin && hiMax {
			return bvUnsat, nil
		}
		if b.cmp(lo, maximum) > 0 || b.cmp(hi, minimum) < 0 {
			return bvConverted, nil
		}
		if loMin {
			negated = false
			b.num.add(lo, hi, b.z(1))
			b.num.set(hi, maximum)
			loMin, hiMax = b.cmp(lo, minimum) <= 0, true
		} else if hiMax {
			negated = false
			b.num.sub(hi, lo, b.z(1))
			b.num.set(lo, minimum)
			hiMax, loMin = b.cmp(hi, maximum) >= 0, true
		}
	}
	if loMin {
		b.num.set(lo, minimum)
	}
	if hiMax {
		b.num.set(hi, maximum)
	}
	marker = "["
	if negated {
		marker = "~["
	}
	if err = b.traceValue("record", 57, "record1 ", value, ":"+marker+b.str(lo)+";"+b.str(hi)+"]"); err != nil {
		return bvUndefined, err
	}
	if b.cmp(lo, hi) > 0 {
		if negated {
			return bvConverted, nil
		}
		return bvUnsat, nil
	}
	if loMin && hiMax {
		if negated {
			return bvUnsat, nil
		}
		return bvConverted, nil
	}
	*intervals = append(*intervals, bvBoundInterval{lo: lo, hi: hi, negated: negated, value: value})
	return bvConverted, nil
}

func (b *BVBounds) BoundLower(value ast.ASTHandle, lower *unumeral.Rational) (_ bool, resultErr error) {
	defer b.numeralFailure(&resultErr)
	if err := b.open(); err != nil {
		return false, err
	}
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return false, err
	}
	if ast.ASTSourceDebug && (lower == nil || lower.IsNeg() || b.cmp(lower, b.pow(width)) >= 0) {
		return false, ast.ErrASTDomain
	}
	if err = b.traceValue("bound_lo", 524, "lower ", value, ":"+b.str(lower)); err != nil {
		return false, err
	}
	current, found := b.lowers[value]
	if !found {
		if _, err = b.lowerOrder.InsertIfNotThere(value); err != nil {
			return false, err
		}
	}
	if !found || b.cmp(current, lower) < 0 {
		b.lowers[value] = b.cpy(lower)
	}
	return b.okay, nil
}

func (b *BVBounds) BoundUpper(value ast.ASTHandle, upper *unumeral.Rational) (_ bool, resultErr error) {
	defer b.numeralFailure(&resultErr)
	if err := b.open(); err != nil {
		return false, err
	}
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return false, err
	}
	if ast.ASTSourceDebug && (upper == nil || upper.IsNeg() || b.cmp(upper, b.pow(width)) >= 0) {
		return false, ast.ErrASTDomain
	}
	if err = b.traceValue("bound_up", 535, "upper ", value, ":"+b.str(upper)); err != nil {
		return false, err
	}
	current, found := b.uppers[value]
	if !found {
		if _, err = b.upperOrder.InsertIfNotThere(value); err != nil {
			return false, err
		}
	}
	if !found || b.cmp(upper, current) < 0 {
		b.uppers[value] = b.cpy(upper)
	}
	return b.okay, nil
}

func (b *BVBounds) addNegative(value ast.ASTHandle, lo, hi *unumeral.Rational) error {
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return err
	}
	if err = b.debugInterval(width, lo, hi); err != nil {
		return err
	}
	if err = b.traceValue("add_neg_bound", 545, "negative bound ", value, ":"+b.str(lo)+";"+b.str(hi)); err != nil {
		return err
	}
	if _, found := b.negative[value]; !found {
		if _, err = b.negativeOrder.InsertIfNotThere(value); err != nil {
			return err
		}
	}
	b.negative[value] = append(b.negative[value], bvBoundInterval{lo: b.cpy(lo), hi: b.cpy(hi), value: value})
	return nil
}

func (b *BVBounds) AddBoundUnsigned(value ast.ASTHandle, lo, hi *unumeral.Rational, negated bool) (_ bool, resultErr error) {
	defer b.numeralFailure(&resultErr)
	if err := b.open(); err != nil {
		return false, err
	}
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return false, err
	}
	if err = b.debugInterval(width, lo, hi); err != nil {
		return false, err
	}
	marker := "["
	if negated {
		marker = "~["
	}
	if err = b.traceValue("add_bound_unsigned", 453, "bound_unsigned ", value, ": "+marker+b.str(lo)+";"+b.str(hi)+"]"); err != nil {
		return false, err
	}
	one, maximum := b.z(1), b.max(width)
	loMin, hiMax := lo.IsZero(), b.cmp(hi, maximum) == 0
	if negated {
		switch {
		case loMin && hiMax:
			b.okay = false
		case loMin:
			_, err = b.BoundLower(value, b.addR(hi, one))
		case hiMax:
			_, err = b.BoundUpper(value, b.subR(lo, one))
		default:
			err = b.addNegative(value, lo, hi)
		}
		return b.okay, err
	}
	if !loMin {
		if _, err = b.BoundLower(value, lo); err != nil {
			return false, err
		}
	}
	if !hiMax {
		if _, err = b.BoundUpper(value, hi); err != nil {
			return false, err
		}
	}
	return b.okay, nil
}

func (b *BVBounds) convertSigned(value ast.ASTHandle, lo, hi *unumeral.Rational, negated bool, intervals *[]bvBoundInterval) (bvConversion, error) {
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return bvUndefined, err
	}
	if ast.ASTSourceDebug && (lo == nil || hi == nil || b.cmp(lo, hi) > 0) {
		return bvUndefined, ast.ErrASTDomain
	}
	marker := "["
	if negated {
		marker = "~["
	}
	if err = b.traceValue("convert_signed", 476, "convert_signed ", value, ":"+marker+b.str(lo)+";"+b.str(hi)+"]"); err != nil {
		return bvUndefined, err
	}
	zero, one, modulus := b.z(0), b.z(1), b.pow(width)
	loNegative, hiNegative := lo.IsNeg(), hi.IsNeg()
	if !loNegative && !hiNegative {
		return b.record(value, lo, hi, negated, intervals)
	}
	if loNegative && hiNegative {
		return b.record(value, b.addR(modulus, lo), b.addR(modulus, hi), negated, intervals)
	}
	if negated {
		a, err := b.record(value, b.addR(modulus, lo), b.subR(modulus, one), true, intervals)
		if err != nil || a == bvUnsat {
			return a, err
		}
		c, err := b.record(value, zero, hi, true, intervals)
		if err != nil || c == bvUnsat {
			return c, err
		}
		return bvConverted, nil
	}
	lower := b.addR(hi, one)
	upper := b.subR(b.addR(modulus, lo), one)
	if b.cmp(lower, upper) <= 0 {
		return b.record(value, lower, upper, true, intervals)
	}
	return bvConverted, nil
}

func (b *BVBounds) AddBoundSigned(value ast.ASTHandle, lo, hi *unumeral.Rational, negated bool) (_ bool, resultErr error) {
	defer b.numeralFailure(&resultErr)
	marker := " "
	if negated {
		marker = "~"
	}
	if err := b.traceValue("add_bound_signed", 500, "bound_signed ", value, ":"+marker+b.str(lo)+";"+b.str(hi)); err != nil {
		return false, err
	}
	var intervals []bvBoundInterval
	converted, err := b.convertSigned(value, lo, hi, negated, &intervals)
	if err != nil {
		return false, err
	}
	if converted == bvUnsat {
		b.okay = false
		return false, nil
	}
	for _, interval := range intervals {
		if _, err = b.AddBoundUnsigned(interval.value, interval.lo, interval.hi, interval.negated); err != nil {
			return false, err
		}
	}
	return b.okay, nil
}

func (b *BVBounds) satisfiableValue(value ast.ASTHandle) (result bool, err error) {
	if err := b.traceValue("is_sat", 606, "is_sat ", value, ""); err != nil {
		return false, err
	}
	defer func() {
		resultText := "0"
		if result {
			resultText = "1"
		}
		traceErr := b.traceValue("is_sat", 608, "is_sat ", value, "\nres: "+resultText)
		err = errors.Join(err, traceErr)
	}()
	width, err := b.manager.BitVectorSize(value)
	if err != nil {
		return false, err
	}
	lower, upper := b.z(0), b.max(width)
	if v, found := b.lowers[value]; found {
		lower = b.cpy(v)
	}
	if v, found := b.uppers[value]; found {
		upper = b.cpy(v)
	}
	if b.cmp(lower, upper) > 0 {
		return false, nil
	}
	if err = b.traceText("is_sat_core", 623, "is_sat bound:"+b.str(lower)+"-"+b.str(upper)); err != nil {
		return false, err
	}
	intervals := b.negative[value]
	sort.Slice(intervals, func(i, j int) bool { return b.cmp(intervals[i].lo, intervals[j].lo) < 0 })
	one := b.z(1)
	ptr := b.cpy(lower)
	newLo := b.cpy(lower)
	newHi := b.subR(lower, one)
	sat := false
	for _, interval := range intervals {
		if b.cmp(ptr, interval.hi) > 0 {
			continue
		}
		if b.cmp(ptr, interval.lo) < 0 {
			if !sat {
				b.num.set(newLo, ptr)
			}
			b.num.sub(newHi, interval.lo, one)
			if b.cmp(newHi, upper) > 0 {
				b.num.set(newHi, upper)
			}
			sat = true
		}
		if err = b.traceText("is_sat_core", 644, "is_sat new_lo, new_hi:"+b.str(newLo)+"-"+b.str(newHi)); err != nil {
			return false, err
		}
		b.num.add(ptr, interval.hi, one)
		if err = b.traceText("is_sat_core", 646, "is_sat ptr, new_hi:"+b.str(ptr)+"-"+b.str(newHi)); err != nil {
			return false, err
		}
		if b.cmp(ptr, upper) > 0 {
			break
		}
	}
	if b.cmp(ptr, upper) <= 0 {
		if !sat {
			b.num.set(newLo, ptr)
		}
		b.num.set(newHi, upper)
		sat = true
	}
	if b.cmp(newHi, upper) < 0 {
		_, err = b.BoundUpper(value, newHi)
	}
	if err == nil && b.cmp(newLo, lower) > 0 {
		_, err = b.BoundLower(value, newLo)
	}
	if err != nil {
		return false, err
	}
	if err = b.traceText("is_sat_core", 658, "is_sat new_lo, new_hi:"+b.str(newLo)+"-"+b.str(newHi)); err != nil {
		return false, err
	}
	if sat && b.cmp(newLo, newHi) == 0 {
		if _, found := b.singletons[value]; found {
			if ast.ASTSourceDebug {
				return false, ast.ErrASTDomain
			}
		} else {
			if err = b.traceValue("record_singleton", 600, "singleton:", value, ":"+b.str(newLo)); err != nil {
				return false, err
			}
			if _, err = b.singletonOrder.InsertIfNotThere(value); err != nil {
				return false, err
			}
			b.singletons[value] = b.cpy(newLo)
		}
	}
	return sat, nil
}

func (b *BVBounds) IsSatisfiable() (_ bool, resultErr error) {
	defer b.numeralFailure(&resultErr)
	if err := b.open(); err != nil {
		return false, err
	}
	if !b.okay {
		return false, nil
	}
	// Source is_sat walks m_unsigned_lowers, m_unsigned_uppers, then
	// m_negative_intervals in table order. is_sat(v) only finds or adds
	// keys of the other tables, so a snapshot of each is its iteration.
	seen := make(map[ast.ASTHandle]bool)
	for _, table := range []*ast.ObjMap{b.lowerOrder, b.upperOrder} {
		for _, value := range bvBoundsKeys(table) {
			if seen[value] {
				continue
			}
			seen[value] = true
			if yes, err := b.satisfiableValue(value); err != nil || !yes {
				return yes, err
			}
		}
	}
	for _, value := range bvBoundsKeys(b.negativeOrder) {
		if seen[value] {
			continue
		}
		if yes, err := b.satisfiableValue(value); err != nil || !yes {
			return yes, err
		}
	}
	return true, nil
}

func (b *BVBounds) bitVectorNumeral(value *unumeral.Rational, width uint32) (result ast.ASTHandle, err error) {
	service := b.manager.NumeralService()
	n, err := service.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, service.Del(&n)) }()
	if err = service.SetString(&n, b.str(value)); err != nil {
		return result, err
	}
	return b.manager.BitVectorNumeral(&n, width)
}

// Rewrite implements bv_bounds::rewrite for bounded Boolean conjunctions and
// disjunctions. The object retains the accumulated bounds, as in the source.
func (b *BVBounds) Rewrite(limit uint32, declaration ast.ASTHandle, arguments ast.AppArguments, out ast.ASTRef) (_ RewriteStatus, resultErr error) {
	defer b.numeralFailure(&resultErr)
	if err := b.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := b.manager.GetFamilyID(declaration)
	if err != nil || family != b.manager.BasicFamilyID() {
		return RewriteFailed, err
	}
	kind, err := b.manager.GetDeclKind(declaration)
	if err != nil || kind != ast.ASTOpAnd && kind != ast.ASTOpOr || arguments.Count() > limit {
		return RewriteFailed, err
	}
	negated := kind == ast.ASTOpOr
	type convertedArgument struct {
		original  ast.ASTHandle
		undefined bool
		intervals []bvBoundInterval
	}
	converted := make([]convertedArgument, 0, arguments.Count())
	for i := uint32(0); i < arguments.Count() && b.okay; i++ {
		argument, e := arguments.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		entry := convertedArgument{original: argument}
		status, e := b.convert(argument, &entry.intervals, negated)
		if e != nil {
			return RewriteFailed, e
		}
		entry.undefined = status == bvUndefined
		converted = append(converted, entry)
		if status == bvUnsat {
			b.okay = false
			break
		}
		for _, interval := range entry.intervals {
			if _, e = b.AddBoundUnsigned(interval.value, interval.lo, interval.hi, interval.negated); e != nil {
				return RewriteFailed, e
			}
		}
	}
	sat, err := b.IsSatisfiable()
	if err != nil {
		return RewriteFailed, err
	}
	if !b.okay || !sat {
		value, e := b.manager.MkBoolVal(negated)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, b.manager.SetRef(out, value)
	}
	remaining := make([]ast.ASTHandle, 0, len(converted)+len(b.singletons))
	hasSingleton := false
	for _, entry := range converted {
		if err = b.traceValue("rewrite", 284, "check red: ", entry.original, ""); err != nil {
			return RewriteFailed, err
		}
		if entry.undefined {
			if err = b.traceText("rewrite", 286, "unprocessed"); err != nil {
				return RewriteFailed, err
			}
			remaining = append(remaining, entry.original)
			continue
		}
		redundant := len(entry.intervals) == 0
		isSingleton := false
		if !redundant {
			value := entry.intervals[0].value
			width, e := b.manager.BitVectorSize(value)
			if e != nil {
				return RewriteFailed, e
			}
			lo, hi := b.z(0), b.max(width)
			if v, found := b.lowers[value]; found {
				lo = v
			}
			if v, found := b.uppers[value]; found {
				hi = v
			}
			if err = b.traceValue("rewrite", 302, "bounds: ", value, "["+b.str(lo)+"-"+b.str(hi)+"]"); err != nil {
				return RewriteFailed, err
			}
			isSingleton = b.cmp(lo, hi) == 0
		}
		if !redundant && !isSingleton {
			remaining = append(remaining, entry.original)
		}
		hasSingleton = hasSingleton || isSingleton
		if err = b.traceValueIf(redundant, "rewrite", 308, "redundant: ", entry.original, ""); err != nil {
			return RewriteFailed, err
		}
	}
	if len(remaining) == int(arguments.Count()) && !hasSingleton {
		return RewriteFailed, nil
	}
	for _, value := range bvBoundsKeys(b.singletonOrder) {
		singleton := b.singletons[value]
		width, e := b.manager.BitVectorSize(value)
		if e != nil {
			return RewriteFailed, e
		}
		numeral, e := b.bitVectorNumeral(singleton, width)
		if e != nil {
			return RewriteFailed, e
		}
		equality, e := b.manager.MkEq(value, numeral)
		if e != nil {
			return RewriteFailed, e
		}
		if negated {
			equality, e = b.manager.MkNot(equality)
			if e != nil {
				return RewriteFailed, e
			}
		}
		remaining = append(remaining, equality)
	}
	var result ast.ASTHandle
	switch len(remaining) {
	case 0:
		result, err = b.manager.MkBoolVal(!negated)
	case 1:
		result = remaining[0]
	default:
		if negated {
			result, err = b.manager.MkOrArgs(remaining)
		} else {
			result, err = b.manager.MkAndArgs(remaining)
		}
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, b.manager.SetRef(out, result)
}
