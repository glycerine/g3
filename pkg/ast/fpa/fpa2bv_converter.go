package fpa

// Source: src/ast/fpa/fpa2bv_converter.h/.cpp (class fpa2bv_converter:
// constructor, members, reset, mk_bv_uf) and the bv_util/fpa_util
// constructors it calls.
//
// Conventions of this port:
//   - Each source function keeps its name (mk_add -> mkAdd).
//   - Source expr_ref locals are frame slots released in reverse
//     declaration order when the function returns; nested constructor calls
//     are raw handles, as in the source.
//   - Arguments that are themselves constructor calls are evaluated right to
//     left, as GCC does for the source's function arguments.
//   - Errors propagate by panic(fpaFailure) and are recovered at the
//     exported entry points.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// BoolSimplifier is the converter's m_simp (a default bool_rewriter). The
// rewriter package supplies it; it cannot be imported here because it
// imports this package.
type BoolSimplifier interface {
	MkAnd(args []ast.ASTHandle, out ast.ASTRef) error
	MkOr(args []ast.ASTHandle, out ast.ASTRef) error
	MkNot(arg ast.ASTHandle, out ast.ASTRef) error
	MkIte(c, t, e ast.ASTHandle, out ast.ASTRef) error
	MkEq(a, b ast.ASTHandle, out ast.ASTRef) error
	MkXor(a, b ast.ASTHandle, out ast.ASTRef) error
	Close() error
}

// BV_RM_VAL.
const (
	bvRMTiesToAway = 0
	bvRMTiesToEven = 1
	bvRMToNegative = 2
	bvRMToPositive = 3
	bvRMToZero     = 4
)

// FPA2BVConverter is fpa2bv_converter.
type FPA2BVConverter struct {
	self            *FPA2BVConverter
	m               *ast.ASTManager
	simp            BoolSimplifier
	fid             ast.FamilyID // m_util (fpa) family
	bvFid           ast.FamilyID
	header          uint64
	closed          bool
	hiFPUnspecified bool
	// m_const2bv, m_rm_const2bv, m_uf2bvuf: obj_map<func_decl, ...> whose
	// keys and values the converter holds a reference to.
	const2bv   *ast.ObjMap
	rmConst2bv *ast.ObjMap
	uf2bvuf    *ast.ObjMap
	// m_min_max_ufs: obj_map<func_decl, pair<app*, app*>>.
	minMaxUFs *ast.ObjMap
	// m_extra_assertions: expr_ref_vector.
	extraAssertions []ast.ASTRef
	// Virtual overrides (theory_fpa's fpa2bv_converter_wrapped).
	overrides FPA2BVOverrides
}

// FPA2BVOverrides are the virtual mk_const, mk_rm_const and mk_uf methods
// a derived converter replaces. Nil members keep the base behavior.
type FPA2BVOverrides struct {
	MkConst   func(c *FPA2BVConverter, f ast.ASTHandle, result ast.ASTRef) error
	MkRMConst func(c *FPA2BVConverter, f ast.ASTHandle, result ast.ASTRef) error
	MkUF      func(c *FPA2BVConverter, f ast.ASTHandle, args []ast.ASTHandle, result ast.ASTRef) error
}

type fpaFailure struct{ err error }

// NewFPA2BVConverter is fpa2bv_converter(ast_manager & m); simp is m_simp,
// which the converter owns and closes.
func NewFPA2BVConverter(m *ast.ASTManager, simp BoolSimplifier) (*FPA2BVConverter, error) {
	if m == nil || simp == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(FPA2BVConverter{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, errors.Join(err, simp.Close())
	}
	c := &FPA2BVConverter{m: m, simp: simp, header: n, hiFPUnspecified: true,
		const2bv: ast.NewObjMap(m), rmConst2bv: ast.NewObjMap(m),
		uf2bvuf: ast.NewObjMap(m), minMaxUFs: ast.NewObjMap(m)}
	c.self = c
	var err error
	if _, err = m.RegisterArithmeticPlugin(); err == nil {
		if c.bvFid, err = m.RegisterBitVectorPlugin(); err == nil {
			c.fid, err = m.RegisterFloatingPointPlugin()
		}
	}
	if err != nil {
		return nil, errors.Join(err, c.Close())
	}
	return c, nil
}

// SetOverrides installs a derived converter's virtual methods.
func (c *FPA2BVConverter) SetOverrides(o FPA2BVOverrides) { c.overrides = o }

// SetUnspecifiedFPHi is set_unspecified_fp_hi.
func (c *FPA2BVConverter) SetUnspecifiedFPHi(v bool) { c.hiFPUnspecified = v }

// Manager returns the converter's ast_manager.
func (c *FPA2BVConverter) Manager() *ast.ASTManager { return c.m }

func (c *FPA2BVConverter) open() error {
	if c == nil || c.self != c || c.closed {
		return ast.ErrASTHandle
	}
	return nil
}

// guard recovers an fpaFailure into *err at an exported entry point.
func (c *FPA2BVConverter) guard(err *error) {
	if r := recover(); r != nil {
		f, ok := r.(fpaFailure)
		if !ok {
			panic(r)
		}
		*err = errors.Join(*err, f.err)
	}
}

func (c *FPA2BVConverter) chk(err error) {
	if err != nil {
		panic(fpaFailure{err})
	}
}

func (c *FPA2BVConverter) must(h ast.ASTHandle, err error) ast.ASTHandle {
	c.chk(err)
	return h
}

func (c *FPA2BVConverter) mustU(v uint32, err error) uint32 {
	c.chk(err)
	return v
}

// ---- expr_ref frames ----

// eref is one expr_ref local.
type eref struct {
	c *FPA2BVConverter
	r ast.ASTRef
}

func (e *eref) set(h ast.ASTHandle) { e.c.chk(e.c.m.SetRef(e.r, h)) }
func (e *eref) get() ast.ASTHandle  { return e.c.must(e.c.m.RefGet(e.r)) }

// frame owns a function's expr_ref locals.
type frame struct {
	c    *FPA2BVConverter
	refs []*eref
}

func (c *FPA2BVConverter) frame() *frame { return &frame{c: c} }

// ref declares one expr_ref local.
func (f *frame) ref() *eref {
	r, err := f.c.m.NewRef(ast.ASTHandle{})
	f.c.chk(err)
	e := &eref{c: f.c, r: r}
	f.refs = append(f.refs, e)
	return e
}

// refs declares n expr_ref locals in order.
func (f *frame) refsN(n int) []*eref {
	out := make([]*eref, n)
	for i := range out {
		out[i] = f.ref()
	}
	return out
}

// done destroys the locals in reverse declaration order.
func (f *frame) done() {
	var err error
	for i := len(f.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, f.c.m.DelRef(&f.refs[i].r))
	}
	f.refs = nil
	if err != nil {
		if r := recover(); r != nil {
			panic(r)
		}
		panic(fpaFailure{err})
	}
}

// ---- m_simp ----

func (c *FPA2BVConverter) simpAnd(out *eref, args ...ast.ASTHandle) {
	c.chk(c.simp.MkAnd(args, out.r))
}
func (c *FPA2BVConverter) simpOr(out *eref, args ...ast.ASTHandle) {
	c.chk(c.simp.MkOr(args, out.r))
}
func (c *FPA2BVConverter) simpNot(a ast.ASTHandle, out *eref) { c.chk(c.simp.MkNot(a, out.r)) }
func (c *FPA2BVConverter) simpIte(cond, t, e ast.ASTHandle, out *eref) {
	c.chk(c.simp.MkIte(cond, t, e, out.r))
}
func (c *FPA2BVConverter) simpEq(a, b ast.ASTHandle, out *eref)  { c.chk(c.simp.MkEq(a, b, out.r)) }
func (c *FPA2BVConverter) simpXor(a, b ast.ASTHandle, out *eref) { c.chk(c.simp.MkXor(a, b, out.r)) }

// ---- ast_manager raw constructors ----

func (c *FPA2BVConverter) mEq(a, b ast.ASTHandle) ast.ASTHandle { return c.must(c.m.MkEq(a, b)) }
func (c *FPA2BVConverter) mIte(cond, t, e ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkIte(cond, t, e))
}
func (c *FPA2BVConverter) mNot(a ast.ASTHandle) ast.ASTHandle { return c.must(c.m.MkNot(a)) }
func (c *FPA2BVConverter) mAnd(args ...ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkAndArgs(args))
}
func (c *FPA2BVConverter) mOr(args ...ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkOrArgs(args))
}
func (c *FPA2BVConverter) mTrue() ast.ASTHandle  { return c.must(c.m.MkTrue()) }
func (c *FPA2BVConverter) mFalse() ast.ASTHandle { return c.must(c.m.MkFalse()) }
func (c *FPA2BVConverter) arg(e ast.ASTHandle, i uint32) ast.ASTHandle {
	return c.must(c.m.GetArg(e, i))
}

// ---- bv_util ----

func (c *FPA2BVConverter) bvSize(e ast.ASTHandle) uint32 { return c.mustU(c.m.BitVectorSize(e)) }
func (c *FPA2BVConverter) bvSort(sz uint32) ast.ASTHandle {
	return c.must(c.m.BitVectorSort(sz))
}

// bvNum is m_bv_util.mk_numeral(rational(v), sz).
func (c *FPA2BVConverter) bvNum(v int64, sz uint32) ast.ASTHandle {
	return c.must(c.m.BitVectorNumeralRat(new(big.Rat).SetInt64(v), sz))
}
func (c *FPA2BVConverter) bvNumBig(v *big.Int, sz uint32) ast.ASTHandle {
	return c.must(c.m.BitVectorNumeralRat(new(big.Rat).SetInt(v), sz))
}
func (c *FPA2BVConverter) bvApp(op int32, args ...ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkFamilyApp(c.bvFid, op, nil, args))
}
func (c *FPA2BVConverter) extract(high, low uint32, e ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorExtract(high, low, e))
}
func (c *FPA2BVConverter) concat(args ...ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorConcat(args))
}
func (c *FPA2BVConverter) zeroExtend(n uint32, e ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorZeroExtend(n, e))
}
func (c *FPA2BVConverter) signExtend(n uint32, e ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorSignExtend(n, e))
}
func (c *FPA2BVConverter) bvAdd(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorAdd(a, b))
}
func (c *FPA2BVConverter) bvSub(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorSub(a, b))
}
func (c *FPA2BVConverter) bvMul(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorMul(a, b))
}
func (c *FPA2BVConverter) bvNeg(a ast.ASTHandle) ast.ASTHandle { return c.must(c.m.MkBitVectorNeg(a)) }
func (c *FPA2BVConverter) bvNot(a ast.ASTHandle) ast.ASTHandle { return c.must(c.m.MkBitVectorNot(a)) }
func (c *FPA2BVConverter) bvOr(args ...ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorOr(args))
}
func (c *FPA2BVConverter) bvAnd(args ...ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorAnd(args))
}
func (c *FPA2BVConverter) bvXor(args ...ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorXor(args))
}
func (c *FPA2BVConverter) bvShl(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorSHL(a, b))
}
func (c *FPA2BVConverter) bvLshr(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorLSHR(a, b))
}
func (c *FPA2BVConverter) bvAshr(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorASHR(a, b))
}
func (c *FPA2BVConverter) bvUrem(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorURem(a, b))
}
func (c *FPA2BVConverter) ule(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorULE(a, b))
}
func (c *FPA2BVConverter) sle(a, b ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorSLE(a, b))
}
func (c *FPA2BVConverter) redor(a ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorReductionOr(a))
}
func (c *FPA2BVConverter) redand(a ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkBitVectorReductionAnd(a))
}

// m_mpf_manager.m_powers2.m1(n, negated): 2^n - 1, negated on request.
func powers2m1(n uint32, negated bool) *big.Int {
	v := new(big.Int).Lsh(big.NewInt(1), uint(n))
	v.Sub(v, big.NewInt(1))
	if negated {
		v.Neg(v)
	}
	return v
}

// ---- fpa_util ----

// mkFP is m_util.mk_fp(sgn, exp, sig).
func (c *FPA2BVConverter) mkFP(sgn, exp, sig ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkFamilyApp(c.fid, ast.FPAOpFP, nil, []ast.ASTHandle{sgn, exp, sig}))
}

// mkBV2RM is m_util.mk_bv2rm(e).
func (c *FPA2BVConverter) mkBV2RM(e ast.ASTHandle) ast.ASTHandle {
	return c.must(c.m.MkFamilyApp(c.fid, ast.FPAOpBV2RM, nil, []ast.ASTHandle{e}))
}

func (c *FPA2BVConverter) isAppOf(e ast.ASTHandle, fid ast.FamilyID, op int32) bool {
	k := c.must2kind(e)
	if k != ast.ASTApp {
		return false
	}
	f, err := c.m.GetFamilyID(e)
	c.chk(err)
	if f != fid {
		return false
	}
	o, err := c.m.GetDeclKind(e)
	c.chk(err)
	return o == op
}

func (c *FPA2BVConverter) must2kind(e ast.ASTHandle) ast.ASTKind {
	k, err := c.m.GetKind(e)
	c.chk(err)
	return k
}

// isFP is m_util.is_fp(e).
func (c *FPA2BVConverter) isFP(e ast.ASTHandle) bool { return c.isAppOf(e, c.fid, ast.FPAOpFP) }

// isBV2RM is m_util.is_bv2rm(e).
func (c *FPA2BVConverter) isBV2RM(e ast.ASTHandle) bool {
	return c.isAppOf(e, c.fid, ast.FPAOpBV2RM)
}

// sortShape returns (ebits, sbits) of a floating-point sort.
func (c *FPA2BVConverter) sortShape(s ast.ASTHandle) (uint32, uint32) {
	e, sb, ok, err := c.m.FloatingPointSortShape(s)
	c.chk(err)
	if !ok {
		c.chk(ast.ErrASTDomain)
	}
	return e, sb
}

func (c *FPA2BVConverter) ebits(s ast.ASTHandle) uint32 { e, _ := c.sortShape(s); return e }
func (c *FPA2BVConverter) sbits(s ast.ASTHandle) uint32 { _, sb := c.sortShape(s); return sb }

// isFloatSort is m_util.is_float(sort).
func (c *FPA2BVConverter) isFloatSort(s ast.ASTHandle) bool {
	_, _, ok, err := c.m.FloatingPointSortShape(s)
	c.chk(err)
	return ok
}

// isRMSort is m_util.is_rm(sort).
func (c *FPA2BVConverter) isRMSort(s ast.ASTHandle) bool {
	ok, err := c.m.IsRoundingModeSort(s)
	c.chk(err)
	return ok
}

// isFloat is fpa2bv_converter::is_float(expr*): an app whose range is a
// floating-point sort.
func (c *FPA2BVConverter) isFloat(e ast.ASTHandle) bool {
	if c.must2kind(e) != ast.ASTApp {
		return false
	}
	return c.isFloatSort(c.must(c.m.GetSort(e)))
}

// isRM is fpa2bv_converter::is_rm(expr*).
func (c *FPA2BVConverter) isRM(e ast.ASTHandle) bool {
	if c.must2kind(e) != ast.ASTApp {
		return false
	}
	return c.isRMSort(c.must(c.m.GetSort(e)))
}

func (c *FPA2BVConverter) rangeOf(f ast.ASTHandle) ast.ASTHandle { return c.must(c.m.GetRange(f)) }
func (c *FPA2BVConverter) sortOf(e ast.ASTHandle) ast.ASTHandle  { return c.must(c.m.GetSort(e)) }

// ---- reset / mk_bv_uf ----

// Reset is reset(): dec_ref_map_key_values on m_const2bv, m_rm_const2bv and
// m_uf2bvuf, then m_min_max_ufs, then m_extra_assertions.reset().
func (c *FPA2BVConverter) Reset() (err error) {
	if err = c.open(); err != nil {
		return err
	}
	defer c.guard(&err)
	c.reset()
	return nil
}

func (c *FPA2BVConverter) reset() {
	for _, table := range []*ast.ObjMap{c.const2bv, c.rmConst2bv, c.uf2bvuf} {
		// dec_ref_map_key_values: each key, then its value, in table order.
		for _, e := range table.Entries() {
			c.chk(c.m.DecRef(e.Key))
			c.chk(c.m.DecRef(e.Value[0]))
		}
		table.Reset()
	}
	for _, e := range c.minMaxUFs.Entries() {
		c.chk(c.m.DecRef(e.Key))
		c.chk(c.m.DecRef(e.Value[0]))
		c.chk(c.m.DecRef(e.Value[1]))
	}
	c.minMaxUFs.Reset()
	c.resetExtraAssertions()
}

func (c *FPA2BVConverter) resetExtraAssertions() {
	var err error
	for i := range c.extraAssertions {
		err = errors.Join(err, c.m.DelRef(&c.extraAssertions[i]))
	}
	c.extraAssertions = nil
	c.chk(err)
}

// pushExtraAssertion is m_extra_assertions.push_back(e).
func (c *FPA2BVConverter) pushExtraAssertion(e ast.ASTHandle) {
	r, err := c.m.NewRef(e)
	c.chk(err)
	c.extraAssertions = append(c.extraAssertions, r)
}

// ExtraAssertions returns m_extra_assertions (borrowed).
func (c *FPA2BVConverter) ExtraAssertions() ([]ast.ASTHandle, error) {
	out := make([]ast.ASTHandle, len(c.extraAssertions))
	for i, r := range c.extraAssertions {
		h, err := c.m.RefGet(r)
		if err != nil {
			return nil, err
		}
		out[i] = h
	}
	return out, nil
}

// ResetExtraAssertions is m_extra_assertions.reset().
func (c *FPA2BVConverter) ResetExtraAssertions() (err error) {
	defer c.guard(&err)
	c.resetExtraAssertions()
	return nil
}

// mkBVUF is mk_bv_uf.
func (c *FPA2BVConverter) mkBVUF(f ast.ASTHandle, domain []ast.ASTHandle, rng ast.ASTHandle) ast.ASTHandle {
	if v, ok, err := c.uf2bvuf.Find(f); err != nil {
		c.chk(err)
	} else if ok {
		return v[0]
	}
	res := c.must(c.m.MkFreshFuncDecl(usymbol.Symbol{}, usymbol.Symbol{}, domain, rng))
	c.chk(c.uf2bvuf.Insert(f, [2]ast.ASTHandle{res}))
	c.chk(c.m.IncRef(f))
	c.chk(c.m.IncRef(res))
	return res
}

// Close is ~fpa2bv_converter: reset().
func (c *FPA2BVConverter) Close() (err error) {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	defer c.guard(&err)
	c.reset()
	c.chk(c.simp.Close())
	c.m.Budget().Release(c.header)
	c.closed = true
	return nil
}

// ConverterSnapshot holds borrowed entries of the converter's
// model-conversion maps, in their obj_map table order.
type ConverterSnapshot struct {
	Constants         []ast.ObjMapEntry
	RoundingConstants []ast.ObjMapEntry
	ConvertedUFs      []ast.ObjMapEntry
	MinMaxSigns       []ast.ObjMapEntry
}
