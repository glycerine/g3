package sat

import "github.com/glycerine/g3/pkg/util/lbool"

type satExtensionFixture struct {
	events        []string
	phase         lbool.LBool
	checkResult   SATExtensionCheckResult
	antecedents   []Literal
	propagateKeep bool
	err           error
}

func (f *satExtensionFixture) record(name string) error {
	f.events = append(f.events, name)
	return f.err
}
func (f *satExtensionFixture) Propagate(_ Literal, _ SATExternalConstraintIndex, _ bool) (bool, error) {
	return f.propagateKeep, f.record("propagate")
}
func (f *satExtensionFixture) GetAntecedents(_ Literal, _ SATExternalJustificationIndex, out *[]Literal) error {
	*out = append(*out, f.antecedents...)
	return f.record("antecedents")
}
func (f *satExtensionFixture) Asserted(Literal) error { return f.record("asserted") }
func (f *satExtensionFixture) Check() (SATExtensionCheckResult, error) {
	f.events = append(f.events, "check")
	return f.checkResult, f.err
}
func (f *satExtensionFixture) Push() error            { return f.record("push") }
func (f *satExtensionFixture) Pop(uint32) error       { return f.record("pop") }
func (f *satExtensionFixture) Simplify() error        { return f.record("simplify") }
func (f *satExtensionFixture) ClausesModified() error { return f.record("clauses") }
func (f *satExtensionFixture) GetPhase(BoolVar) (lbool.LBool, error) {
	f.events = append(f.events, "phase")
	return f.phase, f.err
}
