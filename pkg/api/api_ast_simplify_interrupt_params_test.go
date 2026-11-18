package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestSMTContextParameterizedSimplifyReleasesInterruptScope(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := context.NewParameterSet()
	if err != nil {
		t.Fatal(err)
	}
	defer parameters.Close()
	result, err := context.SimplifyWithParameters(truth, parameters)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.DelRef(&result); err != nil {
		t.Fatal(err)
	}
	scope, err := newSMTContextInterruptScope(context, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("parameterized simplify leaked interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}

// AUDIT.md api_01-F4: Z3_simplify_ex builds a bare th_rewriter and pushes
// no rlimit scope, so an rlimit parameter does not stop the rewrite.
func TestSimplifyIgnoresRLimitParam(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	sum, err := manager.ArithmeticNumeralInt64(int64(0), true)
	if err != nil {
		t.Fatal(err)
	}
	root, err := manager.NewRef(sum)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.DelRef(&root)
	for i := int64(1); i <= 50; i++ {
		n, err := manager.ArithmeticNumeralInt64(int64(i), true)
		if err != nil {
			t.Fatal(err)
		}
		if sum, err = manager.MkArithmeticAdd(n, sum); err != nil {
			t.Fatal(err)
		}
		if err = manager.SetRef(root, sum); err != nil {
			t.Fatal(err)
		}
	}
	parameters, err := context.NewParameterSet()
	if err != nil {
		t.Fatal(err)
	}
	defer parameters.Close()
	if err = parameters.SetUInt("rlimit", 1); err != nil {
		t.Fatal(err)
	}
	result, err := context.SimplifyWithParameters(sum, parameters)
	if err != nil {
		t.Fatal("rlimit stopped simplify", err)
	}
	defer manager.DelRef(&result)
	got, err := manager.RefGet(result)
	if err != nil {
		t.Fatal(err)
	}
	want, err := manager.ArithmeticNumeralInt64(int64(1275), true)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("simplify did not fold the sum")
	}
}

// AUDIT.md api_01-F4: without a timeout parameter simplify uses the
// context timeout (mk_c(c)->get_timeout()).
func TestSimplifyTimeoutDefaultsToContextTimeout(t *testing.T) {
	context := newAPIInterruptFixture(t)
	if err := context.SetUIntOption("timeout", 37); err != nil {
		t.Fatal(err)
	}
	var seen []uint32
	saved := simplifyTimer
	simplifyTimer = func(ms uint32, h control.EventHandler) (*control.ScopedTimer, error) {
		seen = append(seen, ms)
		return saved(ms, h)
	}
	defer func() { simplifyTimer = saved }()
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	result, err := context.Simplify(truth)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.DelRef(&result); err != nil {
		t.Fatal(err)
	}
	parameters, err := context.NewParameterSet()
	if err != nil {
		t.Fatal(err)
	}
	defer parameters.Close()
	if err = parameters.SetUInt("timeout", 5000); err != nil {
		t.Fatal(err)
	}
	if result, err = context.SimplifyWithParameters(truth, parameters); err != nil {
		t.Fatal(err)
	}
	if err = manager.DelRef(&result); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != 37 || seen[1] != 5000 {
		t.Fatal("simplify timeouts", seen)
	}
}
