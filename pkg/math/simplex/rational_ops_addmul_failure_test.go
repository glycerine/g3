package simplex

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// TestRatCalcAddMulFailureReleasesTemporary injects an allocation failure at
// every point of a synchronized manager's addmul. The source's temporary is
// a scoped local destroyed by the exception unwind, so after any failure the
// manager must still compare values instead of reporting a pending
// temporary (which made the qe allocation-frontier test's AST close spin).
func TestRatCalcAddMulFailureReleasesTemporary(t *testing.T) {
	budget := arena.NewBudget(1 << 30)
	q, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{PageSize: 2, Budget: budget}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}}}, Synchronized: true})
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	value := func(text string) *numeral.Rational {
		r := numeral.NewRational()
		if err := q.SetString(&r, text); err != nil {
			t.Fatal(err)
		}
		return &r
	}
	a := value("123456789012345678901234567890/7")
	b := value("98765432109876543210987654321/11")
	c := value("-55555555555555555555555555555/13")
	pre := numeral.NewRational()
	if err := q.Add(a, b, &pre); err != nil {
		t.Fatalf("add before any failure: %v", err)
	}
	_ = q.Del(&pre)
	failed := 0
	for stop := int64(0); stop < 64; stop++ {
		out := numeral.NewRational()
		calc := ratCalc{q: q}
		budget.FailAfter(stop)
		calc.addMul(&out, a, b, c)
		budget.FailAfter(-1)
		if calc.err == nil {
			_ = q.Del(&out)
			break
		}
		failed++
		// Equal reads values in place, as the AST table's parameter
		// comparison does; a pending temporary refuses it.
		if _, err := q.Equal(a, a); err != nil {
			t.Fatalf("stop %d: manager refuses comparison after addmul failure: %v", stop, err)
		}
		_ = q.Del(&out)
	}
	if failed == 0 {
		t.Fatal("no injected addmul failure")
	}
	for _, r := range []*numeral.Rational{a, b, c} {
		_ = q.Del(r)
	}
}
