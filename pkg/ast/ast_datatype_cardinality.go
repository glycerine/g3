package ast

// Source: src/ast/datatype_decl_plugin.h:104-190 (param_size);
// src/ast/datatype_decl_plugin.cpp:123-147 (param_size factories),
// :193-235 (plugin::mk_sort size evaluation), :579-673
// (util::get_sort_size, util::compute_datatype_size_functions).

import (
	"math"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func datatypeSizeAdd(left, right SortSize) SortSize {
	if left.IsInfinite() {
		return left
	}
	if right.IsInfinite() {
		return right
	}
	if left.IsVeryBig() {
		return left
	}
	if right.IsVeryBig() {
		return right
	}
	a, _ := left.Size()
	b, _ := right.Size()
	if math.MaxUint64-a < b {
		return VeryBigSortSize()
	}
	return FiniteSortSize(a + b)
}

func datatypeSizeMultiply(left, right SortSize) SortSize {
	if left.IsInfinite() {
		return left
	}
	if right.IsInfinite() {
		return right
	}
	if left.IsVeryBig() {
		return left
	}
	if right.IsVeryBig() {
		return right
	}
	a, _ := left.Size()
	b, _ := right.Size()
	if a != 0 && b > math.MaxUint64/a {
		return VeryBigSortSize()
	}
	return FiniteSortSize(a * b)
}

// datatypeSizePower is param_size::power::eval: s1^s2.
func datatypeSizePower(s1, s2 SortSize) SortSize {
	if s1.IsInfinite() {
		return s1
	}
	if s2.IsInfinite() {
		return s2
	}
	if s1.IsVeryBig() {
		return s1
	}
	if s2.IsVeryBig() {
		return s2
	}
	base, _ := s1.Size()
	exponent, _ := s2.Size()
	if base == 1 {
		return s1
	}
	if exponent == 1 {
		return s1
	}
	if base > (2<<20) || exponent > 10 {
		return VeryBigSortSize()
	}
	result := FiniteSortSize(1)
	for i := uint64(0); i < exponent; i++ {
		result = datatypeSizeMultiply(result, FiniteSortSize(base))
	}
	return result
}

type datatypeParamSizeKind uint8

const (
	datatypeParamSizeOffset datatypeParamSizeKind = iota
	datatypeParamSizePlus
	datatypeParamSizeTimes
	datatypeParamSizePower
	datatypeParamSizeParam
)

// datatypeParamSize is the concrete param_size::size expression: offset,
// plus, times, power and sparam. The parameter sorts it names are formal
// parameters pinned by their datatype definitions.
type datatypeParamSize struct {
	kind       datatypeParamSizeKind
	offset     SortSize
	arg1, arg2 *datatypeParamSize
	param      astNodeRef
}

func datatypeParamSizeMkOffset(s SortSize) *datatypeParamSize {
	return &datatypeParamSize{kind: datatypeParamSizeOffset, offset: s}
}

func datatypeParamSizeMkParam(p astNodeRef) *datatypeParamSize {
	return &datatypeParamSize{kind: datatypeParamSizeParam, param: p}
}

func datatypeParamSizeMkBinary(kind datatypeParamSizeKind, a1, a2 *datatypeParamSize) *datatypeParamSize {
	return &datatypeParamSize{kind: kind, arg1: a1, arg2: a2}
}

// datatypeParamSizeMkTimes is size::mk_times(ptr_vector<size>&).
func datatypeParamSizeMkTimes(szs []*datatypeParamSize) *datatypeParamSize {
	if len(szs) == 0 {
		return datatypeParamSizeMkOffset(FiniteSortSize(1))
	}
	if len(szs) == 1 {
		return szs[0]
	}
	r := szs[0]
	for i := 1; i < len(szs); i++ {
		r = datatypeParamSizeMkBinary(datatypeParamSizeTimes, r, szs[i])
	}
	return r
}

// datatypeParamSizeMkPlus is size::mk_plus(ptr_vector<size>&).
func datatypeParamSizeMkPlus(szs []*datatypeParamSize) *datatypeParamSize {
	if len(szs) == 0 {
		return datatypeParamSizeMkOffset(FiniteSortSize(0))
	}
	if len(szs) == 1 {
		return szs[0]
	}
	r := szs[0]
	for i := 1; i < len(szs); i++ {
		r = datatypeParamSizeMkBinary(datatypeParamSizePlus, r, szs[i])
	}
	return r
}

// subst is size::subst. The C++ sparam::subst reads S[m_param] with an
// asserting obj_map lookup; a missing key there is a null dereference (z3
// 4.7.1 crashes when a parametric datatype field instantiates another
// block's parametric datatype at a different sort). Go keeps the parameter.
func (s *datatypeParamSize) subst(S map[astNodeRef]*datatypeParamSize) *datatypeParamSize {
	switch s.kind {
	case datatypeParamSizeOffset:
		return s
	case datatypeParamSizeParam:
		if r, found := S[s.param]; found {
			return r
		}
		return s
	default:
		return datatypeParamSizeMkBinary(s.kind, s.arg1.subst(S), s.arg2.subst(S))
	}
}

// eval is size::eval. A parameter missing from S (unreachable from
// plugin::mk_sort, which binds every definition parameter) is infinite.
func (s *datatypeParamSize) eval(S map[astNodeRef]SortSize) SortSize {
	switch s.kind {
	case datatypeParamSizeOffset:
		return s.offset
	case datatypeParamSizeParam:
		if r, found := S[s.param]; found {
			return r
		}
		return InfiniteSortSize()
	case datatypeParamSizePlus:
		s1 := s.arg1.eval(S)
		s2 := s.arg2.eval(S)
		return datatypeSizeAdd(s1, s2)
	case datatypeParamSizeTimes:
		s1 := s.arg1.eval(S)
		s2 := s.arg2.eval(S)
		return datatypeSizeMultiply(s1, s2)
	default:
		s1 := s.arg1.eval(S)
		s2 := s.arg2.eval(S)
		return datatypeSizePower(s1, s2)
	}
}

func (m *ASTManager) datatypeStoredSortSize(sort astNodeRef) (SortSize, error) {
	info := m.sortAt(sort.slot).info
	if info == (ASTSortInfo{}) {
		return InfiniteSortSize(), nil
	}
	record, err := m.sortInfo(info)
	if err != nil {
		return SortSize{}, err
	}
	return record.size, nil
}

// datatypeSortParameters returns the sort parameters of a datatype sort,
// util::get_datatype_parameter_sort for each index.
func (m *ASTManager) datatypeSortParameters(sort astNodeRef) ([]astNodeRef, error) {
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil {
		return nil, err
	}
	var result []astNodeRef
	for i := uint32(1); i < info.count; i++ {
		parameter, err := m.infoParameter(info, i)
		if err != nil {
			return nil, err
		}
		value, err := m.parameter(parameter)
		if err != nil {
			return nil, err
		}
		if value.Kind != ASTParameterAST {
			return nil, ErrASTDomain
		}
		result = append(result, value.ast)
	}
	return result, nil
}

// datatypeUtilIsDatatypeSort is util::is_datatype(s) with the sort's name.
func (m *ASTManager) datatypeUtilIsDatatypeSort(sort astNodeRef) (usymbol.Symbol, bool, error) {
	p := m.datatypePlugin
	info := m.sortAt(sort.slot).info
	if p == nil || info == (ASTSortInfo{}) {
		return usymbol.Symbol{}, false, nil
	}
	record, err := m.sortInfo(info)
	if err != nil {
		return usymbol.Symbol{}, false, err
	}
	if record.family != p.family || record.kind != ASTDatatypeSort {
		return usymbol.Symbol{}, false, nil
	}
	return m.sortAt(sort.slot).name, true, nil
}

// datatypeGetSortSize is util::get_sort_size.
func (m *ASTManager) datatypeGetSortSize(params []astNodeRef, s astNodeRef) (*datatypeParamSize, error) {
	if len(params) == 0 {
		size, err := m.datatypeStoredSortSize(s)
		if err != nil {
			return nil, err
		}
		return datatypeParamSizeMkOffset(size), nil
	}
	name, isDatatype, err := m.datatypeUtilIsDatatypeSort(s)
	if err != nil {
		return nil, err
	}
	if isDatatype {
		S := make(map[astNodeRef]*datatypeParamSize)
		parameters, err := m.datatypeSortParameters(s)
		if err != nil {
			return nil, err
		}
		for _, ps := range parameters {
			sz, err := m.datatypeGetSortSize(params, ps)
			if err != nil {
				return nil, err
			}
			S[ps] = sz
		}
		d := m.datatypePlugin.definitionNames[name]
		if d == nil || d.sizeFunction == nil {
			// Source get_def asserts the definition and its size exist.
			return nil, ErrASTDomain
		}
		return d.sizeFunction.subst(S), nil
	}
	if m.arrayPlugin != nil {
		fid, kind, err := m.declIdentity(s)
		if err != nil {
			return nil, err
		}
		if fid == m.arrayPlugin.family && kind == 0 {
			info, err := m.sortInfo(m.sortAt(s.slot).info)
			if err != nil {
				return nil, err
			}
			n := info.count - 1
			var szs []*datatypeParamSize
			for i := uint32(0); i < n; i++ {
				domain, err := m.arraySortParameter(info, i)
				if err != nil {
					return nil, err
				}
				sz, err := m.datatypeGetSortSize(params, domain)
				if err != nil {
					return nil, err
				}
				szs = append(szs, sz)
			}
			sz1 := datatypeParamSizeMkTimes(szs)
			rangeSort, err := m.arraySortParameter(info, n)
			if err != nil {
				return nil, err
			}
			sz2, err := m.datatypeGetSortSize(params, rangeSort)
			if err != nil {
				return nil, err
			}
			return datatypeParamSizeMkBinary(datatypeParamSizePower, sz2, sz1), nil
		}
	}
	for _, p := range params {
		if s == p {
			return datatypeParamSizeMkParam(s), nil
		}
	}
	size, err := m.datatypeStoredSortSize(s)
	if err != nil {
		return nil, err
	}
	return datatypeParamSizeMkOffset(size), nil
}

// datatypeComputeSizeFunctions is util::compute_datatype_size_functions.
// A field whose datatype is still GRAY (on the todo stack) makes the whole
// datatype infinite; other fields read the size currently stored on their
// sort, which for a block sibling is still the default infinite size.
func (m *ASTManager) datatypeComputeSizeFunctions(names []usymbol.Symbol) error {
	const (
		gray  = 1
		black = 2
	)
	p := m.datatypePlugin
	alreadyFound := make(map[usymbol.Symbol]uint8)
	contains := func(s usymbol.Symbol) bool {
		for _, n := range names {
			if n == s {
				return true
			}
		}
		return false
	}
	todo := append([]usymbol.Symbol(nil), names...)
	for len(todo) != 0 {
		s := todo[len(todo)-1]
		if alreadyFound[s] == black {
			todo = todo[:len(todo)-1]
			continue
		}
		alreadyFound[s] = gray
		isInfinite := false
		canProcess := true
		d := p.definitionNames[s]
		if d == nil {
			return ErrASTDomain
		}
		for i := range d.constructors {
			for _, r := range d.constructors[i].ranges {
				s2, isDatatype, err := m.datatypeUtilIsDatatypeSort(r)
				if err != nil {
					return err
				}
				if !isDatatype {
					continue
				}
				if st, found := alreadyFound[s2]; found {
					if st == gray {
						isInfinite = true
					}
				} else if contains(s2) {
					todo = append(todo, s2)
					canProcess = false
				}
			}
		}
		if !canProcess {
			continue
		}
		todo = todo[:len(todo)-1]
		alreadyFound[s] = black
		if isInfinite {
			d.sizeFunction = datatypeParamSizeMkOffset(InfiniteSortSize())
			continue
		}
		var sAdd []*datatypeParamSize
		for i := range d.constructors {
			var sMul []*datatypeParamSize
			for _, r := range d.constructors[i].ranges {
				sz, err := m.datatypeGetSortSize(d.parameters, r)
				if err != nil {
					return err
				}
				sMul = append(sMul, sz)
			}
			sAdd = append(sAdd, datatypeParamSizeMkTimes(sMul))
		}
		d.sizeFunction = datatypeParamSizeMkPlus(sAdd)
	}
	return nil
}

// datatypeApplySortSize is the size step of plugin::mk_sort: when the
// sort's definition has a size function, evaluate it with each definition
// parameter bound to the stored size of the matching sort argument.
func (m *ASTManager) datatypeApplySortSize(sort astNodeRef, arguments []astNodeRef) error {
	d := m.datatypePlugin.definitionNames[m.sortAt(sort.slot).name]
	if d == nil || d.sizeFunction == nil {
		return nil
	}
	S := make(map[astNodeRef]SortSize, len(arguments))
	for i, r := range arguments {
		if i >= len(d.parameters) {
			break
		}
		size, err := m.datatypeStoredSortSize(r)
		if err != nil {
			return err
		}
		S[d.parameters[i]] = size
	}
	ts := d.sizeFunction.eval(S)
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil {
		return err
	}
	info.size = ts
	return nil
}
