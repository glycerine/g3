package ast

// SetNextExprID implements ast_manager::set_next_expr_id for the SMT-LIB
// debug command, skipping IDs present in the canonical AST table.
func (m *ASTManager) SetNextExprID(id uint32) error {
	if err := m.lock(); err != nil {
		return err
	}
	defer m.mu.Unlock()
	for {
		id = m.ExprIDs.SetNextID(id)
		used := false
		for _, node := range m.table.entries.All() {
			if m.nodeAt(node.slot).id == id {
				used = true
				break
			}
		}
		if !used {
			return nil
		}
		id++
	}
}
