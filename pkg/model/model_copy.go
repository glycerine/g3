package model

// Source: src/model/model.cpp:44-75;
// src/tactic/filter_model_converter.cpp:42-50.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// CopyFunctionInterpretation owns the source fi->copy temporary until successful
// registration transfers it. A stopped partial copy remains reachable by Close.
func (md *ModelCore) CopyFunctionInterpretation(d ast.ASTHandle, source *FunctionInterpretation) error {
	if e := md.open(); e != nil {
		return e
	}
	if e := source.open(); e != nil {
		return e
	}
	if source.manager != md.manager {
		return ast.ErrASTHandle
	}
	if md.copiedFunction != nil {
		return arena.ErrClosed
	}
	var e error
	md.copiedFunction, e = source.copyWithMemoryTrace(true)
	if e != nil {
		return e
	}
	if e = md.RegisterFunction(d, md.copiedFunction); e != nil {
		return e
	}
	md.copiedFunction = nil
	return nil
}
func (md *ModelCore) CopyFunctionInterpretations(source *ModelCore) error {
	if e := md.open(); e != nil {
		return e
	}
	if e := source.open(); e != nil {
		return e
	}
	if md.manager != source.manager {
		return ast.ErrASTHandle
	}
	for _, entry := range source.functionEntries() {
		if entry.function != nil {
			if e := md.CopyFunctionInterpretation(entry.declaration, entry.function); e != nil {
				return e
			}
		}
	}
	return nil
}
func (md *Model) CopyFunctionInterpretations(source *Model) error {
	if e := md.open(); e != nil {
		return e
	}
	if e := source.open(); e != nil {
		return e
	}
	return md.CoreField.CopyFunctionInterpretations(&source.CoreField)
}

// Copy follows source construction, then constants, functions and universes.
// A nonnil partial copy returned on failure must be closed by its caller.
func (md *Model) Copy() (copied *Model, err error) {
	if err = md.open(); err != nil {
		return nil, err
	}
	copied, err = NewModel(md.CoreField.manager)
	if err != nil {
		return copied, err
	}
	copied.traceSourceAllocated("src/model/model.cpp", 69)
	if err = copied.CoreField.CopyConstantInterpretations(&md.CoreField); err != nil {
		return copied, err
	}
	if err = copied.CopyFunctionInterpretations(md); err != nil {
		return copied, err
	}
	err = copied.CopyUniverseInterpretations(md)
	return copied, err
}

// TransferFunctionInterpretations is proto_model::mk_model's m_finterp loop:
// in table order each interpretation moves to md (m->register_decl(k, fi)),
// and the source drops its key reference; then m_finterp.reset(). The source
// keeps its constants.
func (md *ModelCore) TransferFunctionInterpretations(source *ModelCore) error {
	if e := md.open(); e != nil {
		return e
	}
	if e := source.open(); e != nil {
		return e
	}
	if md.manager != source.manager || md == source {
		return ast.ErrASTHandle
	}
	for _, entry := range source.functionEntries() {
		if entry.function == nil {
			continue
		}
		if e := md.RegisterFunction(entry.declaration, entry.function); e != nil {
			return e
		}
		entry.function = nil
		if entry.declarationOwned {
			consumed, e := source.manager.DecRefConsumed(entry.declaration)
			if consumed {
				entry.declarationOwned = false
			}
			if e != nil {
				return e
			}
		}
		source.Functions.Del(entry.declaration)
	}
	source.functionLayout.Reset()
	source.functionDeclarations = nil
	return nil
}
