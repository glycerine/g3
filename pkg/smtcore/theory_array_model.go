package smtcore

// Source: theory_array_base model default partition, collect_selects,
// propagate_selects and array_value_proc preparation.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/hash"
)

// smtArraySelectSet is theory_array_base::select_set, a
// ptr_hashtable<enode, sel_hash, sel_eq> (src/util/hashtable.h
// core_hashtable). Model construction iterates it, so the array entries
// follow its table order: open addressing on sel_hash, which hashes the
// roots of the select's index arguments. Entries are never removed.
type smtArraySelectSet struct {
	table []smtArraySelectEntry
	size  int
}

type smtArraySelectEntry struct {
	read *SMTEnode
	hash uint32
}

func newSMTArraySelectSet() *smtArraySelectSet {
	return &smtArraySelectSet{table: make([]smtArraySelectEntry, 8)} // DEFAULT_HASHTABLE_INITIAL_CAPACITY
}

// smtArraySelectHash is sel_hash: get_composite_hash with a zero kind hash
// over the root hashes of the index arguments.
func smtArraySelectHash(read *SMTEnode) (uint32, error) {
	n := uint32(len(read.arguments) - 1)
	if n == 0 {
		return 0, ast.ErrASTDomain
	}
	var failure error
	h := hash.Composite(0, n, func(i uint32) uint32 {
		v, err := read.arguments[i+1].root.Hash()
		if err != nil && failure == nil {
			failure = err
		}
		return v
	})
	return h, failure
}

func smtArraySelectMove(source, target []smtArraySelectEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if entry.read == nil {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if target[i].read == nil {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if target[i].read == nil {
				target[i], placed = entry, true
			}
		}
	}
}

// find is core_hashtable::find under sel_eq.
func (s *smtArraySelectSet) find(read *SMTEnode) (*SMTEnode, error) {
	h, err := smtArraySelectHash(read)
	if err != nil {
		return nil, err
	}
	mask := uint32(len(s.table) - 1)
	begin := int(h & mask)
	for k := 0; k < len(s.table); k++ {
		curr := &s.table[(begin+k)&int(mask)]
		if curr.read == nil {
			return nil, nil
		}
		if curr.hash == h && sameArrayModelIndices(curr.read, read) {
			return curr.read, nil
		}
	}
	return nil, nil
}

// insert is core_hashtable::insert: an equal entry has its data replaced.
func (s *smtArraySelectSet) insert(read *SMTEnode) error {
	if s.size<<2 > len(s.table)*3 {
		target := make([]smtArraySelectEntry, len(s.table)<<1)
		smtArraySelectMove(s.table, target)
		s.table = target
	}
	h, err := smtArraySelectHash(read)
	if err != nil {
		return err
	}
	mask := uint32(len(s.table) - 1)
	begin := int(h & mask)
	for k := 0; k < len(s.table); k++ {
		curr := &s.table[(begin+k)&int(mask)]
		if curr.read == nil {
			*curr = smtArraySelectEntry{read: read, hash: h}
			s.size++
			return nil
		}
		if curr.hash == h && sameArrayModelIndices(curr.read, read) {
			curr.read = read
			return nil
		}
	}
	return ast.ErrASTDomain
}

// entries lists the set in table (iteration) order.
func (s *smtArraySelectSet) entries() []*SMTEnode {
	if s == nil {
		return nil
	}
	result := make([]*SMTEnode, 0, s.size)
	for _, entry := range s.table {
		if entry.read != nil {
			result = append(result, entry.read)
		}
	}
	return result
}

type smtArrayModelElse struct {
	expression ast.ASTHandle
	fresh      *SMTExtraFreshValue
}

type smtArrayModelData struct {
	parents       []int
	defaults      []*SMTEnode
	elseValues    []smtArrayModelElse
	unspecified   bool
	selects       map[*SMTEnode]*smtArraySelectSet
	selectDomains []*SMTEnode
}

func (d *smtArrayModelData) find(variable int) int {
	root := variable
	for d.parents[root] >= 0 {
		root = d.parents[root]
	}
	for d.parents[variable] >= 0 {
		next := d.parents[variable]
		d.parents[variable] = root
		variable = next
	}
	return root
}

func (t *SMTArrayTheory) mergeArrayModelData(d *smtArrayModelData, first, second int) error {
	first, second = d.find(first), d.find(second)
	if first == second {
		return nil
	}
	if d.parents[first] > d.parents[second] {
		first, second = second, first
	}
	d.parents[first] += d.parents[second]
	d.parents[second] = first
	if d.defaults[first] == nil {
		d.defaults[first] = d.defaults[second]
	}
	return t.emitArrayTrace("array", "mg_merge", 568, func() bool { return d.defaults[second] != nil }, func(out io.Writer) error {
		if err := displayArrayTraceExpression(t.state.manager, out, d.defaults[second].root.owner); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(t.state.manager, out, d.defaults[first].root.owner); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTArrayTheory) setArrayModelDefault(d *smtArrayModelData, variable int, node *SMTEnode) error {
	if node == nil {
		return ast.ErrASTDomain
	}
	if err := t.emitArrayTrace("array", "set_default", 521, nil, func(out io.Writer) error {
		if _, err := fmt.Fprintf(out, "set default: %d ", variable); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(t.state.manager, out, node.owner); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	root := d.find(variable)
	if d.defaults[root] == nil {
		d.defaults[root] = node
	}
	return nil
}

func (t *SMTArrayTheory) modelRepresentative(node *SMTEnode) (int, error) {
	variable := node.root.TheoryVar(t.FamilyID())
	if variable < 0 || int(variable) >= len(t.variables) {
		return 0, ast.ErrASTDomain
	}
	return int(variable), nil
}

func (t *SMTArrayTheory) prepareArrayModel(generator *SMTModelGenerator) error {
	d := &smtArrayModelData{
		parents: make([]int, len(t.variables)), defaults: make([]*SMTEnode, len(t.variables)),
		elseValues: make([]smtArrayModelElse, len(t.variables)), unspecified: true,
		selects: make(map[*SMTEnode]*smtArraySelectSet),
	}
	t.modelData = d
	for i := range d.parents {
		d.parents[i] = -1
	}
	for _, node := range t.variables {
		if !generator.context.isRelevant(node.owner) {
			continue
		}
		op, array, err := t.arrayOperation(node.owner)
		if err != nil {
			return err
		}
		if array && (op == ast.ASTArrayOpStore || op == ast.ASTArrayOpConst || op == ast.ASTArrayOpDefault) {
			d.unspecified = false
			break
		}
	}
	if !d.unspecified {
		for variable, node := range t.variables {
			if !generator.context.isRelevant(node.owner) {
				continue
			}
			representative, err := t.modelRepresentative(node)
			if err != nil {
				return err
			}
			if err = t.mergeArrayModelData(d, variable, representative); err != nil {
				return err
			}
			op, array, err := t.arrayOperation(node.owner)
			if err != nil {
				return err
			}
			if !array {
				continue
			}
			switch op {
			case ast.ASTArrayOpStore:
				baseVariable := node.arguments[0].TheoryVar(t.FamilyID())
				if baseVariable < 0 {
					return ast.ErrASTDomain
				}
				base, err := t.modelRepresentative(node.arguments[0])
				if err != nil {
					return err
				}
				if err = t.mergeArrayModelData(d, variable, base); err != nil {
					return err
				}
				if err = t.emitArrayTrace("array", "collect_defaults", 646, nil, func(out io.Writer) error {
					if _, err := io.WriteString(out, "merge: "); err != nil {
						return err
					}
					if err := displayArrayTraceExpression(t.state.manager, out, node.owner); err != nil {
						return err
					}
					_, err := fmt.Fprintf(out, " %d %d\n", variable, baseVariable)
					return err
				}); err != nil {
					return err
				}
			case ast.ASTArrayOpConst:
				if err = t.setArrayModelDefault(d, variable, node.arguments[0]); err != nil {
					return err
				}
			case ast.ASTArrayOpDefault:
				base := node.arguments[0].TheoryVar(t.FamilyID())
				if base < 0 {
					return ast.ErrASTDomain
				}
				if err = t.setArrayModelDefault(d, int(base), node); err != nil {
					return err
				}
			}
		}
	}
	for variable, node := range t.variables {
		representative, err := t.modelRepresentative(node)
		if err != nil {
			return err
		}
		if variable != representative || !generator.context.isRelevant(node.root.owner) {
			continue
		}
		root := node.root
		for _, parent := range root.parents {
			op, array, err := t.arrayOperation(parent.owner)
			if err != nil {
				return err
			}
			if !array || op != ast.ASTArrayOpSelect || !parent.IsCongruenceRoot() || !generator.context.isRelevant(parent.owner) || parent.arguments[0].root != root {
				continue
			}
			if _, err = d.addSelect(root, parent); err != nil {
				return err
			}
		}
	}
	// Source first traverses the original select domains, then processes
	// newly reached store-parent domains through the explicit worklist.
	var todo []smtArrayEnodePair
	domains := append([]*SMTEnode(nil), d.selectDomains...)
	for _, root := range domains {
		for _, read := range d.selects[root].entries() {
			if err := t.propagateModelSelect(root, read, generator, &todo); err != nil {
				return err
			}
		}
	}
	for head := 0; head < len(todo); head++ {
		pair := todo[head]
		if err := t.propagateModelSelect(pair.first, pair.second, generator, &todo); err != nil {
			return err
		}
	}
	return nil
}

// selectSet is get_select_set: the root's set, created and recorded in the
// domain list on first use.
func (d *smtArrayModelData) selectSet(root *SMTEnode) *smtArraySelectSet {
	set, exists := d.selects[root]
	if !exists {
		set = newSMTArraySelectSet()
		d.selects[root] = set
		d.selectDomains = append(d.selectDomains, root)
	}
	return set
}

func (d *smtArrayModelData) addSelect(root, read *SMTEnode) (bool, error) {
	set := d.selectSet(root)
	previous, err := set.find(read)
	if err != nil {
		return false, err
	}
	if previous != nil && previous.root != read.root {
		return false, fmt.Errorf("array model repeated index has distinct value roots %v and %v: %w", previous.owner, read.owner, ast.ErrASTDomain)
	}
	return previous == nil, set.insert(read)
}

func sameArrayModelIndices(first, second *SMTEnode) bool {
	if len(first.arguments) != len(second.arguments) {
		return false
	}
	for i := 1; i < len(first.arguments); i++ {
		if first.arguments[i].root != second.arguments[i].root {
			return false
		}
	}
	return true
}

func (t *SMTArrayTheory) propagateModelSelect(root, read *SMTEnode, generator *SMTModelGenerator, todo *[]smtArrayEnodePair) error {
	if !generator.context.isRelevant(root.owner) {
		return nil
	}
	for _, parent := range root.parents {
		op, array, err := t.arrayOperation(parent.owner)
		if err != nil {
			return err
		}
		if !array || op != ast.ASTArrayOpStore || !generator.context.isRelevant(parent.owner) || parent.arguments[0].root != root {
			continue
		}
		if len(parent.arguments) != len(read.arguments)+1 {
			return ast.ErrASTDomain
		}
		previous, err := t.modelData.selectSet(parent.root).find(read)
		if err != nil {
			return err
		}
		// Native upward propagation keeps an existing store-side entry. It
		// does not replace it with the base-side read for the same indices.
		if previous != nil {
			continue
		}
		overwritten := true
		for i := 1; i < len(read.arguments); i++ {
			if read.arguments[i].root != parent.arguments[i].root {
				overwritten = false
				break
			}
		}
		if overwritten {
			continue
		}
		added, err := t.modelData.addSelect(parent.root, read)
		if err != nil {
			return err
		}
		if added {
			*todo = append(*todo, smtArrayEnodePair{parent.root, read})
		}
	}
	return nil
}

func (t *SMTArrayTheory) FinalizeModel(*SMTModelGenerator) error {
	t.modelData = nil
	return nil
}
