package model

// ScratchCopy returns a copy of md for a debug-only check (a source SASSERT
// or DEBUG_CODE block) that evaluates with model completion. Completion
// registers interpretations in the model it evaluates; release builds skip
// the check and never see them, so the check evaluates the copy instead.
// A full model is copied with model::copy (constants, functions, universes);
// a bare core gets its constants and functions. The caller closes the copy,
// including a nonnil partial copy returned with an error.
func (md *ModelCore) ScratchCopy() (*ModelCore, error) {
	if e := md.open(); e != nil {
		return nil, e
	}
	if md.Derived != nil {
		copied, e := md.Derived.Copy()
		if copied == nil {
			return nil, e
		}
		return &copied.CoreField, e
	}
	copied, e := NewModelCore(md.manager)
	if e != nil {
		return copied, e
	}
	copied.valueProvider = md.valueProvider
	if e = copied.CopyConstantInterpretations(md); e == nil {
		e = copied.CopyFunctionInterpretations(md)
	}
	return copied, e
}
