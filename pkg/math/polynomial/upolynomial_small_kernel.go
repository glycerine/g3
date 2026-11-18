package polynomial

// Source: src/math/polynomial/upolynomial.cpp coefficient loops (set, rem,
// mk_monic, mul) over src/util/mpzzp.h inline values.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// upolySmallOps lets a coefficient loop compute the mpzzp operations of
// inline (small) values directly, as z3's is_small paths do, instead of
// resolving the numeric caller context for every coefficient, and run the
// other operations on the integer manager directly (direct). It is taken
// once per loop (smallOps) and is off in source debug, trace and checked
// builds, and while any failure transport or callback runs. The small
// methods return false, changing nothing, when an operand or the result is
// not inline or the target holds a big cell; the *Op methods then run the
// direct operation, and report false only when the loop must perform the
// full operation itself. Every path publishes the same value.
type upolySmallOps struct {
	on, direct bool
	unumeral.SmallModulus
	mod     *unumeral.IntegerModularManager
	z       *unumeral.IntegerManager
	scratch *unumeral.CRAScratch
}

// smallOpsFor is smallOps for a loop running under context. The root-ID,
// Sturm and s-expression arithmetic frames retain the receipts of their
// numeral operations (copies, add-mul children) for their own cleanup, so
// their loops always take the full operations.
func (m *UPolynomialManager) smallOpsFor(context *integerAllocationInvocationContext) upolySmallOps {
	if context.rootID != nil || context.sequenceSturm != nil || context.sexprArithmetic != nil {
		return upolySmallOps{}
	}
	return m.smallOps()
}

func (m *UPolynomialManager) smallOps() upolySmallOps {
	if polynomialValidateOwnership || m.modular == nil || !m.modular.DirectKernel() {
		return upolySmallOps{}
	}
	s, ok := m.modular.SmallKernel()
	if m.kernelScratch == nil {
		m.kernelScratch = new(unumeral.CRAScratch)
	}
	return upolySmallOps{on: ok, direct: true, SmallModulus: s, mod: m.modular, z: m.modular.Integers(), scratch: m.kernelScratch}
}

func (s upolySmallOps) setOp(dst, src *unumeral.Int) (bool, error) {
	if s.set(s.z, dst, src) {
		return true, nil
	}
	if !s.direct {
		return false, nil
	}
	return true, s.mod.DirectCopy(dst, src)
}

func (s upolySmallOps) mulOp(a, b, out *unumeral.Int) (bool, error) {
	if s.mul(a, b, out) {
		return true, nil
	}
	if !s.direct {
		return false, nil
	}
	if done, e := s.mod.DirectMulAdd(nil, a, b, out, false, s.scratch); done || e != nil {
		return true, e
	}
	return true, s.mod.DirectMul(a, b, out)
}

func (s upolySmallOps) subMulOp(a, b, c, d *unumeral.Int) (bool, error) {
	if s.subMul(a, b, c, d) {
		return true, nil
	}
	if !s.direct {
		return false, nil
	}
	if done, e := s.mod.DirectMulAdd(a, b, c, d, true, s.scratch); done || e != nil {
		return true, e
	}
	return true, s.mod.DirectSubMul(a, b, c, d)
}

func (s upolySmallOps) addMulOp(a, b, c, d *unumeral.Int) (bool, error) {
	if s.addMul(a, b, c, d) {
		return true, nil
	}
	if !s.direct {
		return false, nil
	}
	if done, e := s.mod.DirectMulAdd(a, b, c, d, false, s.scratch); done || e != nil {
		return true, e
	}
	return true, s.mod.DirectAddMul(a, b, c, d)
}

func fitsInline(v int64) bool { return v >= -1<<31 && v <= 1<<31-1 }

// set is mpzzp set(dst, src): copy, then p_normalize. A big source reduced
// into an inline modulus is read in place: z3 copies it and rem replaces
// the copy by the inline remainder, releasing the cell again.
func (s upolySmallOps) set(z *unumeral.IntegerManager, dst, src *unumeral.Int) bool {
	if !s.on || dst.Cell != 0 {
		return false
	}
	if src.Cell != 0 {
		if s.Z {
			return false
		}
		dst.Val = int32(s.Normalize(z.RemSmall(src, uint32(s.P))))
		return true
	}
	dst.Val = int32(s.Normalize(int64(src.Val)))
	return true
}

// mul is mpzzp mul(a, b, out).
func (s upolySmallOps) mul(a, b, out *unumeral.Int) bool {
	if !s.on || a.Cell != 0 || b.Cell != 0 || out.Cell != 0 {
		return false
	}
	v := s.Normalize(int64(a.Val) * int64(b.Val))
	if !fitsInline(v) {
		return false
	}
	out.Val = int32(v)
	return true
}

// subMul is mpzzp submul(a, b, c, d): d = a - b*c, then p_normalize.
func (s upolySmallOps) subMul(a, b, c, d *unumeral.Int) bool {
	if !s.on || a.Cell != 0 || b.Cell != 0 || c.Cell != 0 || d.Cell != 0 {
		return false
	}
	v := s.Normalize(int64(a.Val) - int64(b.Val)*int64(c.Val))
	if !fitsInline(v) {
		return false
	}
	d.Val = int32(v)
	return true
}

// addMul is mpzzp addmul(a, b, c, d): d = a + b*c, then p_normalize.
func (s upolySmallOps) addMul(a, b, c, d *unumeral.Int) bool {
	if !s.on || a.Cell != 0 || b.Cell != 0 || c.Cell != 0 || d.Cell != 0 {
		return false
	}
	v := s.Normalize(int64(a.Val) + int64(b.Val)*int64(c.Val))
	if !fitsInline(v) {
		return false
	}
	d.Val = int32(v)
	return true
}

// add is mpzzp add(a, b, c).
func (s upolySmallOps) add(a, b, c *unumeral.Int) bool {
	if !s.on || a.Cell != 0 || b.Cell != 0 || c.Cell != 0 {
		return false
	}
	v := s.Normalize(int64(a.Val) + int64(b.Val))
	if !fitsInline(v) {
		return false
	}
	c.Val = int32(v)
	return true
}

func (s upolySmallOps) addOp(a, b, c *unumeral.Int) (bool, error) {
	if s.add(a, b, c) {
		return true, nil
	}
	if !s.direct {
		return false, nil
	}
	return true, s.mod.DirectAdd(a, b, c)
}

// divOp is div's small path; other operands take the full operation.
func (s upolySmallOps) divOp(a, b, c *unumeral.Int) bool {
	return s.div(a, b, c)
}

// div is mpzzp div(a, b, c) of a prime field: a * inv(b).
func (s upolySmallOps) div(a, b, c *unumeral.Int) bool {
	if !s.on || s.Z || !s.Field || a.Cell != 0 || b.Cell != 0 || c.Cell != 0 || s.Normalize(int64(b.Val)) == 0 {
		return false
	}
	c.Val = int32(s.Normalize(int64(a.Val) * s.Inv(int64(b.Val))))
	return true
}

// viewValues is read's validation done once for a whole view; an empty
// view, which read never resolves, has no values.
func (m *UPolynomialManager) viewValues(v UPolynomialView) ([]unumeral.Int, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	if v.count == 0 {
		return nil, nil
	}
	store := m.boundStore(v.origin)
	if store == nil || v.backing == 0 || uint64(v.backing) > uint64(len(store.backings)) {
		return nil, ErrUPolynomialOwner
	}
	b := &store.backings[v.backing-1]
	if !b.live || b.generation != v.generation {
		return nil, ErrUPolynomialOwner
	}
	if uint64(v.start)+uint64(v.count) > uint64(len(b.values)) {
		return nil, arena.ErrBounds
	}
	return b.values[v.start : v.start+v.count], nil
}
