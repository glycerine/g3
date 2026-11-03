package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrUDocDatalogFragment = errors.New("muz/fp: unsupported doc Datalog fragment")

// The source doc plugin owns one width-indexed manager cache. Expression
// query copies borrow that cache and independently own their relation values.
type datalogUDocContext struct {
	manager  *ast.ASTManager
	factory  *rel.UDocRelationFactory
	borrowed bool
	seeds    map[ast.ASTHandle]*rel.TypedUDocRelation
	result   map[ast.ASTHandle]*rel.TypedUDocRelation
	order    []ast.ASTHandle
}

func newDatalogUDocContext(manager *ast.ASTManager) (*datalogUDocContext, error) {
	factory, err := rel.NewUDocRelationFactory(manager, arena.VectorConfig{})
	if err != nil {
		return nil, err
	}
	return &datalogUDocContext{manager: manager, factory: factory, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation)}, nil
}

func (c *datalogUDocContext) discardResult() error {
	for d, r := range c.result {
		if err := r.Close(); err != nil {
			return err
		}
		delete(c.result, d)
	}
	c.result = nil
	return nil
}

func (c *datalogUDocContext) close() error {
	if c == nil {
		return nil
	}
	if err := c.discardResult(); err != nil {
		return err
	}
	for d, r := range c.seeds {
		if err := r.Close(); err != nil {
			return err
		}
		delete(c.seeds, d)
	}
	if !c.borrowed && c.factory != nil {
		c.factory.Close()
	}
	c.factory = nil
	return nil
}

func (c *datalogUDocContext) sync(e *PropositionalEngine) error {
	// Validate the whole new signature set before creating or publishing owners.
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		supported, err := c.factory.CanHandleSignature(state.sorts)
		if err != nil {
			return err
		}
		if !supported {
			return ErrUDocDatalogFragment
		}
	}
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		relation := c.seeds[d]
		if relation == nil {
			var err error
			relation, err = c.factory.NewEmpty(state.sorts)
			if err != nil {
				return err
			}
			// Keep new owners reachable across a migration failure.
			c.seeds[d] = relation
			c.order = append(c.order, d)
		}
		if state.table != nil {
			facts, err := state.table.Facts()
			if err != nil {
				return err
			}
			for _, fact := range facts {
				values := make([]uint64, len(fact))
				for i, value := range fact {
					values[i] = uint64(value)
				}
				if _, err = relation.Core().AddFact(values); err != nil {
					return err
				}
			}
		}
		state.table = nil
	}
	return nil
}

func (c *datalogUDocContext) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	relation := c.seeds[declaration]
	if relation == nil {
		return ErrUDocDatalogFragment
	}
	if err := c.discardResult(); err != nil {
		return err
	}
	_, err := relation.AddFact(values)
	return err
}

func (c *datalogUDocContext) snapshot() error {
	if err := c.discardResult(); err != nil {
		return err
	}
	c.result = make(map[ast.ASTHandle]*rel.TypedUDocRelation, len(c.seeds))
	for _, d := range c.order {
		copy, err := c.seeds[d].Clone()
		if err != nil {
			return err
		}
		c.result[d] = copy
	}
	return nil
}

func (c *datalogUDocContext) status(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	for _, d := range declarations {
		r := c.result[d]
		if r == nil {
			return lbool.Undef, ErrUDocDatalogFragment
		}
		empty, err := r.Empty()
		if err != nil {
			return lbool.Undef, err
		}
		if !empty {
			return lbool.True, nil
		}
	}
	return lbool.False, nil
}

func (c *datalogUDocContext) answer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	r := c.result[declaration]
	if r == nil {
		return ast.ASTRef{}, ErrUDocDatalogFragment
	}
	return r.ToFormula()
}

func (c *datalogUDocContext) clone() (*datalogUDocContext, error) {
	copy := &datalogUDocContext{manager: c.manager, factory: c.factory, borrowed: true, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation), order: append([]ast.ASTHandle(nil), c.order...)}
	for _, d := range c.order {
		relation, err := c.seeds[d].Clone()
		if err != nil {
			return copy, err
		}
		copy.seeds[d] = relation
	}
	return copy, nil
}
