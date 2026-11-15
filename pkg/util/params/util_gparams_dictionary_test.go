package params

import (
	"strings"

	uhash "github.com/glycerine/g3/pkg/util/hash"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

import "testing"

func TestGparamsDictionariesUseRequiredGoOrdering(t *testing.T) {
	var descriptors gparamsDescriptorDictionary
	descriptors.init()
	for i := uint32(0); i < 8; i++ {
		descriptors.set(usymbol.Numerical(i), i, i+1)
	}
	descriptors.set(usymbol.Numerical(2), 2, 99)
	if got, ok := descriptors.get(usymbol.Numerical(2), 2); !ok || got != 99 || descriptors.size() != 8 {
		t.Fatal(got, ok, descriptors.size())
	}
	// Numerical symbols hash to their value: slot order is ascending here.
	want := uint32(0)
	for key := range descriptors.all() {
		if key != usymbol.Numerical(want) {
			t.Fatal("descriptor table order", key, want)
		}
		want++
	}

	var refs gparamsRefDictionary
	refs.init()
	refs.set(usymbol.Numerical(1), 4)
	refs.set(usymbol.Numerical(1), 9)
	if got, ok := refs.get(usymbol.Numerical(1)); !ok || got != 9 || refs.size() != 1 {
		t.Fatal(got, ok, refs.size())
	}
	refs.reset()
	if refs.size() != 0 {
		t.Fatal(refs.size())
	}

	var descriptions gparamsDescriptionDictionary
	descriptions.init()
	text := utext.RegionCString{Offset: 7, Present: true}
	descriptions.set(usymbol.Numerical(3), text)
	if got, ok := descriptions.get(usymbol.Numerical(3)); !ok || got != text {
		t.Fatal(got, ok)
	}
}

// AUDIT.md util_params_01-F1: m_module_param_descrs is a dictionary over
// core_hashtable keyed by symbol::hash() (string_hash(name, 17)); display
// walks its slots. Registering z3's modules in gparams_register_modules
// order (repeats skipped, as register_module finds them first) yields z3's
// [module] order.
func TestGparamsDescriptorDictionaryTableOrder(t *testing.T) {
	registered := []string{"ackermannization", "rewriter", "nnf", "pi", "pp", "interp", "algebraic", "rcf", "model_evaluator", "model", "fixedpoint", "nlsat", "opt", "parser", "sat", "smt", "combined_solver", "sls", "lp"}
	var d gparamsDescriptorDictionary
	d.init()
	names := map[usymbol.Symbol]string{}
	for i, name := range registered {
		k := usymbol.Numerical(uint32(1000 + i))
		names[k] = name
		d.set(k, uhash.String(name, 17), uint32(i+1))
	}
	var order []string
	for k, v := range d.all() {
		if registered[v-1] != names[k] {
			t.Fatal("value", k, v)
		}
		order = append(order, names[k])
	}
	if got := strings.Join(order, " "); got != "pi pp fixedpoint model sat opt lp nnf algebraic combined_solver rcf model_evaluator ackermannization nlsat parser rewriter interp smt sls" {
		t.Fatal("module table order", got)
	}
	for i, name := range registered {
		if v, ok := d.get(usymbol.Numerical(uint32(1000+i)), uhash.String(name, 17)); !ok || v != uint32(i+1) {
			t.Fatal("lookup", name, v, ok)
		}
	}
	if len(d.table) != 32 || d.size() != 19 {
		t.Fatal("capacity", len(d.table), d.size())
	}
	d.reset()
	if d.size() != 0 || len(d.table) != 32 {
		t.Fatal("reset", d.size(), len(d.table))
	}
}
