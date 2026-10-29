package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"testing"
)

func TestForwardTrigPurifyLifecycleAndTranslation(t *testing.T) {
	r, m, _ := forwardTrigFixture(t, false)
	translated, e := r.Translate(m)
	equationMust(t, e)
	equationMust(t, translated.(*ForwardTrigPurifyTactic).Close())
	equationMust(t, r.IncRef())
	equationMust(t, r.DecRef())
	if e = r.open(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("retired forward trig tactic", e)
	}
}
