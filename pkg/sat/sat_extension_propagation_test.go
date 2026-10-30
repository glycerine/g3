package sat

import "testing"

func TestSATExtensionPropagatesExternalWatch(t *testing.T) {
	state, variables, _ := satPropagationFixture(t, 1)
	extension := &satExtensionFixture{propagateKeep: true}
	state.extension = extension
	state.extensionPresent = true
	watch, err := newSATExternalWatch(42)
	satOK(t, err)
	satPropagationPush(t, state, 0, watch)
	satOK(t, state.assign(0, satJustification{}))
	ok, err := state.propagateCore(false)
	satOK(t, err)
	if !ok || variables.qhead != 1 || len(extension.events) != 1 || extension.events[0] != "propagate" {
		t.Fatal(ok, variables.qhead, extension.events)
	}
}
