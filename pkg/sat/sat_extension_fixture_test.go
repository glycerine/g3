package sat

import "github.com/glycerine/g3/pkg/util/lbool"

type satExtensionBindingFixture struct {
	events        []string
	phase         lbool.LBool
	checkResult   SATExtensionCheckResult
	antecedents   []Literal
	propagateKeep bool
	err           error
}

func (f *satExtensionBindingFixture) record(name string) error {
	f.events = append(f.events, name)
	return f.err
}
func (f *satExtensionBindingFixture) Propagate(_ Literal, _ SATExternalConstraintIndex, keep *bool) error {
	*keep = f.propagateKeep
	return f.record("propagate")
}
func (f *satExtensionBindingFixture) GetAntecedents(_ Literal, _ SATExternalJustificationIndex, out *[]Literal) error {
	*out = append(*out, f.antecedents...)
	return f.record("antecedents")
}
func (f *satExtensionBindingFixture) Asserted(Literal) error { return f.record("asserted") }
func (f *satExtensionBindingFixture) Check() (SATExtensionCheckResult, error) {
	f.events = append(f.events, "check")
	return f.checkResult, f.err
}
func (f *satExtensionBindingFixture) Push() error            { return f.record("push") }
func (f *satExtensionBindingFixture) Pop(uint32) error       { return f.record("pop") }
func (f *satExtensionBindingFixture) Simplify() error        { return f.record("simplify") }
func (f *satExtensionBindingFixture) ClausesModified() error { return f.record("clauses") }
func (f *satExtensionBindingFixture) GetPhase(BoolVar) (lbool.LBool, error) {
	f.events = append(f.events, "phase")
	return f.phase, f.err
}

var _ SATExtension = (*satExtensionBindingFixture)(nil)
