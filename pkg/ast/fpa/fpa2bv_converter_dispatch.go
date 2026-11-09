package fpa

// Source: src/ast/fpa/fpa2bv_rewriter.cpp:57-170 (fpa2bv_rewriter_cfg::
// reduce_app), which dispatches each application to the converter.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// ReduceApp is fpa2bv_rewriter_cfg::reduce_app: done reports BR_DONE, and
// false is BR_FAILED (result untouched).
func (c *FPA2BVConverter) ReduceApp(f ast.ASTHandle, args []ast.ASTHandle, result ast.ASTRef) (done bool, err error) {
	if err = c.open(); err != nil {
		return false, err
	}
	defer c.guard(&err)
	return c.reduceApp(f, args, &eref{c: c, r: result}), nil
}

func (c *FPA2BVConverter) reduceApp(f ast.ASTHandle, args []ast.ASTHandle, result *eref) bool {
	m := c.m
	family, err := m.GetFamilyID(f)
	c.chk(err)
	kind, err := m.GetDeclKind(f)
	c.chk(err)
	num := len(args)
	if num == 0 && family == ast.NullFamilyID && c.isFloatSort(c.rangeOf(f)) {
		c.mkConst(f, result)
		return true
	}
	if num == 0 && family == ast.NullFamilyID && c.isRMSort(c.rangeOf(f)) {
		c.mkRMConst(f, result)
		return true
	}
	basic := family == m.BasicFamilyID()
	if basic && kind == ast.ASTOpEq {
		ds := c.must(m.GetDomain(f, 0))
		if c.isFloatSort(ds) {
			c.mkEq(args[0], args[1], result)
			return true
		} else if c.isRMSort(ds) {
			result.set(c.mEq(args[0], args[1]))
			return true
		}
		return false
	} else if basic && kind == ast.ASTOpIte {
		if c.isFloat(args[1]) || c.isRM(args[1]) {
			c.mkIte(args[0], args[1], args[2], result)
			return true
		}
		return false
	} else if basic && kind == ast.ASTOpDistinct {
		ds := c.must(m.GetDomain(f, 0))
		if c.isFloatSort(ds) || c.isRMSort(ds) {
			c.mkDistinct(args, result)
			return true
		}
		return false
	}
	if family == c.fid {
		switch kind {
		case ast.FPAOpRMNearestAway, ast.FPAOpRMNearestEven, ast.FPAOpRMTowardNegative,
			ast.FPAOpRMTowardPositive, ast.FPAOpRMTowardZero:
			c.mkRoundingMode(kind, result)
		case ast.FPAOpNumeral:
			c.mkNumeral(c.must(m.MkApp(f, nil)), result)
		case ast.FPAOpPositiveInf:
			c.mkPinfSort(c.rangeOf(f), result)
		case ast.FPAOpNegativeInf:
			c.mkNinfSort(c.rangeOf(f), result)
		case ast.FPAOpPositiveZero:
			c.mkPzeroSort(c.rangeOf(f), result)
		case ast.FPAOpNegativeZero:
			c.mkNzeroSort(c.rangeOf(f), result)
		case ast.FPAOpNaN:
			c.mkNanSort(c.rangeOf(f), result)
		case ast.FPAOpAdd:
			c.mkAddDecl(f, args, result)
		case ast.FPAOpSub:
			c.mkSubDecl(f, args, result)
		case ast.FPAOpNeg:
			c.mkNegDecl(f, args, result)
		case ast.FPAOpMul:
			c.mkMulDecl(f, args, result)
		case ast.FPAOpDiv:
			c.mkDivDecl(f, args, result)
		case ast.FPAOpRem:
			c.mkRemDecl(f, args, result)
		case ast.FPAOpAbs:
			c.mkAbsDecl(f, args, result)
		case ast.FPAOpMin:
			c.mkMin(f, args, result)
		case ast.FPAOpMax:
			c.mkMax(f, args, result)
		case ast.FPAOpFMA:
			c.mkFMA(f, args, result)
		case ast.FPAOpSqrt:
			c.mkSqrt(f, args, result)
		case ast.FPAOpRoundToIntegral:
			c.mkRoundToIntegralDecl(f, args, result)
		case ast.FPAOpEq:
			c.mkFloatEqDecl(f, args, result)
		case ast.FPAOpLt:
			c.mkFloatLtDecl(f, args, result)
		case ast.FPAOpGt:
			c.mkFloatGtDecl(f, args, result)
		case ast.FPAOpLe:
			c.mkFloatLeDecl(f, args, result)
		case ast.FPAOpGe:
			c.mkFloatGeDecl(f, args, result)
		case int32(ast.FPAIsZero):
			c.mkIsZeroDecl(args, result)
		case int32(ast.FPAIsNaN):
			c.mkIsNanDecl(args, result)
		case int32(ast.FPAIsInfinite):
			c.mkIsInfDecl(args, result)
		case int32(ast.FPAIsNormal):
			c.mkIsNormalDecl(args, result)
		case int32(ast.FPAIsSubnormal):
			c.mkIsSubnormalDecl(args, result)
		case int32(ast.FPAIsPositive):
			c.mkIsPositiveDecl(args, result)
		case int32(ast.FPAIsNegative):
			c.mkIsNegativeDecl(args, result)
		case ast.FPAOpToFP:
			c.mkToFP(f, args, result)
		case ast.FPAOpToFPUnsigned:
			c.mkToFPUnsigned(f, args, result)
		case ast.FPAOpFP:
			c.mkFPDecl(args, result)
		case ast.FPAOpToUBV:
			c.mkToUBV(f, args, result)
		case ast.FPAOpToSBV:
			c.mkToSBV(f, args, result)
		case ast.FPAOpToReal:
			c.mkToReal(f, args, result)
		case ast.FPAOpToIEEEBV:
			c.mkToIEEEBV(f, args, result)
		case ast.FPAOpBVWrap, ast.FPAOpBV2RM:
			return false
		default:
			c.chk(ast.ErrASTDomain) // NOT_IMPLEMENTED_YET
		}
		return true
	}
	contains, err := m.ContainsFloatingPoint(f)
	c.chk(err)
	if contains {
		c.mkUF(f, args, result)
		return true
	}
	return false
}

// MkFP is fpa_util::mk_fp(sgn, exp, sig).
func (c *FPA2BVConverter) MkFP(sgn, exp, sig ast.ASTHandle) (h ast.ASTHandle, err error) {
	defer c.guard(&err)
	return c.mkFP(sgn, exp, sig), nil
}

// MkEq is mk_eq(a, b, result), used by theory_fpa.
func (c *FPA2BVConverter) MkEq(a, b ast.ASTHandle, result ast.ASTRef) (err error) {
	defer c.guard(&err)
	c.mkEq(a, b, &eref{c: c, r: result})
	return nil
}

// IsFloatSort is fpa_util::is_float(sort).
func (c *FPA2BVConverter) IsFloatSort(s ast.ASTHandle) (ok bool, err error) {
	defer c.guard(&err)
	return c.isFloatSort(s), nil
}

// Snapshot returns borrowed entries of m_const2bv, m_rm_const2bv,
// m_uf2bvuf and m_min_max_ufs in their table order.
func (c *FPA2BVConverter) Snapshot() (ConverterSnapshot, error) {
	if err := c.open(); err != nil {
		return ConverterSnapshot{}, err
	}
	return ConverterSnapshot{
		Constants:         c.const2bv.Entries(),
		RoundingConstants: c.rmConst2bv.Entries(),
		ConvertedUFs:      c.uf2bvuf.Entries(),
		MinMaxSigns:       c.minMaxUFs.Entries(),
	}, nil
}

// FlushExtraAssertions transfers m_extra_assertions to the caller, which
// must DecRef every returned assertion, and resets the vector.
func (c *FPA2BVConverter) FlushExtraAssertions() (out []ast.ASTHandle, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	defer c.guard(&err)
	for _, r := range c.extraAssertions {
		h := c.must(c.m.RefGet(r))
		c.chk(c.m.IncRef(h))
		out = append(out, h)
	}
	c.resetExtraAssertions()
	return out, nil
}

// Const2BVFind is m_const2bv.find(f, r).
func (c *FPA2BVConverter) Const2BVFind(f ast.ASTHandle) (ast.ASTHandle, bool, error) {
	v, ok, err := c.const2bv.Find(f)
	return v[0], ok, err
}

// Const2BVInsert is m_const2bv.insert(f, r); m.inc_ref(f); m.inc_ref(r).
func (c *FPA2BVConverter) Const2BVInsert(f, r ast.ASTHandle) error {
	if err := c.const2bv.Insert(f, [2]ast.ASTHandle{r}); err != nil {
		return err
	}
	if err := c.m.IncRef(f); err != nil {
		return err
	}
	return c.m.IncRef(r)
}

// RMConst2BVFind is m_rm_const2bv.find(f, r).
func (c *FPA2BVConverter) RMConst2BVFind(f ast.ASTHandle) (ast.ASTHandle, bool, error) {
	v, ok, err := c.rmConst2bv.Find(f)
	return v[0], ok, err
}

// RMConst2BVInsert is m_rm_const2bv.insert(f, r); m.inc_ref(f); m.inc_ref(r).
func (c *FPA2BVConverter) RMConst2BVInsert(f, r ast.ASTHandle) error {
	if err := c.rmConst2bv.Insert(f, [2]ast.ASTHandle{r}); err != nil {
		return err
	}
	if err := c.m.IncRef(f); err != nil {
		return err
	}
	return c.m.IncRef(r)
}

// MkConst is mk_const(f, result); the result stays owned by m_const2bv.
func (c *FPA2BVConverter) MkConst(f ast.ASTHandle) (h ast.ASTHandle, err error) {
	if err = c.open(); err != nil {
		return h, err
	}
	defer c.guard(&err)
	g := c.frame()
	defer g.done()
	r := g.ref()
	c.mkConst(f, r)
	return r.get(), nil
}

// MkRMConst is mk_rm_const(f, result); the result stays owned by
// m_rm_const2bv.
func (c *FPA2BVConverter) MkRMConst(f ast.ASTHandle) (h ast.ASTHandle, err error) {
	if err = c.open(); err != nil {
		return h, err
	}
	defer c.guard(&err)
	g := c.frame()
	defer g.done()
	r := g.ref()
	c.mkRMConst(f, r)
	return r.get(), nil
}
