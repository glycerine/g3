package ast

// Source: src/ast/array_decl_plugin.cpp:139-211,468-476;
// src/ast/array_decl_plugin.h:172-175.

import (
	"errors"
	"fmt"
)

func (m *ASTManager) arrayMapParameter(ps []ASTParameter) (astNodeRef, error) {
	const message = "array operation requires one function declaration parameter (the function to be mapped)"
	if len(ps) != 1 {
		return astNodeRef{}, arrayDeclarationError(message)
	}
	p, e := m.parameter(ps[0])
	if e != nil {
		return astNodeRef{}, e
	}
	if p.Kind != ASTParameterAST {
		return astNodeRef{}, arrayDeclarationError(message)
	}
	r, e := m.internal(m.external(p.ast))
	if e != nil {
		return astNodeRef{}, e
	}
	if m.nodeAt(r.slot).kind != ASTFuncDecl {
		return astNodeRef{}, arrayDeclarationError(message)
	}
	return r, nil
}

// arrayMapArraySort is is_array_sort(s) for array_decl_plugin::mk_map: the
// sort's info when s is an ARRAY_SORT of this plugin, else nil.
func (m *ASTManager) arrayMapArraySort(h ASTHandle) (*astInfoRecord, error) {
	s, e := m.requireKind(h, ASTSort)
	if e != nil {
		return nil, e
	}
	fid, kind, e := m.declIdentity(s)
	if e != nil {
		return nil, e
	}
	if fid != m.arrayPlugin.family || kind != 0 {
		return nil, nil
	}
	return m.sortInfo(m.sortAt(s.slot).info)
}
func (m *ASTManager) arrayMapSort(h ASTHandle) (*astInfoRecord, error) {
	s, e := m.requireKind(h, ASTSort)
	if e != nil {
		return nil, e
	}
	fid, kind, e := m.declIdentity(s)
	if e != nil {
		return nil, e
	}
	if m.arrayPlugin == nil || fid != m.arrayPlugin.family || kind != 0 {
		return nil, ErrASTDomain
	}
	return m.sortInfo(m.sortAt(s.slot).info)
}
func (m *ASTManager) arrayMapDecl(ps []ASTParameter, domain []ASTHandle) (result astNodeRef, err error) {
	f, e := m.arrayMapParameter(ps)
	if e != nil {
		return result, e
	}
	decl := m.declAt(f.slot)
	if uint64(len(domain)) != uint64(decl.arity) {
		return result, arrayDeclarationError(fmt.Sprintf("map expects to take as many arguments as the function being mapped, it was given %d but expects %d", len(domain), decl.arity))
	}
	if len(domain) == 0 {
		return result, arrayDeclarationError("don't use map on constants")
	}
	var first *astInfoRecord
	for i, h := range domain {
		current, e := m.arrayMapArraySort(h)
		if e != nil {
			return result, e
		}
		if current == nil {
			return result, arrayDeclarationError(fmt.Sprintf("map expects an array sort as argument at position %d", i))
		}
		if i == 0 {
			first = current
		}
		// The source's double space after "domain," is kept.
		if current.count != first.count {
			return result, arrayDeclarationError(fmt.Sprintf("map expects all arguments to have the same array domain,  this is not the case for argument %d", i))
		}
		for j := uint32(0); j+1 < first.count; j++ {
			a, e := m.arraySortParameter(first, j)
			if e != nil {
				return result, e
			}
			b, e := m.arraySortParameter(current, j)
			if e != nil {
				return result, e
			}
			if a != b {
				return result, arrayDeclarationError(fmt.Sprintf("map expects all arguments to have the same array domain, this is not the case for argument %d", i))
			}
		}
		actual, e := m.arraySortParameter(current, current.count-1)
		if e != nil {
			return result, e
		}
		expected, e := m.readInternal(decl.domain, uint32(i))
		if e != nil {
			return result, e
		}
		// Source requires identity; map does not use array coercion compatibility.
		if actual != expected {
			return result, arrayDeclarationError(fmt.Sprintf("map expects the argument at position %d to have the array range the same as the function", i))
		}
	}
	var parameters []ASTParameter
	defer func() { err = errors.Join(err, m.dropArrayParameters(parameters)) }()
	for j := uint32(0); j+1 < first.count; j++ {
		s, e := m.arraySortParameter(first, j)
		if e != nil {
			return result, e
		}
		p, e := m.newParameterAST(m.external(s))
		if e != nil {
			return result, e
		}
		parameters = append(parameters, p)
	}
	p, e := m.newParameterAST(m.external(decl.rangeSort))
	if e != nil {
		return result, e
	}
	parameters = append(parameters, p)
	output, e := m.arraySort(0, parameters)
	if e != nil {
		return result, e
	}
	flags, e := m.funcFlags(f)
	if e != nil {
		return result, e
	}
	inherited := ASTFuncFlags{LeftAssociative: flags.LeftAssociative, RightAssociative: flags.RightAssociative, Commutative: flags.Commutative, Injective: flags.Injective}
	return m.pluginDecl(m.arrayPlugin.names[4], m.arrayPlugin.family, ASTArrayOpMap, ps, domain, output, inherited, false)
}

// ArrayMapFunction borrows the function parameter of an array-map declaration.
func (m *ASTManager) ArrayMapFunction(d ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(d, ASTFuncDecl)
	if e != nil {
		return ASTHandle{}, e
	}
	fid, kind, e := m.declIdentity(r)
	if e != nil {
		return ASTHandle{}, e
	}
	if m.arrayPlugin == nil || fid != m.arrayPlugin.family || kind != ASTArrayOpMap {
		return ASTHandle{}, ErrASTDomain
	}
	info, e := m.funcInfo(m.declAt(r.slot).info)
	if e != nil {
		return ASTHandle{}, e
	}
	if info.count != 1 {
		return ASTHandle{}, ErrASTDomain
	}
	p, e := m.infoParameter(info, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	f, e := m.arrayMapParameter([]ASTParameter{p})
	return m.external(f), e
}

// MkArrayMap follows array_util::mk_map with a borrowed argument view.
func (m *ASTManager) MkArrayMap(f ASTHandle, args AppArguments) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	name, e := m.Symbols.Intern("array")
	if e != nil {
		return result, e
	}
	fid, e := m.families.MkID(name)
	if e != nil {
		return result, e
	}
	p, e := m.newParameterAST(f)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(p)) }()
	return m.mkArrayAppArguments(fid, ASTArrayOpMap, []ASTParameter{p}, args)
}

// Source: src/ast/array_decl_plugin.h array sort accessors;
// src/ast/array_decl_plugin.cpp:589-598, vector<parameter> construction.
func (m *ASTManager) ArrayArity(s ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	info, e := m.arrayMapSort(s)
	if e != nil {
		return 0, e
	}
	return info.count - 1, nil
}
func (m *ASTManager) ArrayDomain(s ASTHandle, i uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	info, e := m.arrayMapSort(s)
	if e != nil {
		return ASTHandle{}, e
	}
	if i >= info.count-1 {
		return ASTHandle{}, ErrASTDomain
	}
	r, e := m.arraySortParameter(info, i)
	return m.external(r), e
}

// ArrayRange returns the final sort parameter, matching get_array_range.
func (m *ASTManager) ArrayRange(s ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	info, e := m.arrayMapSort(s)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e := m.arraySortParameter(info, info.count-1)
	return m.external(r), e
}
func (m *ASTManager) MkArraySort(domain AppArguments, rangeSort ASTHandle) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	name, e := m.Symbols.Intern("array")
	if e != nil {
		return result, e
	}
	fid, e := m.families.MkID(name)
	if e != nil {
		return result, e
	}
	var parameters []ASTParameter
	defer func() { err = errors.Join(err, m.dropArrayParameters(parameters)) }()
	for i := uint32(0); i < domain.Count(); i++ {
		s, e := domain.At(i)
		if e != nil {
			return result, e
		}
		p, e := m.newParameterAST(s)
		if e != nil {
			return result, e
		}
		parameters = append(parameters, p)
	}
	p, e := m.newParameterAST(rangeSort)
	if e != nil {
		return result, e
	}
	parameters = append(parameters, p)
	if !m.pluginAt(fid).present {
		return result, nil
	}
	r, e := m.arraySort(0, parameters)
	return m.external(r), e
}
