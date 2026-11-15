package ddnf

// Source: src/util/hashtable.h core_hashtable, as ddnf_node::ddnf_nodes, a
// ptr_hashtable<ddnf_node, hash, eq> whose hash is tbv_manager::hash of the
// node's tbv and whose equality is tbv equality. Its iteration order is the
// order compile_eq writes its disjuncts in, so it is emulated: open
// addressing with linear probing from hash & (capacity - 1), doubling (and
// rehashing in slot order) when (size + deleted) * 4 > capacity * 3. Entries
// are only inserted.

const nodeHashtableInitialCapacity = 8 // DEFAULT_HASHTABLE_INITIAL_CAPACITY

type nodeHashEntry struct {
	used bool
	node *Node
	hash uint32
}

type nodeHashtable struct {
	table []nodeHashEntry
	size  int
}

func newNodeHashtable() *nodeHashtable {
	return &nodeHashtable{table: make([]nodeHashEntry, nodeHashtableInitialCapacity)}
}

func moveNodeTable(source, target []nodeHashEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if !entry.used {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if !target[i].used {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if !target[i].used {
				target[i], placed = entry, true
			}
		}
	}
}

// insert is core_hashtable::insert(&n).
func (h *nodeHashtable) insert(c *Core, node *Node) error {
	if h.size<<2 > len(h.table)*3 {
		bigger := make([]nodeHashEntry, len(h.table)<<1)
		moveNodeTable(h.table, bigger)
		h.table = bigger
	}
	hash, err := c.vectors.Hash(node.vector)
	if err != nil {
		return err
	}
	mask := uint32(len(h.table) - 1)
	begin := int(hash & mask)
	for _, r := range [2][2]int{{begin, len(h.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &h.table[i]
			if curr.used {
				if curr.hash == hash {
					equal, err := c.vectors.Equals(curr.node.vector, node.vector)
					if err != nil {
						return err
					}
					if equal {
						return nil
					}
				}
				continue
			}
			*curr = nodeHashEntry{used: true, node: node, hash: hash}
			h.size++
			return nil
		}
	}
	return ErrCore // UNREACHABLE: the table always has a free slot
}

// members lists the nodes in slot order (the table's iterator).
func (h *nodeHashtable) members() []*Node {
	if h == nil {
		return nil
	}
	out := make([]*Node, 0, h.size)
	for _, entry := range h.table {
		if entry.used {
			out = append(out, entry.node)
		}
	}
	return out
}
