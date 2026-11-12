package base

// Source: src/muz/base/dl_rule_set.cpp rule_dependencies::populate(rule_set),
// populate(rule), ensure_key, insert, and rule_stratifier::traverse, process
// (Gabow's strongly connected components followed by the topological order
// of the component graph). The source iterates obj_map and obj_hashtable
// tables, so the predicate order is their table order (ast.ObjMap).

import "github.com/glycerine/g3/pkg/ast"

// sourceDependencies is rule_dependencies: m_data's key layout and, per
// key, its item_set of masters.
type sourceDependencies struct {
	manager *ast.ASTManager
	keys    *ast.ObjMap
	sets    map[ast.ASTHandle]*ast.ObjMap
}

func newSourceDependencies(manager *ast.ASTManager) *sourceDependencies {
	return &sourceDependencies{manager: manager, keys: ast.NewObjMap(manager), sets: make(map[ast.ASTHandle]*ast.ObjMap)}
}

// ensureKey is rule_dependencies::ensure_key.
func (d *sourceDependencies) ensureKey(pred ast.ASTHandle) (*ast.ObjMap, error) {
	if _, err := d.keys.InsertIfNotThere(pred); err != nil {
		return nil, err
	}
	set := d.sets[pred]
	if set == nil {
		set = ast.NewObjMap(d.manager)
		d.sets[pred] = set
	}
	return set, nil
}

// masters is get_deps(f): the item_set's entries in table order, empty when
// f is not a key.
func (d *sourceDependencies) masters(pred ast.ASTHandle) []ast.ObjMapEntry {
	set := d.sets[pred]
	if set == nil {
		return nil
	}
	return set.Entries()
}

// populateRule is rule_dependencies::populate(rule const*): the head is a
// key; the tails are walked last pushed first, and every predicate
// application met is a master of the head and a key of its own.
func (s *RuleSet) populateRuleDependencies(d *sourceDependencies, rule *Rule) error {
	state, err := rule.open()
	if err != nil {
		return err
	}
	manager := s.manager.manager
	head, err := manager.GetDecl(state.head)
	if err != nil {
		return err
	}
	set, err := d.ensureKey(head)
	if err != nil {
		return err
	}
	todo := make([]ast.ASTHandle, 0, len(state.tail))
	for _, tail := range state.tail {
		todo = append(todo, tail.handle)
	}
	visited := make(map[ast.ASTHandle]bool)
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if visited[current] {
			continue
		}
		visited[current] = true
		kind, err := manager.GetKind(current)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			declaration, err := manager.GetDecl(current)
			if err != nil {
				return err
			}
			if _, predicate := s.manager.predicates[declaration]; predicate {
				if _, err = set.InsertIfNotThere(declaration); err != nil {
					return err
				}
				if _, err = d.ensureKey(declaration); err != nil {
					return err
				}
			}
			count, err := manager.GetNumArgs(current)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := manager.GetArg(current, i)
				if err != nil {
					return err
				}
				todo = append(todo, argument)
			}
		case ast.ASTQuantifier:
			body, err := manager.GetQuantifierBody(current)
			if err != nil {
				return err
			}
			todo = append(todo, body)
		}
	}
	return nil
}

// sourceStratifier is rule_stratifier.
type sourceStratifier struct {
	manager       *ast.ASTManager
	deps          *sourceDependencies
	preorder      map[ast.ASTHandle]uint32
	componentNums map[ast.ASTHandle]int
	stackS        []ast.ASTHandle
	stackP        []ast.ASTHandle
	components    []*ast.ObjMap
	nextPreorder  uint32
	firstPreorder uint32
}

// traverse is rule_stratifier::traverse.
func (t *sourceStratifier) traverse(el ast.ASTHandle) error {
	if pNum, found := t.preorder[el]; found {
		if pNum < t.firstPreorder {
			return nil // traversed in a previous sweep
		}
		if _, assigned := t.componentNums[el]; assigned {
			return nil
		}
		for len(t.stackP) != 0 {
			onStack := t.preorder[t.stackP[len(t.stackP)-1]]
			if onStack <= pNum {
				break
			}
			t.stackP = t.stackP[:len(t.stackP)-1]
		}
		return nil
	}
	pNum := t.nextPreorder
	t.nextPreorder++
	t.preorder[el] = pNum
	t.stackS = append(t.stackS, el)
	t.stackP = append(t.stackP, el)
	for _, child := range t.deps.masters(el) {
		if err := t.traverse(child.Key); err != nil {
			return err
		}
	}
	if el == t.stackP[len(t.stackP)-1] {
		compNum := len(t.components)
		comp := ast.NewObjMap(t.manager)
		t.components = append(t.components, comp)
		for {
			last := len(t.stackS) - 1
			member := t.stackS[last]
			t.stackS = t.stackS[:last]
			if _, err := comp.InsertIfNotThere(member); err != nil {
				return err
			}
			t.componentNums[member] = compNum
			if member == el {
				break
			}
		}
		t.stackP = t.stackP[:len(t.stackP)-1]
	}
	return nil
}

// process is rule_stratifier::process: the strata are the components in the
// order masters first.
func (t *sourceStratifier) process() ([][]ast.ASTHandle, error) {
	if t.deps.keys.Len() == 0 {
		return nil, nil
	}
	keys := t.deps.keys.Entries()
	for _, entry := range keys {
		t.firstPreorder = t.nextPreorder
		if err := t.traverse(entry.Key); err != nil {
			return nil, err
		}
	}
	inDegrees := make([]int, len(t.components))
	for _, entry := range keys {
		elComp := t.componentNums[entry.Key]
		for _, target := range t.deps.masters(entry.Key) {
			if tgtComp := t.componentNums[target.Key]; tgtComp != elComp {
				inDegrees[tgtComp]++
			}
		}
	}
	live := append([]*ast.ObjMap(nil), t.components...)
	var strats []*ast.ObjMap
	for i := range live {
		if inDegrees[i] == 0 {
			strats = append(strats, live[i])
			live[i] = nil
		}
	}
	for index := 0; index < len(strats); index++ {
		for _, member := range strats[index].Entries() {
			for _, target := range t.deps.masters(member.Key) {
				tgtComp := t.componentNums[target.Key]
				if live[tgtComp] == nil {
					continue // intra-component, or already ordered
				}
				inDegrees[tgtComp]--
				if inDegrees[tgtComp] == 0 {
					strats = append(strats, live[tgtComp])
					live[tgtComp] = nil
				}
			}
		}
	}
	// reverse the strats array, so that only the later components depend on
	// earlier ones
	result := make([][]ast.ASTHandle, 0, len(strats))
	for i := len(strats) - 1; i >= 0; i-- {
		entries := strats[i].Entries()
		stratum := make([]ast.ASTHandle, len(entries))
		for j := range entries {
			stratum[j] = entries[j].Key
		}
		result = append(result, stratum)
	}
	return result, nil
}

// SourceStrata is rule_set::close followed by get_stratifier().get_strats():
// the predicate dependencies are populated per head predicate in
// m_head2rules table order (each head's rules in insertion order), and the
// strongly connected components are returned masters first, each in its
// GroupedHeads lists the head declarations in m_head2rules table order
// (rule_set::begin_grouped_rules).
func (s *RuleSet) GroupedHeads() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	var heads []ast.ASTHandle
	if s.headTable != nil {
		for _, entry := range s.headTable.Entries() {
			heads = append(heads, entry.Key)
		}
	}
	return heads, nil
}

// item_set table order.
func (s *RuleSet) SourceStrata() ([][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	deps := newSourceDependencies(s.manager.manager)
	if s.headTable != nil {
		for _, entry := range s.headTable.Entries() {
			for _, rule := range s.byHead[entry.Key] {
				if err := s.populateRuleDependencies(deps, rule); err != nil {
					return nil, err
				}
			}
		}
	}
	stratifier := &sourceStratifier{manager: s.manager.manager, deps: deps, preorder: make(map[ast.ASTHandle]uint32), componentNums: make(map[ast.ASTHandle]int)}
	return stratifier.process()
}
