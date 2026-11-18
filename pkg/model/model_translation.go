package model

// Source: src/model/func_interp.cpp:328-341; src/model/model.cpp:152-182.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Translate copies a function graph through an ASTTranslation. The returned
// graph owns all translated entries and its fallback.
func (f *FunctionInterpretation) Translate(translation *ast.ASTTranslation) (result *FunctionInterpretation, err error) {
	if err = f.open(); err != nil {
		return nil, err
	}
	if translation == nil || translation.From() != f.manager || translation.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err = NewFunctionInterpretation(translation.To(), f.arity)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	args := make([]ast.ASTHandle, f.arity)
	for _, entry := range f.entries {
		for n := uint32(0); n < f.arity; n++ {
			arg, e := entry.Argument(n)
			if e != nil {
				return nil, e
			}
			args[n], e = translation.Translate(arg)
			if e != nil {
				return nil, e
			}
		}
		value, e := entry.Result()
		if e != nil {
			return nil, e
		}
		value, e = translation.Translate(value)
		if e != nil {
			return nil, e
		}
		if e = result.InsertNewEntry(args, value); e != nil {
			return nil, e
		}
	}
	fallback, err := translation.Translate(f.fallback)
	if err != nil {
		return nil, err
	}
	if err = result.SetElse(fallback); err != nil {
		return nil, err
	}
	return result, nil
}

// Translate copies constants, function graphs, and universes in their source
// dictionary order through a shared AST translation cache.
func (md *Model) Translate(translation *ast.ASTTranslation) (result *Model, err error) {
	if err = md.open(); err != nil {
		return nil, err
	}
	if translation == nil || translation.From() != md.CoreField.manager || translation.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err = NewModel(translation.To())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	for _, entry := range md.CoreField.constantEntries() {
		// register_decl(translator(m_key), translator(m_value)): the
		// release build (GCC, right-to-left arguments) translates the
		// value first.
		translatedValue, e := translation.Translate(entry.value)
		if e != nil {
			return nil, e
		}
		translatedDeclaration, e := translation.Translate(entry.declaration)
		if e != nil {
			return nil, e
		}
		if e = result.Core().RegisterConstant(translatedDeclaration, translatedValue); e != nil {
			return nil, e
		}
	}
	for _, entry := range md.CoreField.functionEntries() {
		// fi->translate(translator) runs before translator(m_key).
		translatedFunction, e := entry.function.Translate(translation)
		if e != nil {
			return nil, e
		}
		translatedDeclaration, e := translation.Translate(entry.declaration)
		if e != nil {
			return nil, errors.Join(e, translatedFunction.Close())
		}
		if e = result.Core().RegisterFunction(translatedDeclaration, translatedFunction); e != nil {
			return nil, errors.Join(e, translatedFunction.Close())
		}
	}
	for _, sort := range md.universeSorts() {
		universe, _ := md.Universes.Get2(sort)
		// new_universe is built before translator(it3->m_key).
		values := make([]ast.ASTHandle, len(universe.values))
		var e error
		for n, value := range universe.values {
			values[n], e = translation.Translate(value)
			if e != nil {
				return nil, e
			}
		}
		translatedSort, e := translation.Translate(sort)
		if e != nil {
			return nil, e
		}
		if e = result.RegisterUniverse(translatedSort, values); e != nil {
			return nil, e
		}
	}
	return result, nil
}
