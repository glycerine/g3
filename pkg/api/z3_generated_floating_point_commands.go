package api

// Source: generated api_commands.cpp and src/api/api_fpa.cpp.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedFloatingPointCommands(r *Z3Replayer) error {
	names := []string{
		"rounding_mode_sort", "round_nearest_ties_to_even", "rne", "round_nearest_ties_to_away", "rna",
		"round_toward_positive", "rtp", "round_toward_negative", "rtn", "round_toward_zero", "rtz",
		"sort", "sort_half", "sort_16", "sort_single", "sort_32", "sort_double", "sort_64", "sort_quadruple", "sort_128",
		"nan", "inf", "zero", "fp", "numeral_float", "numeral_double", "numeral_int", "numeral_int_uint", "numeral_int64_uint64",
	}
	for i, name := range names {
		id := uint32(571 + i)
		if err := r.RegisterCommand(id, "Z3_mk_fpa_"+name, func(r *Z3Replayer) error { return replayFloatingPointConstructor(r, id) }); err != nil {
			return err
		}
	}
	return nil
}

func replayFloatingPointConstructor(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	m := owner.results.manager
	family, err := m.RegisterFloatingPointPlugin()
	if err != nil {
		return err
	}
	var value ast.ASTHandle
	switch {
	case id == 571:
		value, err = m.RoundingModeSort()
	case id >= 572 && id <= 581:
		modes := []int32{ast.FPAOpRMNearestEven, ast.FPAOpRMNearestAway, ast.FPAOpRMTowardPositive, ast.FPAOpRMTowardNegative, ast.FPAOpRMTowardZero}
		value, err = m.MkFamilyApp(family, modes[(id-572)/2], nil, nil)
	case id >= 582 && id <= 590:
		var ebits, sbits uint32
		if id == 582 {
			ebits, err = r.Uint(1)
			if err != nil {
				return err
			}
			sbits, err = r.Uint(2)
			if err != nil {
				return err
			}
		} else {
			shapes := [][2]uint32{{5, 11}, {8, 24}, {11, 53}, {15, 113}}
			shape := shapes[(id-583)/2]
			ebits, sbits = shape[0], shape[1]
		}
		value, err = m.FloatingPointSort(ebits, sbits)
	case id >= 591 && id <= 593:
		sort, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		if id == 591 {
			value, err = m.MkFPANaNOfSort(sort.handle)
		} else {
			negative, e := r.Bool(2)
			if e != nil {
				return e
			}
			if id == 592 {
				value, err = m.MkFPAInfinityOfSort(sort.handle, negative)
			} else {
				value, err = m.MkFPAZeroOfSort(sort.handle, negative)
			}
		}
	case id == 594:
		args := make([]ast.ASTHandle, 3)
		for i := range args {
			v, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			args[i] = v.handle
		}
		value, err = m.MkFamilyApp(family, ast.FPAOpFP, nil, args)
	case id >= 595 && id <= 597:
		sort, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		switch id {
		case 595:
			v, e := r.Float(1)
			if e != nil {
				return e
			}
			value, err = m.MkFPANumeralFloat32(v, sort.handle)
		case 596:
			v, e := r.Double(1)
			if e != nil {
				return e
			}
			value, err = m.MkFPANumeralFloat64(v, sort.handle)
		case 597:
			v, e := r.Int(1)
			if e != nil {
				return e
			}
			value, err = m.MkFPANumeralInt32(v, sort.handle)
		}
	case id == 598 || id == 599:
		sign, e := r.Bool(1)
		if e != nil {
			return e
		}
		var exponent int64
		var significand uint64
		if id == 598 {
			v, e := r.Int(2)
			if e != nil {
				return e
			}
			exponent = int64(v)
			n, e := r.Uint(3)
			if e != nil {
				return e
			}
			significand = uint64(n)
		} else {
			exponent, e = r.Int64(2)
			if e != nil {
				return e
			}
			significand, e = r.Uint64(3)
			if e != nil {
				return e
			}
		}
		sort, e := r.astObject(owner, 4)
		if e != nil {
			return e
		}
		value, err = m.MkFPANumeralComponents(sign, exponent, significand, sort.handle)
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}
