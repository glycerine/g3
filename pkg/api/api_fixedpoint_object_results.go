package api

// Source: src/api/api_datalog.cpp:314-349, 405-440.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// NumLevels reports the number of PDR or Spacer frames for a relation.
func (f *APIFixedpoint) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	if err := f.open(); err != nil {
		return 0, err
	}
	return f.command.FixedpointNumLevels(declaration)
}

// CoverDelta returns an owning reference to a PDR or Spacer frame.
func (f *APIFixedpoint) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.FixedpointCoverDelta(level, declaration)
}

// AddCover adds a property to a PDR or Spacer frame.
func (f *APIFixedpoint) AddCover(level int, declaration, property ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointCover(level, declaration, property)
}

// AddInvariant adds a property to the invariant frame.
func (f *APIFixedpoint) AddInvariant(declaration, property ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointInvariant(declaration, property)
}

// Reachable returns an owning reference to the Spacer reach summary.
func (f *APIFixedpoint) Reachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.FixedpointReachable(declaration)
}

func (f *APIFixedpoint) ReasonUnknown() (string, error) {
	if err := f.open(); err != nil {
		return "", err
	}
	return f.command.FixedpointReasonUnknown()
}

func (f *APIFixedpoint) Answer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.FixedpointAnswer(declaration)
}

func (f *APIFixedpoint) Model(declaration ast.ASTHandle) (*model.Model, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return f.command.FixedpointModel(declaration)
}

func (f *APIFixedpoint) EvaluateModel(declaration, expression ast.ASTHandle, completion bool) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.EvaluateFixedpointModel(declaration, expression, completion)
}
