package ast

// Source: src/ast/datatype_decl_plugin.cpp:193-381;
// src/ast/datatype_decl_plugin.h:24-38,237-288.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

const ASTDatatypeSort int32 = 0

const (
	ASTDatatypeConstructor int32 = iota
	ASTDatatypeRecognizer
	ASTDatatypeIs
	ASTDatatypeAccessor
	ASTDatatypeUpdateField
)

func datatypeDeclarationError(message string) error {
	return ASTException{z3exception.NewDefaultException(message)}
}

// astDatatypeUpdateFieldSortError is plugin::mk_update_field's pending
// "second argument to field update should be <rng> instead of <dom>",
// whose sorts formatSortErrors prints with mk_ismt2_pp once the manager
// is unlocked.
type astDatatypeUpdateFieldSortError struct{ expected, actual astNodeRef }

func (e *astDatatypeUpdateFieldSortError) Error() string {
	return "second argument to field update has the wrong sort"
}

func (m *ASTManager) isDatatypeSort(p *astDatatypePluginOwner, h ASTHandle) (astNodeRef, bool, error) {
	r, err := m.requireKind(h, ASTSort)
	if err != nil {
		return astNodeRef{}, false, err
	}
	info, err := m.sortInfo(m.sortAt(r.slot).info)
	if err != nil {
		return astNodeRef{}, false, err
	}
	return r, info.family == p.family && info.kind == ASTDatatypeSort, nil
}

func (m *ASTManager) datatypeSort(kind int32, parameters []ASTParameter) (result astNodeRef, err error) {
	return m.datatypeSortCore(kind, parameters, true)
}

// datatypeSortCore is plugin::mk_sort. applySize false is the block's first
// instantiation in end_def_block, where the C++ m_defs already holds the new
// definition, which has no size function yet ("not setting size").
func (m *ASTManager) datatypeSortCore(kind int32, parameters []ASTParameter, applySize bool) (result astNodeRef, err error) {
	p := m.datatypePlugin
	if p == nil || !p.published {
		return astNodeRef{}, ErrASTHandle
	}
	if kind != ASTDatatypeSort || len(parameters) < 1 {
		return astNodeRef{}, datatypeDeclarationError("invalid datatype")
	}
	first, err := m.parameter(parameters[0])
	if err != nil {
		return astNodeRef{}, err
	}
	if first.Kind != ASTParameterSymbol {
		return astNodeRef{}, datatypeDeclarationError("invalid datatype")
	}
	for _, parameter := range parameters[1:] {
		value, e := m.parameter(parameter)
		if e != nil {
			return astNodeRef{}, e
		}
		if value.Kind != ASTParameterAST {
			return astNodeRef{}, datatypeDeclarationError("invalid datatype")
		}
		if e = m.validNode(value.ast); e != nil || m.nodeAt(value.ast.slot).kind != ASTSort {
			if e != nil {
				return astNodeRef{}, e
			}
			return astNodeRef{}, datatypeDeclarationError("invalid datatype")
		}
	}
	slot, generation, err := m.newInfo(true, p.family, kind, parameters, InfiniteSortSize(), ASTFuncFlags{}, true)
	if err != nil {
		return astNodeRef{}, err
	}
	defer func() { err = errors.Join(err, m.dropInfoStorage(slot, generation)) }()
	result, err = m.mkSort(first.symbol, ASTSortInfo{m.origin, slot, generation})
	if err == nil && applySize {
		arguments := make([]astNodeRef, 0, len(parameters)-1)
		for _, parameter := range parameters[1:] {
			value, e := m.parameter(parameter)
			if e != nil {
				return result, e
			}
			arguments = append(arguments, value.ast)
		}
		err = m.datatypeApplySortSize(result, arguments)
	}
	return result, err
}

func (m *ASTManager) datatypeDecl(kind int32, parameters []ASTParameter, domain []ASTHandle, suppliedRange ASTHandle) (astNodeRef, error) {
	p := m.datatypePlugin
	if p == nil || !p.published {
		return astNodeRef{}, ErrASTHandle
	}
	if kind < ASTDatatypeConstructor || kind > ASTDatatypeUpdateField {
		return astNodeRef{}, datatypeDeclarationError("invalid datatype operator kind")
	}
	for _, h := range domain {
		if _, err := m.requireKind(h, ASTSort); err != nil {
			return astNodeRef{}, err
		}
	}
	parameter := func(index int, expected ASTParameterKind) (*astParameterRecord, error) {
		if index < 0 || index >= len(parameters) {
			return nil, ErrASTDomain
		}
		value, err := m.parameter(parameters[index])
		if err != nil {
			return nil, err
		}
		if value.Kind != expected {
			return nil, ErrASTDomain
		}
		return value, nil
	}
	funcDeclParameter := func(index int) (astNodeRef, error) {
		value, err := parameter(index, ASTParameterAST)
		if err != nil {
			return astNodeRef{}, err
		}
		if err = m.validNode(value.ast); err != nil {
			return astNodeRef{}, err
		}
		if m.nodeAt(value.ast.slot).kind != ASTFuncDecl {
			return astNodeRef{}, ErrASTDomain
		}
		return value.ast, nil
	}
	boolRange := func() astNodeRef { return m.boolSort }

	// Source VALIDATE_PARAM(_pred_): "invalid parameter to datatype
	// function " followed by the predicate's source text.
	invalid := func(predicate string) error {
		return datatypeDeclarationError("invalid parameter to datatype function " + predicate)
	}
	isDatatype := func(h ASTHandle) (astNodeRef, bool, error) {
		return m.isDatatypeSort(p, h)
	}

	switch kind {
	case ASTDatatypeConstructor:
		const predicate = "num_parameters == 1 && parameters[0].is_symbol() && range && u().is_datatype(range)"
		if len(parameters) != 1 || suppliedRange.IsNull() {
			return astNodeRef{}, invalid(predicate)
		}
		name, err := parameter(0, ASTParameterSymbol)
		if err != nil {
			return astNodeRef{}, invalid(predicate)
		}
		rangeSort, yes, err := isDatatype(suppliedRange)
		if err != nil {
			return astNodeRef{}, err
		}
		if !yes {
			return astNodeRef{}, invalid(predicate)
		}
		return m.pluginDecl(name.symbol, p.family, kind, parameters, domain, rangeSort, ASTFuncFlags{}, true)

	case ASTDatatypeRecognizer:
		const predicate = "arity == 1 && num_parameters == 2 && parameters[1].is_symbol() && parameters[0].is_ast() && is_func_decl(parameters[0].get_ast())"
		if len(domain) != 1 || len(parameters) != 2 {
			return astNodeRef{}, invalid(predicate)
		}
		name, err := parameter(1, ASTParameterSymbol)
		if err != nil {
			return astNodeRef{}, invalid(predicate)
		}
		if _, err := funcDeclParameter(0); err != nil {
			return astNodeRef{}, invalid(predicate)
		}
		if _, yes, err := isDatatype(domain[0]); err != nil || !yes {
			if err != nil {
				return astNodeRef{}, err
			}
			return astNodeRef{}, invalid("u().is_datatype(domain[0])")
		}
		return m.pluginDecl(name.symbol, p.family, kind, parameters, domain, boolRange(), ASTFuncFlags{}, true)

	case ASTDatatypeIs:
		const predicate = "arity == 1 && num_parameters == 1 && parameters[0].is_ast() && is_func_decl(parameters[0].get_ast())"
		if len(domain) != 1 || len(parameters) != 1 {
			return astNodeRef{}, invalid(predicate)
		}
		if _, err := funcDeclParameter(0); err != nil {
			return astNodeRef{}, invalid(predicate)
		}
		if _, yes, err := isDatatype(domain[0]); err != nil || !yes {
			if err != nil {
				return astNodeRef{}, err
			}
			return astNodeRef{}, invalid("u().is_datatype(domain[0])")
		}
		name, err := m.Symbols.Intern("is")
		if err != nil {
			return astNodeRef{}, err
		}
		return m.pluginDecl(name, p.family, kind, parameters, domain, boolRange(), ASTFuncFlags{}, true)

	case ASTDatatypeAccessor:
		const predicate = "arity == 1 && num_parameters == 2 && parameters[0].is_symbol() && parameters[1].is_symbol()"
		if len(domain) != 1 || len(parameters) != 2 {
			return astNodeRef{}, invalid(predicate)
		}
		name, err := parameter(0, ASTParameterSymbol)
		if err != nil {
			return astNodeRef{}, invalid(predicate)
		}
		if _, err = parameter(1, ASTParameterSymbol); err != nil {
			return astNodeRef{}, invalid(predicate)
		}
		if _, yes, err := isDatatype(domain[0]); err != nil || !yes {
			if err != nil {
				return astNodeRef{}, err
			}
			return astNodeRef{}, invalid("u().is_datatype(domain[0])")
		}
		// Source SASSERT(range).
		if suppliedRange.IsNull() {
			return astNodeRef{}, ErrASTDomain
		}
		rangeSort, err := m.requireKind(suppliedRange, ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		return m.pluginDecl(name.symbol, p.family, kind, parameters, domain, rangeSort, ASTFuncFlags{}, true)

	case ASTDatatypeUpdateField:
		// Source plugin::mk_update_field.
		if len(parameters) != 1 {
			return astNodeRef{}, datatypeDeclarationError("invalid parameters for datatype field update")
		}
		if _, err := parameter(0, ASTParameterAST); err != nil {
			return astNodeRef{}, datatypeDeclarationError("invalid parameters for datatype field update")
		}
		if len(domain) != 2 {
			return astNodeRef{}, datatypeDeclarationError("invalid number of arguments for datatype field update")
		}
		accessor, err := funcDeclParameter(0)
		if err != nil {
			return astNodeRef{}, datatypeDeclarationError("datatype field update requires a datatype accessor as the second argument")
		}
		family, accessorKind, err := m.declIdentity(accessor)
		if err != nil || family != p.family || accessorKind != ASTDatatypeAccessor {
			if err != nil {
				return astNodeRef{}, err
			}
			return astNodeRef{}, datatypeDeclarationError("datatype field update requires a datatype accessor as the second argument")
		}
		record := m.declAt(accessor.slot)
		accessorDomain, err := m.readInternal(record.domain, 0)
		if err != nil {
			return astNodeRef{}, err
		}
		first, err := m.requireKind(domain[0], ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		second, err := m.requireKind(domain[1], ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		if accessorDomain != first {
			return astNodeRef{}, datatypeDeclarationError("first argument to field update should be a data-type")
		}
		if record.rangeSort != second {
			return astNodeRef{}, &astDatatypeUpdateFieldSortError{expected: record.rangeSort, actual: second}
		}
		name, err := m.Symbols.Intern("update-field")
		if err != nil {
			return astNodeRef{}, err
		}
		return m.pluginDecl(name, p.family, kind, parameters, domain, first, ASTFuncFlags{}, false)
	}
	return astNodeRef{}, datatypeDeclarationError("invalid datatype operator kind")
}
