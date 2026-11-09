package fpa

// Source: src/ast/fpa/bv2fpa_converter.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	"github.com/glycerine/g3/pkg/model"
)

// Converter rebuilds floating-point interpretations from the bit-vector model
// produced by ast/fpa. It owns retained copies of the source converter maps.
type Converter struct {
	self    *Converter
	manager *ast.ASTManager
	// m_const2bv, m_rm_const2bv, m_uf2bvuf, m_specials: obj_maps.
	constants         *ast.ObjMap
	roundingConstants *ast.ObjMap
	convertedUFs      *ast.ObjMap
	minMaxSigns       *ast.ObjMap
	header            uint64
	closed            bool
}

func NewConverter(m *ast.ASTManager, source *astfpa.FPA2BVConverter) (owner *Converter, result error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	bytes := uint64(reflect.TypeOf(Converter{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	owner = &Converter{
		manager: m, header: bytes,
		constants: ast.NewObjMap(m), roundingConstants: ast.NewObjMap(m),
		convertedUFs: ast.NewObjMap(m), minMaxSigns: ast.NewObjMap(m),
	}
	owner.self = owner
	if source == nil {
		return owner, nil
	}
	snapshot, err := source.Snapshot()
	if err != nil {
		return owner, errors.Join(err, owner.Close())
	}
	retain := func(key, value ast.ASTHandle) error {
		if err := m.IncRef(key); err != nil {
			return err
		}
		if err := m.IncRef(value); err != nil {
			return errors.Join(err, m.DecRef(key))
		}
		return nil
	}
	for _, entry := range snapshot.Constants {
		key, value := entry.Key, entry.Value[0]
		if err = retain(key, value); err != nil {
			return owner, errors.Join(err, owner.Close())
		}
		if err := owner.constants.Insert(key, [2]ast.ASTHandle{value}); err != nil {
			return nil, err
		}
	}
	for _, entry := range snapshot.RoundingConstants {
		key, value := entry.Key, entry.Value[0]
		if err = retain(key, value); err != nil {
			return owner, errors.Join(err, owner.Close())
		}
		if err := owner.roundingConstants.Insert(key, [2]ast.ASTHandle{value}); err != nil {
			return nil, err
		}
	}
	for _, entry := range snapshot.ConvertedUFs {
		key, value := entry.Key, entry.Value[0]
		if err = retain(key, value); err != nil {
			return owner, errors.Join(err, owner.Close())
		}
		if err := owner.convertedUFs.Insert(key, [2]ast.ASTHandle{value}); err != nil {
			return nil, err
		}
	}
	for _, entry := range snapshot.MinMaxSigns {
		key, values := entry.Key, entry.Value
		if err = m.IncRef(key); err != nil {
			return owner, errors.Join(err, owner.Close())
		}
		if err = m.IncRef(values[0]); err != nil {
			return owner, errors.Join(err, m.DecRef(key), owner.Close())
		}
		if err = m.IncRef(values[1]); err != nil {
			return owner, errors.Join(err, m.DecRef(values[0]), m.DecRef(key), owner.Close())
		}
		if err := owner.minMaxSigns.Insert(key, values); err != nil {
			return nil, err
		}
	}
	return owner, nil
}

func (c *Converter) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil {
		return ast.ErrASTHandle
	}
	_, err := c.manager.ProofsEnabled()
	return err
}

func (c *Converter) Close() (result error) {
	if c == nil || c.self != c || c.closed {
		return nil
	}
	// ~bv2fpa_converter: dec_ref_map_key_values (key, then value) on each map
	// in table order, then the specials (key, first, second).
	for _, entry := range c.constants.Entries() {
		key, value := entry.Key, entry.Value[0]
		result = errors.Join(result, c.manager.DecRef(key), c.manager.DecRef(value))
	}
	for _, entry := range c.roundingConstants.Entries() {
		key, value := entry.Key, entry.Value[0]
		result = errors.Join(result, c.manager.DecRef(key), c.manager.DecRef(value))
	}
	for _, entry := range c.convertedUFs.Entries() {
		key, value := entry.Key, entry.Value[0]
		result = errors.Join(result, c.manager.DecRef(key), c.manager.DecRef(value))
	}
	for _, entry := range c.minMaxSigns.Entries() {
		key, values := entry.Key, entry.Value
		result = errors.Join(result, c.manager.DecRef(key), c.manager.DecRef(values[0]), c.manager.DecRef(values[1]))
	}
	c.constants.Reset()
	c.roundingConstants.Reset()
	c.convertedUFs.Reset()
	c.minMaxSigns.Reset()
	c.manager.Budget().Release(c.header)
	c.header = 0
	c.closed = true
	return result
}

// ConvertBV2FP rebuilds an FPA numeral from evaluated IEEE component fields.
func (c *Converter) ConvertBV2FP(sort, sign, exponent, significand ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	for _, field := range []struct {
		value ast.ASTHandle
		width uint32
	}{{sign, 1}, {exponent, ebits}, {significand, sbits - 1}} {
		if width, e := c.manager.BitVectorSize(field.value); e != nil || width != field.width {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	family, err := c.manager.RegisterFloatingPointPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// convert_bv2fp builds the mpf and m_fpa_util.mk_value(fp_val) directly;
	// it never creates an fp(sgn, exp, sig) application.
	var domain [3]ast.ASTHandle
	for i, component := range []ast.ASTHandle{sign, exponent, significand} {
		if domain[i], err = c.manager.GetSort(component); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	declaration, err := c.manager.MkFamilyFuncDecl(family, ast.FPAOpFP, nil, domain[:], sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, found, err := c.manager.RewriteFPAFPComponentsNumerals(declaration, sign, exponent, significand)
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// ConvertBV2RM maps the converter's three-bit rounding code back to an FPA
// rounding-mode numeral. Values outside 0..3 use toward-zero.
func (c *Converter) ConvertBV2RM(bits ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	service := c.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	var width uint32
	found, err := c.manager.BitVectorNumeralValue(bits, &value, &width)
	if err != nil || !found || width != 3 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	n, err := service.Uint64(&value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind := ast.FPAOpRMTowardZero
	switch n {
	case 0:
		kind = ast.FPAOpRMNearestEven
	case 1:
		kind = ast.FPAOpRMNearestAway
	case 2:
		kind = ast.FPAOpRMTowardPositive
	case 3:
		kind = ast.FPAOpRMTowardNegative
	}
	family, err := c.manager.RegisterFloatingPointPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.MkFamilyApp(family, kind, nil, nil)
}

// ConvertPackedBV evaluates a packed BV constant when necessary, splits its
// IEEE fields, and reconstructs an FPA numeral. Missing interpretations use 0.
func (c *Converter) ConvertPackedBV(mc *model.ModelCore, sort, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	width := ebits + sbits
	var probeWidth uint32
	service := c.manager.NumeralService()
	probe, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	found, probeErr := c.manager.BitVectorNumeralValue(value, &probe, &probeWidth)
	err = errors.Join(probeErr, service.Del(&probe))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !found {
		if mc == nil {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		declaration, e := c.manager.GetDecl(value)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		value, e = mc.ConstantInterpretation(declaration)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if value.IsNull() {
			value, e = c.manager.BitVectorNumeralUnsigned(0, width)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	}
	extract := func(high, low uint32) (node ast.ASTHandle, result error) {
		numerals, e := ast.NewBitVectorExtractNumerals(c.manager)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		defer func() { result = errors.Join(result, numerals.Close()) }()
		var sourceWidth uint32
		matched, e := numerals.Read(value, &sourceWidth)
		if e != nil || !matched || sourceWidth != width {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return ast.ASTHandle{}, e
		}
		node, e = numerals.Evaluate(high, low)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = c.manager.IncRef(node); e != nil {
			return ast.ASTHandle{}, e
		}
		if e = numerals.SourceEnd(); e != nil {
			return ast.ASTHandle{}, errors.Join(e, c.manager.DecRef(node))
		}
		return node, nil
	}
	// sgn, exp and sig are expr_ref extract applications that m_th_rw then
	// folds to v_sgn, v_exp and v_sig; all are released on return.
	for _, bounds := range [][2]uint32{{width - 1, width - 1}, {width - 2, sbits - 1}, {sbits - 2, 0}} {
		app, e := c.manager.MkBitVectorExtract(bounds[0], bounds[1], value)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = c.manager.IncRef(app); e != nil {
			return ast.ASTHandle{}, e
		}
		defer c.manager.DecRef(app)
	}
	sign, err := extract(width-1, width-1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer c.manager.DecRef(sign)
	exponent, err := extract(width-2, sbits-1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer c.manager.DecRef(exponent)
	significand, err := extract(sbits-2, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer c.manager.DecRef(significand)
	return c.ConvertBV2FP(sort, sign, exponent, significand)
}
