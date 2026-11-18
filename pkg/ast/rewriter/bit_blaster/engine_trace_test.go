package bit_blaster

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// bit_blaster_cfg::mk_xor3 / mk_carry trace the operand ids before and after
// sort_args. Matched by tag and function, not by line.
func TestBitBlasterGateTraceTags(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	for _, tag := range []string{"xor3", "xor3_sorted", "carry", "carry_sorted"} {
		tags.Enable(tag)
	}
	if !tags.ShouldTrace("xor3") {
		t.Skip("source trace macro erased")
	}
	b := arena.NewBudget(1 << 30)
	s, err := usymbol.NewManager(usymbol.Config{Budget: b})
	paramsMust(t, err)
	q, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	paramsMust(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: s, Numerals: q, Budget: b, TraceTags: tags})
	paramsMust(t, err)
	t.Cleanup(func() {
		paramsMust(t, m.Close())
		paramsMust(t, q.Close())
		paramsMust(t, s.Close())
	})
	_, err = m.RegisterArithmeticPlugin()
	paramsMust(t, err)
	family, err := m.RegisterBitVectorPlugin()
	paramsMust(t, err)
	config, err := NewConfig(m, family, SMTBitBlasterParams{ExtGates: true})
	paramsMust(t, err)
	boolean, err := m.MkBoolSort()
	paramsMust(t, err)
	var atoms [3]ast.ASTHandle
	for i, name := range []string{"trace-c", "trace-a", "trace-b"} {
		sym, e := m.Symbols.Intern(name)
		paramsMust(t, e)
		atoms[i], e = m.MkConst(sym, boolean)
		paramsMust(t, e)
	}
	out, err := m.NewRef(ast.ASTHandle{})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.DelRef(&out)) }()
	// Pass the operands out of id order so the sorted record differs.
	paramsMust(t, config.MkXor3(atoms[2], atoms[0], atoms[1], out))
	paramsMust(t, config.MkCarry(atoms[2], atoms[0], atoms[1], out))
	text := output.String()
	for _, want := range []string{"[xor3] mk_xor3 ", "[xor3_sorted] mk_xor3 ", "[carry] mk_carry ", "[carry_sorted] mk_carry "} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	id := func(h ast.ASTHandle) string {
		v, e := m.GetASTID(h)
		paramsMust(t, e)
		return "#" + strconv.FormatUint(uint64(v), 10)
	}
	unsorted := id(atoms[2]) + " " + id(atoms[0]) + " " + id(atoms[1])
	sorted := id(atoms[0]) + " " + id(atoms[1]) + " " + id(atoms[2])
	if !strings.Contains(text, unsorted) || !strings.Contains(text, sorted) {
		t.Fatalf("operand id records missing %q / %q in %q", unsorted, sorted, text)
	}
}
