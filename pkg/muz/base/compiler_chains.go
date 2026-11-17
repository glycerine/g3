package base

// Source: src/muz/rel/dl_compiler.cpp compiler::detect_chains and
// cycle_breaker; src/muz/base/dl_rule_set.cpp rule_dependencies' copy
// constructors, restrict, remove and sort_deps. The source walks the
// obj_map and obj_hashtable tables, so the order is their table order
// (ast.ObjMap).

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// copyDependencies is rule_dependencies(o, false): m_data.insert(pred,
// alloc(item_set, orig_items)) in o's table order.
func copyDependencies(o *sourceDependencies) (*sourceDependencies, error) {
	d := newSourceDependencies(o.manager)
	for _, entry := range o.keys.Entries() {
		if err := d.keys.Insert(entry.Key, [2]ast.ASTHandle{}); err != nil {
			return nil, err
		}
		d.sets[entry.Key] = o.sets[entry.Key].Copy()
	}
	return d, nil
}

// reversedDependencies is rule_dependencies(o, true).
func reversedDependencies(o *sourceDependencies) (*sourceDependencies, error) {
	d := newSourceDependencies(o.manager)
	for _, entry := range o.keys.Entries() {
		if _, err := d.ensureKey(entry.Key); err != nil {
			return nil, err
		}
		for _, master := range o.masters(entry.Key) {
			set, err := d.ensureKey(master.Key)
			if err != nil {
				return nil, err
			}
			if _, err = set.InsertIfNotThere(entry.Key); err != nil {
				return nil, err
			}
		}
	}
	return d, nil
}

// removeKey is remove_m_data_entry.
func (d *sourceDependencies) removeKey(pred ast.ASTHandle) error {
	delete(d.sets, pred)
	return d.keys.Erase(pred)
}

// restrict is rule_dependencies::restrict(allowed).
func (d *sourceDependencies) restrict(allowed map[ast.ASTHandle]bool) error {
	var toRemove []ast.ASTHandle
	for _, entry := range d.keys.Entries() {
		if !allowed[entry.Key] {
			toRemove = append(toRemove, entry.Key)
			continue
		}
		// set_intersection(itms, allowed): removed last found first.
		set := d.sets[entry.Key]
		var drop []ast.ASTHandle
		for _, item := range set.Entries() {
			if !allowed[item.Key] {
				drop = append(drop, item.Key)
			}
		}
		for i := len(drop) - 1; i >= 0; i-- {
			if err := set.Erase(drop[i]); err != nil {
				return err
			}
		}
	}
	for _, pred := range toRemove {
		if err := d.removeKey(pred); err != nil {
			return err
		}
	}
	return nil
}

// removeSet is rule_dependencies::remove(to_remove).
func (d *sourceDependencies) removeSet(toRemove []ast.ASTHandle) error {
	for _, pred := range toRemove {
		if err := d.removeKey(pred); err != nil {
			return err
		}
	}
	for _, entry := range d.keys.Entries() {
		set := d.sets[entry.Key]
		for _, pred := range toRemove {
			if err := set.Erase(pred); err != nil {
				return err
			}
		}
	}
	return nil
}

// sortDeps is rule_dependencies::sort_deps(res) of an empty res.
func (d *sourceDependencies) sortDeps() ([]ast.ASTHandle, bool, error) {
	reversed, err := reversedDependencies(d)
	if err != nil {
		return nil, false, err
	}
	var res []ast.ASTHandle
	degs := make(map[ast.ASTHandle]int)
	for _, entry := range d.keys.Entries() {
		if deg := len(d.masters(entry.Key)); deg == 0 {
			res = append(res, entry.Key)
		} else {
			degs[entry.Key] = deg
		}
	}
	for current := 0; current < len(res); current++ {
		for _, child := range reversed.masters(res[current]) {
			degs[child.Key]--
			if degs[child.Key] == 0 {
				res = append(res, child.Key)
			}
		}
	}
	if len(res) < d.keys.Len() {
		return nil, false, nil
	}
	return res, true, nil
}

// cycleBreaker is dl_compiler's cycle_breaker: removed is m_removed (an
// item_set).
type cycleBreaker struct {
	deps         *sourceDependencies
	removed      *ast.ObjMap
	stackContent map[ast.ASTHandle]bool
	visited      map[ast.ASTHandle]bool
}

func (c *cycleBreaker) traverse(v ast.ASTHandle) error {
	if c.visited[v] {
		return nil
	}
	if found, err := c.removed.Contains(v); err != nil || found {
		return err
	}
	c.stackContent[v] = true
	c.visited[v] = true
	for _, dep := range c.deps.masters(v) {
		if c.stackContent[dep.Key] {
			//TODO: find the best vertex to remove in the cycle
			if _, err := c.removed.InsertIfNotThere(v); err != nil {
				return err
			}
			break
		}
		if err := c.traverse(dep.Key); err != nil {
			return err
		}
	}
	c.stackContent[v] = false
	return nil
}

// DetectChains is compiler::detect_chains(preds, ordered_preds,
// global_deltas) over the rule set's dependencies: the predicates of preds
// in the order the compiler evaluates them (sort_deps of the dependencies
// restricted to preds, a vertex of each cycle removed and put last).
func (s *RuleSet) DetectChains(preds []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if !s.finalized || s.sourceDeps == nil {
		return nil, fmt.Errorf("muz/base: finalize rule set before detecting chains")
	}
	allowed := make(map[ast.ASTHandle]bool, len(preds))
	for _, pred := range preds {
		allowed[pred] = true
	}
	deps, err := copyDependencies(s.sourceDeps)
	if err != nil {
		return nil, err
	}
	if err = deps.restrict(allowed); err != nil {
		return nil, err
	}
	breaker := &cycleBreaker{deps: deps, removed: ast.NewObjMap(s.manager.manager), stackContent: make(map[ast.ASTHandle]bool), visited: make(map[ast.ASTHandle]bool)}
	for _, entry := range deps.keys.Entries() {
		if err = breaker.traverse(entry.Key); err != nil {
			return nil, err
		}
	}
	removed := make([]ast.ASTHandle, 0, breaker.removed.Len())
	for _, entry := range breaker.removed.Entries() {
		removed = append(removed, entry.Key)
	}
	if err = deps.removeSet(removed); err != nil {
		return nil, err
	}
	ordered, ok, err := deps.sortDeps()
	if err != nil {
		return nil, err
	}
	if !ok {
		// VERIFY(deps.sort_deps(ordered_preds))
		return nil, fmt.Errorf("muz/base: detect_chains: dependencies not acyclic")
	}
	//the predicates that were removed to get acyclic induced subgraph are put last
	//so that all their local input deltas are already populated
	return append(ordered, removed...), nil
}

// CopiedStratum is func_decl_set global_deltas(head_preds) of the index-th
// stratum: the copy constructor's copy_table reinserts the item_set's keys
// in table order into a table of its capacity, which can reorder keys an
// insertion wrapped around the table's end.
func (s *RuleSet) CopiedStratum(index int) ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if !s.finalized || index < 0 || index >= len(s.strats) || index >= len(s.stratCapacities) {
		return nil, fmt.Errorf("muz/base: no stratum %d", index)
	}
	copied := ast.NewObjMapCapacity(s.manager.manager, s.stratCapacities[index])
	for _, pred := range s.strats[index] {
		if _, err := copied.InsertIfNotThere(pred); err != nil {
			return nil, err
		}
	}
	entries := copied.Entries()
	result := make([]ast.ASTHandle, len(entries))
	for i := range entries {
		result[i] = entries[i].Key
	}
	return result, nil
}
