package rel

// Source: src/muz/rel/dl_lazy_table.h/.cpp lazy_table_ref hierarchy and
// force/eval implementation. This is the concrete retained HashTable
// specialization; interpreted AST filtering remains with its unported table
// condition evaluator.

import (
	"errors"
	"fmt"
)

var ErrLazyTable = errors.New("muz/rel: invalid lazy table operation")

type LazyTableKind uint8

const (
	LazyTableBase LazyTableKind = iota
	LazyTableJoin
	LazyTableProject
	LazyTableRename
	LazyTableFilterIdentical
	LazyTableFilterEqual
	LazyTableFilterByNegation
)

type LazyTableStatistics struct {
	Forces             uint32
	Joins              uint32
	JoinProjects       uint32
	Projects           uint32
	Renames            uint32
	EqualFilters       uint32
	IdenticalFilters   uint32
	NegationFilters    uint32
	SelectEqualProject uint32
}

// LazyTableEngine is the concrete counterpart of lazy_table_plugin. Tables
// from one engine may share deferred nodes and a deterministic statistics log.
type LazyTableEngine struct {
	stats  LazyTableStatistics
	closed bool
}

func NewLazyTableEngine() *LazyTableEngine { return &LazyTableEngine{} }

func (e *LazyTableEngine) open() error {
	if e == nil || e.closed {
		return ErrLazyTable
	}
	return nil
}

func (e *LazyTableEngine) Statistics() LazyTableStatistics {
	if e == nil {
		return LazyTableStatistics{}
	}
	return e.stats
}

func (e *LazyTableEngine) ResetStatistics() {
	if e != nil && !e.closed {
		e.stats = LazyTableStatistics{}
	}
}

func (e *LazyTableEngine) Close() {
	if e == nil {
		return
	}
	e.closed = true
}

type lazyTableNode struct {
	kind      LazyTableKind
	signature Signature
	evaluated *HashTable
	left      *lazyTableNode
	right     *lazyTableNode
	columns1  []int
	columns2  []int
	value     Element
	column    int
}

type LazyTable struct {
	engine *LazyTableEngine
	root   *lazyTableNode
	closed bool
}

func (e *LazyTableEngine) New(signature Signature) (*LazyTable, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	table := NewHashTable(signature)
	return &LazyTable{engine: e, root: &lazyTableNode{kind: LazyTableBase, signature: Signature{domains: signature.Domains()}, evaluated: table}}, nil
}

// AdoptLazyTable transfers ownership of table to a forced base node.
func (e *LazyTableEngine) Adopt(table *HashTable) (*LazyTable, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if table == nil || table.open() != nil {
		return nil, ErrLazyTable
	}
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	return &LazyTable{engine: e, root: &lazyTableNode{kind: LazyTableBase, signature: signature, evaluated: table}}, nil
}

func (t *LazyTable) open() error {
	if t == nil || t.closed || t.root == nil || t.engine == nil || t.engine.open() != nil {
		return ErrLazyTable
	}
	return nil
}

func (t *LazyTable) Close() error {
	if t == nil || t.closed {
		return nil
	}
	t.closed = true
	t.root = nil
	t.engine = nil
	return nil
}

func (t *LazyTable) Kind() LazyTableKind {
	if t.open() != nil {
		return LazyTableBase
	}
	return t.root.kind
}

func (t *LazyTable) Forced() bool { return t.open() == nil && t.root.evaluated != nil }

func (t *LazyTable) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: t.root.signature.Domains()}, nil
}

func (e *LazyTableEngine) evaluate(node *lazyTableNode) (*HashTable, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	if node == nil {
		return nil, ErrLazyTable
	}
	if node.evaluated != nil {
		return node.evaluated, nil
	}
	e.stats.Forces++
	var result *HashTable
	var err error
	switch node.kind {
	case LazyTableJoin:
		left, leftErr := e.evaluate(node.left)
		if leftErr != nil {
			return nil, leftErr
		}
		right, rightErr := e.evaluate(node.right)
		if rightErr != nil {
			return nil, rightErr
		}
		e.stats.Joins++
		result, err = left.Join(right, node.columns1, node.columns2)
	case LazyTableProject:
		if node.left != nil && node.left.kind == LazyTableJoin && node.left.evaluated == nil {
			join := node.left
			left, leftErr := e.evaluate(join.left)
			if leftErr != nil {
				return nil, leftErr
			}
			right, rightErr := e.evaluate(join.right)
			if rightErr != nil {
				return nil, rightErr
			}
			e.stats.JoinProjects++
			var joined *HashTable
			joined, err = left.Join(right, join.columns1, join.columns2)
			if err == nil {
				result, err = joined.Project(node.columns1)
			}
		} else if node.left != nil && node.left.kind == LazyTableFilterEqual && node.left.evaluated == nil && len(node.columns1) == 1 && node.columns1[0] == node.left.column {
			filter := node.left
			source, sourceErr := e.evaluate(filter.left)
			if sourceErr != nil {
				return nil, sourceErr
			}
			e.stats.SelectEqualProject++
			result, err = source.SelectEqualAndProject(filter.value, filter.column)
		} else {
			source, sourceErr := e.evaluate(node.left)
			if sourceErr != nil {
				return nil, sourceErr
			}
			e.stats.Projects++
			result, err = source.Project(node.columns1)
		}
	case LazyTableRename:
		source, sourceErr := e.evaluate(node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		e.stats.Renames++
		result, err = source.RenameCycle(node.columns1)
	case LazyTableFilterIdentical:
		source, sourceErr := e.evaluate(node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		result, err = source.Clone()
		if err == nil {
			e.stats.IdenticalFilters++
			err = result.FilterIdentical(node.columns1)
		}
	case LazyTableFilterEqual:
		source, sourceErr := e.evaluate(node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		result, err = source.Clone()
		if err == nil {
			e.stats.EqualFilters++
			err = result.FilterEqual(node.value, node.column)
		}
	case LazyTableFilterByNegation:
		target, targetErr := e.evaluate(node.left)
		if targetErr != nil {
			return nil, targetErr
		}
		source, sourceErr := e.evaluate(node.right)
		if sourceErr != nil {
			return nil, sourceErr
		}
		result, err = target.Clone()
		if err == nil {
			e.stats.NegationFilters++
			err = result.FilterByNegation(source, node.columns1, node.columns2)
		}
	default:
		return nil, fmt.Errorf("%w: unsupported deferred node kind", ErrLazyTable)
	}
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrLazyTable
	}
	node.evaluated = result
	return result, nil
}

// Eval forces and memoizes the operation DAG and returns a borrowed table.
func (t *LazyTable) Eval() (*HashTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t.engine.evaluate(t.root)
}

func (t *LazyTable) Empty() (bool, error) {
	table, err := t.Eval()
	if err != nil {
		return false, err
	}
	length, err := table.Len()
	return length == 0, err
}

func (t *LazyTable) Contains(fact Fact) (bool, error) {
	table, err := t.Eval()
	if err != nil {
		return false, err
	}
	return table.Contains(fact)
}

func (t *LazyTable) Add(fact Fact) (bool, error) {
	table, err := t.Eval()
	if err != nil {
		return false, err
	}
	return table.Add(fact)
}

func (t *LazyTable) Remove(fact Fact) (bool, error) {
	table, err := t.Eval()
	if err != nil {
		return false, err
	}
	return table.Remove(fact)
}

func (t *LazyTable) Facts() ([]Fact, error) {
	table, err := t.Eval()
	if err != nil {
		return nil, err
	}
	return table.Facts()
}

func (t *LazyTable) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	t.root = &lazyTableNode{kind: LazyTableBase, signature: Signature{domains: t.root.signature.Domains()}, evaluated: NewHashTable(t.root.signature)}
	return nil
}

func (t *LazyTable) Clone() (*LazyTable, error) {
	table, err := t.Eval()
	if err != nil {
		return nil, err
	}
	copy, err := table.Clone()
	if err != nil {
		return nil, err
	}
	return t.engine.Adopt(copy)
}

func (t *LazyTable) Complement() (*LazyTable, error) {
	table, err := t.Eval()
	if err != nil {
		return nil, err
	}
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	if signature.Len() > 1 {
		return nil, fmt.Errorf("%w: complement requires compiler decomposition", ErrLazyTable)
	}
	result := NewHashTable(signature)
	if signature.Len() == 0 {
		length, lengthErr := table.Len()
		if lengthErr != nil {
			return nil, lengthErr
		}
		if length == 0 {
			_, err = result.Add(Fact{})
		}
	} else {
		for value := uint64(0); value < signature.domains[0]; value++ {
			fact := Fact{Element(value)}
			found, containsErr := table.Contains(fact)
			if containsErr != nil {
				return nil, containsErr
			}
			if !found {
				_, err = result.Add(fact)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return t.engine.Adopt(result)
}
