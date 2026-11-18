package ast

// BitVectorConcatArguments exposes the concrete operand list for a BV concat.
func (m *ASTManager) BitVectorConcatArguments(h ASTHandle) (arguments []ASTHandle, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		err = e
		return
	}
	matched, err = m.bitVectorAppKind(r, 37)
	if err != nil || !matched {
		return
	}
	app := m.appAt(r.slot)
	arguments = make([]ASTHandle, app.count)
	for i := uint32(0); i < app.count; i++ {
		child, readErr := m.appArg(app, i)
		if readErr != nil {
			err = readErr
			arguments = nil
			return
		}
		arguments[i] = m.external(child)
	}
	return
}
