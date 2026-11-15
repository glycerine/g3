package ast

// Source: src/ast/array_decl_plugin.cpp:329-431;
// src/ast/array_decl_plugin.h:176-185.

import (
	"errors"
	"fmt"
)

func (m *ASTManager) arraySetDecl(kind int32, domain []ASTHandle) (result astNodeRef, err error) {
	n := len(domain)
	switch kind {
	case ASTArrayOpSetUnion:
		if n == 0 {
			return result, arrayDeclarationError("union takes at least one argument")
		}
	case ASTArrayOpSetIntersect:
		if n == 0 {
			return result, arrayDeclarationError("intersection takes at least one argument")
		}
	case ASTArrayOpSetDifference:
		if n != 2 {
			return result, arrayDeclarationError("set difference takes precisely two arguments")
		}
	case ASTArrayOpSetSubset:
		if n != 2 {
			return result, arrayDeclarationError("subset takes two arguments")
		}
	case ASTArrayOpSetComplement:
		if n != 1 {
			return result, arrayDeclarationError("set complement takes one argument")
		}
	default:
		return result, ErrASTDomain
	}
	// Source array_decl_plugin::check_set_arguments.
	var first astNodeRef
	for i, h := range domain {
		s, e := m.requireKind(h, ASTSort)
		if e != nil {
			return result, e
		}
		if i == 0 {
			first = s
		} else if s != first {
			return result, arrayDeclarationError(fmt.Sprintf("arguments 1 and %d have different sorts", i+1))
		}
		fid, _, e := m.declIdentity(s)
		if e != nil {
			return result, e
		}
		if fid != m.arrayPlugin.family {
			return result, arrayDeclarationError(fmt.Sprintf("argument %d is not of array sort", i+1))
		}
	}
	info, e := m.sortInfo(m.sortAt(first.slot).info)
	if e != nil {
		return result, e
	}
	if info.count <= 1 {
		return result, arrayDeclarationError("expecting 2 or more parameters")
	}
	lastParameter, e := m.infoParameter(info, info.count-1)
	if e != nil {
		return result, e
	}
	lastRecord, e := m.parameter(lastParameter)
	if e != nil {
		return result, e
	}
	if lastRecord.Kind != ASTParameterAST {
		return result, arrayDeclarationError("expecting term parameters")
	}
	if m.nodeAt(lastRecord.ast.slot).kind != ASTSort || lastRecord.ast != m.boolSort {
		return result, arrayDeclarationError("expecting boolean range")
	}
	nameIndex := kind - ASTArrayOpSetUnion + 5
	resultSort := first
	if kind == ASTArrayOpSetSubset {
		resultSort = m.boolSort
	}
	if kind == ASTArrayOpSetUnion || kind == ASTArrayOpSetIntersect {
		flags := ASTFuncFlags{LeftAssociative: true, RightAssociative: true, Commutative: true, Idempotent: true}
		normalized := []ASTHandle{domain[0], domain[0]}
		var ps []ASTParameter
		if kind == ASTArrayOpSetUnion {
			p, e := m.newParameterAST(domain[0])
			if e != nil {
				return result, e
			}
			defer func() { err = errors.Join(err, m.dropParameter(p)) }()
			ps = []ASTParameter{p}
		}
		return m.pluginDecl(m.arrayPlugin.names[nameIndex], m.arrayPlugin.family, kind, ps, normalized, resultSort, flags, false)
	}
	return m.pluginDecl(m.arrayPlugin.names[nameIndex], m.arrayPlugin.family, kind, nil, domain, resultSort, ASTFuncFlags{}, false)
}

func (m *ASTManager) mkConstArray(sort, value ASTHandle) (result ASTHandle, err error) {
	name, e := m.Symbols.Intern("array")
	if e != nil {
		return result, e
	}
	fid, e := m.families.MkID(name)
	if e != nil {
		return result, e
	}
	p, e := m.newParameterAST(sort)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(p)) }()
	r, e := m.mkFamilyApp(fid, ASTArrayOpConst, []ASTParameter{p}, []ASTHandle{value})
	return m.external(r), e
}
func (m *ASTManager) MkConstArray(sort, value ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkConstArray(sort, value)
}
func (m *ASTManager) MkEmptySet(sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkConstArray(sort, m.external(m.falseApp))
}
func (m *ASTManager) MkFullSet(sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkConstArray(sort, m.external(m.trueApp))
}
