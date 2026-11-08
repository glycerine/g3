package fpa

// Source: src/ast/fpa/bv2fpa_converter.cpp:505-542.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Translate copies the converter cache through an ASTTranslation. The result
// owns its translated cache roots independently of both the source converter
// and the translation cache.
func (c *Converter) Translate(translation *ast.ASTTranslation) (*Converter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if translation == nil || translation.From() != c.manager {
		return nil, ast.ErrASTHandle
	}
	result, err := NewConverter(translation.To(), nil)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Converter, error) {
		return nil, errors.Join(err, result.Close())
	}
	translatePair := func(key, value ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, error) {
		translatedKey, err := translation.Translate(key)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		translatedValue, err := translation.Translate(value)
		return translatedKey, translatedValue, err
	}
	retainPair := func(key, value ast.ASTHandle) error {
		if err := result.manager.IncRef(key); err != nil {
			return err
		}
		if err := result.manager.IncRef(value); err != nil {
			return errors.Join(err, result.manager.DecRef(key))
		}
		return nil
	}
	for _, entry := range c.constants.Entries() {
		key, value := entry.Key, entry.Value[0]
		translatedKey, translatedValue, e := translatePair(key, value)
		if e != nil {
			return fail(e)
		}
		if e = retainPair(translatedKey, translatedValue); e != nil {
			return fail(e)
		}
		if err := result.constants.Insert(translatedKey, [2]ast.ASTHandle{translatedValue}); err != nil {
			return nil, err
		}
	}
	for _, entry := range c.roundingConstants.Entries() {
		key, value := entry.Key, entry.Value[0]
		translatedKey, translatedValue, e := translatePair(key, value)
		if e != nil {
			return fail(e)
		}
		if e = retainPair(translatedKey, translatedValue); e != nil {
			return fail(e)
		}
		if err := result.roundingConstants.Insert(translatedKey, [2]ast.ASTHandle{translatedValue}); err != nil {
			return nil, err
		}
	}
	for _, entry := range c.convertedUFs.Entries() {
		key, value := entry.Key, entry.Value[0]
		translatedKey, translatedValue, e := translatePair(key, value)
		if e != nil {
			return fail(e)
		}
		if e = retainPair(translatedKey, translatedValue); e != nil {
			return fail(e)
		}
		if err := result.convertedUFs.Insert(translatedKey, [2]ast.ASTHandle{translatedValue}); err != nil {
			return nil, err
		}
	}
	for _, entry := range c.minMaxSigns.Entries() {
		key, values := entry.Key, entry.Value
		translatedKey, e := translation.Translate(key)
		if e != nil {
			return fail(e)
		}
		translatedFirst, e := translation.Translate(values[0])
		if e != nil {
			return fail(e)
		}
		translatedSecond, e := translation.Translate(values[1])
		if e != nil {
			return fail(e)
		}
		if e = result.manager.IncRef(translatedKey); e != nil {
			return fail(e)
		}
		if e = result.manager.IncRef(translatedFirst); e != nil {
			return fail(errors.Join(e, result.manager.DecRef(translatedKey)))
		}
		if e = result.manager.IncRef(translatedSecond); e != nil {
			return fail(errors.Join(e, result.manager.DecRef(translatedFirst), result.manager.DecRef(translatedKey)))
		}
		if err := result.minMaxSigns.Insert(translatedKey, [2]ast.ASTHandle{translatedFirst, translatedSecond}); err != nil {
			return nil, err
		}
	}
	return result, nil
}
