package fp

// Source: src/muz/rel/dl_compiler.cpp make_add_constant_column and
// make_assembling_code, specialized to arithmetic external/interval products.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type externalIntervalGroundConstant struct {
	sort  ast.ASTHandle
	value ast.ASTHandle
}

// The first current relation borrows a cached singleton. Subsequent currents
// own the JOIN register; a predecessor stays reachable until its Close succeeds.
// The ordered cache owns each distinct (sort, numeral AST) only once.
type externalIntervalDatalogGroundTuple struct {
	cache      map[externalIntervalGroundConstant]*rel.ExternalIntervalProduct
	singletons []*rel.ExternalIntervalProduct
	current    *rel.ExternalIntervalProduct
	joined     *rel.ExternalIntervalProduct
	retiring   *rel.ExternalIntervalProduct
	registers  *externalIntervalDatalogRegisters
	head       ast.ASTHandle
}

func (t *externalIntervalDatalogGroundTuple) assembleCached(registers *externalIntervalDatalogRegisters, head ast.ASTHandle, sorts, values []ast.ASTHandle) error {
	t.registers, t.head = registers, head
	return t.assemble(registers.activation.plugin, registers.activation.entries[head].kinds, sorts, values)
}

func (t *externalIntervalDatalogGroundTuple) assemble(plugin *rel.ExternalRelationPlugin, kinds []string, sorts, values []ast.ASTHandle) error {
	if len(sorts) != len(values) || len(values) < 2 || t.current != nil {
		return ErrExternalIntervalDatalogFragment
	}
	t.cache = make(map[externalIntervalGroundConstant]*rel.ExternalIntervalProduct)
	for column, value := range values {
		key := externalIntervalGroundConstant{sort: sorts[column], value: value}
		singleton := t.cache[key]
		if t.registers != nil {
			var err error
			singleton, err = t.registers.unarySingleton(t.head, sorts[column], value)
			if err != nil {
				return err
			}
		} else if singleton == nil {
			created, selected, err := rel.NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{sorts[column]}, kinds)
			if err != nil {
				return err
			}
			if !selected || created == nil {
				return ErrExternalIntervalDatalogFragment
			}
			t.singletons = append(t.singletons, created)
			t.cache[key] = created
			singleton = created
			if err = singleton.AddFact([]ast.ASTHandle{value}); err != nil {
				return err
			}
		}
		if t.current == nil {
			t.current = singleton
			continue
		}
		joined, err := rel.JoinExternalIntervalProducts(t.current, singleton, nil, nil)
		if err != nil {
			return err
		}
		t.retiring, t.joined, t.current = t.joined, joined, joined
		if t.retiring != nil {
			if err = t.retiring.Close(); err != nil {
				return err
			}
			t.retiring = nil
		}
	}
	return nil
}

func (t *externalIntervalDatalogGroundTuple) closeJoined() error {
	if t.joined != nil {
		if err := t.joined.Close(); err != nil {
			return err
		}
		t.joined = nil
	}
	if t.retiring != nil {
		if err := t.retiring.Close(); err != nil {
			return err
		}
		t.retiring = nil
	}
	t.current = nil
	return nil
}

func (t *externalIntervalDatalogGroundTuple) closeSingletons() error {
	for len(t.singletons) != 0 {
		last := len(t.singletons) - 1
		if err := t.singletons[last].Close(); err != nil {
			return err
		}
		t.singletons[last] = nil
		t.singletons = t.singletons[:last]
	}
	t.cache = nil
	return nil
}
