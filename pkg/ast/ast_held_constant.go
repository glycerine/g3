package ast

// TryIncRefHeldUninterpretedConstant acquires an additional reference to a
// nullary uninterpreted application. The caller must already hold h live in
// this manager. Use the regular path for other nodes and whenever the manager
// has pending work or checks are enabled.
func (m *ASTManager) TryIncRefHeldUninterpretedConstant(h ASTHandle) bool {
	if astValidateHandles || h.IsNull() || !m.lockFree() {
		return false
	}
	slot := &m.nodes[(h.slot-1)/astPageSize].slots[(h.slot-1)%astPageSize]
	if slot.node.kind != ASTApp || !slot.app.declCached || slot.app.count != 0 || slot.app.declFamily != NullFamilyID {
		return false
	}
	slot.node.rc++
	return true
}
