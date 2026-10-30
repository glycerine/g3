package arith

// Source: src/tactic/arith/fm_tactic.cpp:1260-1275,1497-1504.
import "github.com/glycerine/g3/pkg/ast"

type FMEliminator struct {
	Store                  *FMConstraintStore
	Variables              *FMVariableTable
	Config                 FMEliminationConfig
	Inconsistent           bool
	InconsistentDependency ast.ASTDependency
	inconsistentOwned      bool
}

func NewFMEliminator(store *FMConstraintStore, variables *FMVariableTable, config FMEliminationConfig) (*FMEliminator, error) {
	if store == nil || variables == nil {
		return nil, ast.ErrASTDomain
	}
	return &FMEliminator{Store: store, Variables: variables, Config: config}, nil
}

func (e *FMEliminator) markDead(x uint32) {
	for _, c := range e.Variables.Index.Lower[x] {
		c.Dead = true
	}
	for _, c := range e.Variables.Index.Upper[x] {
		c.Dead = true
	}
}

func (e *FMEliminator) setInconsistent(dependency ast.ASTDependency) error {
	e.Inconsistent = true
	e.InconsistentDependency = dependency
	if dependency.IsNull() {
		return nil
	}
	if err := e.Store.manager.DependencyInc(dependency); err != nil {
		e.Inconsistent = false
		e.InconsistentDependency = ast.ASTDependency{}
		return err
	}
	e.inconsistentOwned = true
	return nil
}

func (e *FMEliminator) Close() error {
	if !e.inconsistentOwned {
		return nil
	}
	consumed, err := e.Store.manager.DependencyDecConsumed(e.InconsistentDependency)
	if consumed {
		e.inconsistentOwned = false
		e.InconsistentDependency = ast.ASTDependency{}
	}
	return err
}
