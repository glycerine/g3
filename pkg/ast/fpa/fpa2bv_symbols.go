package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:175-305,4105-4129.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ConverterSnapshot lists the model-conversion maps' entries in obj_map
// iteration order, as bv2fpa_converter's constructor walks them.
type ConverterSnapshot struct {
	Constants         []ast.ObjMapEntry
	RoundingConstants []ast.ObjMapEntry
	ConvertedUFs      []ast.ObjMapEntry
	MinMaxSigns       []ast.ObjMapEntry
}

// Snapshot returns borrowed entries of the model-conversion maps accumulated
// by this converter. A model converter that outlives Converter must retain them.
func (c *Converter) Snapshot() (ConverterSnapshot, error) {
	if err := c.open(); err != nil {
		return ConverterSnapshot{}, err
	}
	return ConverterSnapshot{
		Constants:         c.constants.Entries(),
		RoundingConstants: c.roundingConstants.Entries(),
		ConvertedUFs:      c.convertedUFs.Entries(),
		MinMaxSigns:       c.minMaxSigns.Entries(),
	}, nil
}

func (c *Converter) cacheInsert(cache *ast.ObjMap, declaration, value ast.ASTHandle) error {
	if err := c.manager.IncRef(declaration); err != nil {
		return err
	}
	if err := c.manager.IncRef(value); err != nil {
		_ = c.manager.DecRef(declaration)
		return err
	}
	if err := cache.Insert(declaration, [2]ast.ASTHandle{value}); err != nil {
		return errors.Join(err, c.manager.DecRef(value), c.manager.DecRef(declaration))
	}
	return nil
}

func (c *Converter) addAssertion(value ast.ASTHandle) error {
	if err := c.manager.IncRef(value); err != nil {
		return err
	}
	c.extraAssertions = append(c.extraAssertions, value)
	return nil
}

// ExtraAssertions returns a borrowed view of side constraints introduced by
// symbolic rounding-mode constants and converted uninterpreted functions.
func (c *Converter) ExtraAssertions() []ast.ASTHandle {
	if c == nil || c.self != c || c.closed {
		return nil
	}
	return c.extraAssertions
}

// FlushExtraAssertions transfers the converter's owned references to the
// caller. The caller must DecRef every returned assertion.
func (c *Converter) FlushExtraAssertions() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	result := c.extraAssertions
	c.extraAssertions = nil
	return result, nil
}

// releaseCache is dec_ref_map_key_values: key then value, in table order,
// and then map.reset().
func (c *Converter) releaseCache(cache *ast.ObjMap) (result error) {
	for _, entry := range cache.Entries() {
		result = errors.Join(result, c.manager.DecRef(entry.Key), c.manager.DecRef(entry.Value[0]))
	}
	cache.Reset()
	return result
}

// Reset releases every persistent converted declaration, value and assertion.
func (c *Converter) Reset() (result error) {
	if c == nil || c.self != c || c.manager == nil || c.closed {
		return nil
	}
	result = errors.Join(result, c.releaseCache(c.constants), c.releaseCache(c.roundingConstants), c.releaseCache(c.convertedUFs))
	for _, entry := range c.minMaxSigns.Entries() {
		result = errors.Join(result, c.manager.DecRef(entry.Key), c.manager.DecRef(entry.Value[0]), c.manager.DecRef(entry.Value[1]))
	}
	c.minMaxSigns.Reset()
	for _, assertion := range c.extraAssertions {
		result = errors.Join(result, c.manager.DecRef(assertion))
	}
	c.extraAssertions = nil
	return result
}

// MkConst replaces a null-family FP constant by a stable packed-BV-backed FP.
func (c *Converter) MkConst(declaration ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if value, found, err := c.constants.Find(declaration); err != nil {
		return ast.ASTHandle{}, err
	} else if found {
		return value[0], nil
	}
	arity, err := c.manager.GetArity(declaration)
	if err != nil || arity != 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	sort, err := c.manager.GetRange(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, matched, err := c.manager.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if c.constantFactory != nil {
		result, factoryErr := c.constantFactory(declaration)
		if factoryErr != nil {
			return ast.ASTHandle{}, factoryErr
		}
		resultSort, factoryErr := c.manager.GetSort(result)
		if factoryErr != nil || resultSort != sort {
			if factoryErr != nil {
				return ast.ASTHandle{}, factoryErr
			}
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if factoryErr = c.cacheInsert(c.constants, declaration, result); factoryErr != nil {
			return ast.ASTHandle{}, factoryErr
		}
		return result, nil
	}
	var sign, exponent, significand ast.ASTHandle
	if ast.ASTSourceDebug {
		name, err := c.manager.GetASTName(declaration)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		text, err := c.manager.Symbols.Display(name)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		signSort, err := c.manager.BitVectorSort(1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		expSort, err := c.manager.BitVectorSort(ebits)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		sigSort, err := c.manager.BitVectorSort(sbits - 1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		sign, err = c.manager.MkFreshConst("fpa2bv_sgn_"+text, signSort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		exponent, err = c.manager.MkFreshConst("fpa2bv_exp_"+text, expSort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		significand, err = c.manager.MkFreshConst("fpa2bv_sig_"+text, sigSort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	} else {
		width := ebits + sbits
		packedSort, err := c.manager.BitVectorSort(width)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		packed, err := c.manager.MkFreshConstNullable(nil, packedSort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		sign, err = c.manager.MkBitVectorExtract(width-1, width-1, packed)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		exponent, err = c.manager.MkBitVectorExtract(width-2, sbits-1, packed)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		significand, err = c.manager.MkBitVectorExtract(sbits-2, 0, packed)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	result, err := c.MkFP(sign, exponent, significand)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.cacheInsert(c.constants, declaration, result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// MkRMConst replaces a symbolic rounding-mode constant with a stable BV3 and
// records the source range constraint bv <= 4.
func (c *Converter) MkRMConst(declaration ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if value, found, err := c.roundingConstants.Find(declaration); err != nil {
		return ast.ASTHandle{}, err
	} else if found {
		return value[0], nil
	}
	arity, err := c.manager.GetArity(declaration)
	if err != nil || arity != 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	sort, err := c.manager.GetRange(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	matched, err := c.manager.IsRoundingModeSort(sort)
	if err != nil || !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if c.roundingFactory != nil {
		result, factoryErr := c.roundingFactory(declaration)
		if factoryErr != nil {
			return ast.ASTHandle{}, factoryErr
		}
		resultSort, factoryErr := c.manager.GetSort(result)
		if factoryErr != nil || resultSort != sort {
			if factoryErr != nil {
				return ast.ASTHandle{}, factoryErr
			}
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if factoryErr = c.cacheInsert(c.roundingConstants, declaration, result); factoryErr != nil {
			return ast.ASTHandle{}, factoryErr
		}
		return result, nil
	}
	bvSort, err := c.manager.BitVectorSort(3)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bits, err := c.manager.MkFreshConst("fpa2bv_rm", bvSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := c.wrapRoundingMode(bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	limit, err := c.manager.BitVectorNumeralUnsigned(4, 3)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	constraint, err := c.manager.MkBitVectorULE(bits, limit)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.cacheInsert(c.roundingConstants, declaration, result); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.addAssertion(constraint); err != nil {
		_ = c.roundingConstants.Erase(declaration)
		return ast.ASTHandle{}, errors.Join(err, c.manager.DecRef(declaration), c.manager.DecRef(result))
	}
	return result, nil
}

func (c *Converter) convertedUF(declaration ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	arity, err := c.manager.GetArity(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		domain[i], err = c.manager.GetDomain(declaration, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return c.convertedUFWithDomain(declaration, domain, rangeSort)
}

func (c *Converter) convertedUFWithDomain(declaration ast.ASTHandle, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	if value, found, err := c.convertedUFs.Find(declaration); err != nil {
		return ast.ASTHandle{}, err
	} else if found {
		return value[0], nil
	}
	result, err := c.manager.MkFreshFuncDecl(usymbol.Symbol{}, usymbol.Symbol{}, domain, rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.cacheInsert(c.convertedUFs, declaration, result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// MkUF converts an uninterpreted application whose result is FP or RM and
// records the equality connecting the original and converted applications.
func (c *Converter) MkUF(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	original, err := c.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rangeSort, err := c.manager.GetRange(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rounding, err := c.manager.IsRoundingModeSort(rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !floating && !rounding {
		return original, nil
	}
	width := uint32(3)
	if floating {
		width = ebits + sbits
	}
	bvSort, err := c.manager.BitVectorSort(width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	convertedDecl, err := c.convertedUF(declaration, bvSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bits, err := c.manager.MkApp(convertedDecl, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var result ast.ASTHandle
	if floating {
		result, err = c.FromIEEEBV(rangeSort, bits)
	} else {
		result, err = c.wrapRoundingMode(bits)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equality, err := c.manager.MkEq(original, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.addAssertion(equality); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}
