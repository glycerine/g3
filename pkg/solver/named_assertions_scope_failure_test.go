package solver

import (
	"errors"
	"testing"
)

type namedPublishedScopeCore struct {
	namedCoreFixture
	level   uint32
	publish bool
	failure error
}

func (c *namedPublishedScopeCore) ScopeLevelCore() uint32 { return c.level }
func (c *namedPublishedScopeCore) PushCore() error {
	if c.publish {
		c.level++
	}
	return c.failure
}
func (c *namedPublishedScopeCore) PopCore(count uint32) error {
	c.level -= count
	return nil
}

func TestNamedAssertionsFailedPushTracksCorePublication(t *testing.T) {
	for _, publish := range []bool{false, true} {
		m := solverTestManager(t)
		failure := errors.New("native push refusal")
		core := &namedPublishedScopeCore{level: 3, publish: publish, failure: failure}
		n, err := NewNamedAssertions(m, core)
		solverMust(t, err)
		t.Cleanup(func() { solverMust(t, n.Close()) })
		if err = n.Push(); !errors.Is(err, failure) {
			t.Fatal("push refusal changed", err)
		}
		level, err := n.ScopeLevel()
		solverMust(t, err)
		want := uint32(0)
		if publish {
			want = 1
		}
		if level != want || core.level != 3+want {
			t.Fatal("failed push scope publication", publish, level, core.level)
		}
		if publish {
			solverMust(t, n.Pop(1))
		}
	}
}
