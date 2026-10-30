package api

import "testing"

func TestAPIOptimizeInterruptScopeCloseIsIdempotent(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	scope, err := newAPIOptimizeInterruptScope(f.opt)
	if err != nil {
		t.Fatal(err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
	if scope.handler != nil || scope.scope != nil || !scope.closed {
		t.Fatal("scope retained closed owners")
	}
}
