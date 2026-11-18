package ast

// Source: src/ast/format.cpp:35-117,147-212; src/ast/format.h:29-66.

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestASTFormatSourcePluginAndLiteralPrimitives(t *testing.T) {
	m := astTestManager(t)
	f, e := m.GetFormatManager()
	astTestMust(t, e)
	id, e := m.GetFormatFamilyID()
	astTestMust(t, e)
	again, e := m.GetFormatFamilyID()
	astTestMust(t, e)
	if id != again || id == NullFamilyID {
		t.Fatal("source lazy plugin identity", id, again)
	}
	sort, e := m.FormatSort()
	astTestMust(t, e)
	fid, e := f.GetFamilyID(sort)
	astTestMust(t, e)
	if fid != id {
		t.Fatal(fid, id)
	}
	interpreted, e := f.IsFullyInterpreted(sort)
	astTestMust(t, e)
	if !interpreted {
		t.Fatal("actual newly registered format plugin inherits is_fully_interp true")
	}
	text, e := m.FormatString("hello")
	astTestMust(t, e)
	ind, e := m.FormatIndent(7, text)
	astTestMust(t, e)
	line, e := m.FormatLineBreak()
	astTestMust(t, e)
	choice, e := m.FormatChoice(text, line)
	astTestMust(t, e)
	compose, e := m.FormatCompose([]ASTHandle{text, ind, line, choice})
	astTestMust(t, e)
	nilfmt, e := m.FormatNil()
	astTestMust(t, e)
	for _, h := range []ASTHandle{text, ind, line, choice, compose, nilfmt} {
		kind, e := f.GetKind(h)
		astTestMust(t, e)
		if kind != ASTApp {
			t.Fatal("actual app payload", kind)
		}
		got, e := f.GetSort(h)
		astTestMust(t, e)
		if got != sort {
			t.Fatal(got, sort)
		}
	}
	text2, e := m.FormatString("hello")
	astTestMust(t, e)
	if text2 != text {
		t.Fatal("source hash-consed string", text, text2)
	}
	signed, e := m.FormatInt(-1 << 31)
	astTestMust(t, e)
	signedText, e := m.FormatString("-2147483648")
	astTestMust(t, e)
	if signed != signedText {
		t.Fatal("source signed decimal formatting", signed, signedText)
	}
	unsigned, e := m.FormatUnsigned(^uint32(0))
	astTestMust(t, e)
	unsignedText, e := m.FormatString("4294967295")
	astTestMust(t, e)
	if unsigned != unsignedText {
		t.Fatal("source unsigned decimal formatting", unsigned, unsignedText)
	}
}

func TestASTFormatSourceEveryLazyPluginAndStringReservation(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	success := false
	sawPublished := false
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		b := arena.NewBudget(1 << 30)
		m, e := NewASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
		astTestMust(t, e)
		f, e := m.GetFormatManager()
		astTestMust(t, e)
		b.FailAfter(ordinal)
		h, e := m.FormatString("source prefix text")
		if b.Used() != m.Stats().AllocatedBytes {
			t.Fatal("exact acquired graph before fault lift", ordinal, b.Used(), m.Stats())
		}
		if e != nil {
			if !errors.Is(e, arena.ErrInjectedFailure) {
				t.Fatal(ordinal, e)
			}
			if p := f.formatPluginFailed; p != nil {
				if p.header == 0 || p.self != p || p.owner != f {
					t.Fatal("actual failed wrapper", ordinal)
				}
			}
			if p := f.formatPlugin; p != nil {
				sawPublished = true
				if !p.published || !f.pluginAt(p.family).present {
					t.Fatal("source registry publication before set_manager", ordinal)
				}
			}
		} else {
			success = true
			if h.IsNull() || f.formatPlugin == nil || f.formatPlugin.sort.slot == 0 {
				t.Fatal("completed factory", ordinal)
			}
		}
		b.FailAfter(-1)
		astTestMust(t, m.Close())
		if b.Used() != 0 {
			t.Fatal("exact owning close", ordinal, b.Used())
		}
		if success {
			break
		}
	}
	if !success || !sawPublished {
		t.Fatal(success, sawPublished)
	}
}

func TestASTFormatSourceLexicalParameterAndCopiedInfo(t *testing.T) {
	m := astTestManager(t)
	f, e := m.GetFormatManager()
	astTestMust(t, e)
	h, e := m.FormatString("old text")
	astTestMust(t, e)
	ind, e := m.FormatIndent(^uint32(0), h)
	astTestMust(t, e)
	r, e := f.internal(ind)
	astTestMust(t, e)
	d := f.appAt(r.slot).decl
	info, e := f.funcInfo(f.declAt(d.slot).info)
	astTestMust(t, e)
	p, e := f.infoParameter(info, 0)
	astTestMust(t, e)
	v, e := f.parameter(p)
	astTestMust(t, e)
	if v.Kind != ASTParameterInt || v.integer != -1 {
		t.Fatal("unsigned source parameter(int) conversion", v)
	}
	before := f.liveParameters
	_, e = m.FormatString("next text")
	astTestMust(t, e)
	if f.formatLexical.active || f.formatLexical.info.live || before >= f.liveParameters {
		t.Fatal("lexical source record leaked or no owned copy", before, f.liveParameters)
	}
	if _, e = f.parameter(ASTParameter{f.origin, math.MaxUint32 - 1, f.formatLexical.epoch}); !errors.Is(e, ErrASTHandle) {
		t.Fatal("ended lexical parameter read", e)
	}
	again, e := f.parameter(p)
	astTestMust(t, e)
	if again.integer != -1 {
		t.Fatal("copied info reused lexical parameter", again)
	}
}

func TestASTFormatSourceFlatLiteralChoiceAndSharedChildren(t *testing.T) {
	m := astTestManager(t)
	f, e := m.GetFormatManager()
	astTestMust(t, e)
	text, e := m.FormatString("left")
	astTestMust(t, e)
	line, e := m.FormatLineBreak()
	astTestMust(t, e)
	ind, e := m.FormatIndent(2, line)
	astTestMust(t, e)
	choice, e := m.FormatChoice(line, text)
	astTestMust(t, e)
	root, e := m.FormatCompose([]ASTHandle{ind, choice, ind})
	astTestMust(t, e)
	flat, e := m.FormatFlat(root)
	astTestMust(t, e)
	r, e := f.internal(flat)
	astTestMust(t, e)
	a := f.appAt(r.slot)
	first, e := f.appArg(a, 0)
	astTestMust(t, e)
	again, e := f.appArg(a, 2)
	astTestMust(t, e)
	if first != again {
		t.Fatal("recurse_expr cache shared result", first, again)
	}
	arg, e := f.appArg(f.appAt(first.slot), 0)
	astTestMust(t, e)
	_, kind, e := f.declIdentity(f.appAt(arg.slot).decl)
	astTestMust(t, e)
	if kind != ASTFormatString {
		t.Fatal("line break becomes space", kind)
	}
	middle, e := f.appArg(a, 1)
	astTestMust(t, e)
	if f.external(middle) != line {
		t.Fatal("source CHOICE returns original arg0, not transformed child", f.external(middle), line)
	}
	group, e := m.FormatGroup(root)
	astTestMust(t, e)
	gr, e := f.internal(group)
	astTestMust(t, e)
	original, e := f.appArg(f.appAt(gr.slot), 1)
	astTestMust(t, e)
	if f.external(original) != root {
		t.Fatal("source group preserves original second alternative")
	}
}

func TestASTFormatSourceEveryFlatAllocationAndImmediateGraph(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	first := -1
	for ordinal := int64(0); ordinal < 160; ordinal++ {
		b := arena.NewBudget(1 << 30)
		m, e := NewASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
		astTestMust(t, e)
		args := make([]ASTHandle, 54)
		for i := range args {
			args[i], e = m.FormatUnsigned(uint32(i))
			astTestMust(t, e)
		}
		line, e := m.FormatLineBreak()
		astTestMust(t, e)
		args[1] = line
		root, e := m.FormatCompose(args)
		astTestMust(t, e)
		b.FailAfter(ordinal)
		flat, e := m.FormatFlat(root)
		if b.Used() != m.Stats().AllocatedBytes {
			t.Fatal("actual AST and failed-frame graph before lift", ordinal, b.Used(), m.Stats())
		}
		if e != nil && !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal(ordinal, e)
		}
		if e == nil && flat.IsNull() {
			t.Fatal("successful flat result", ordinal)
		}
		f, _ := m.GetFormatManager()
		if e != nil {
			if ordinal == 0 {
				if f.formatFrames != nil {
					t.Fatal("cold frame allocation refused before construction", ordinal)
				}
			} else if f.formatFrames == nil || f.formatFrames.bodyCause == nil || !f.formatFrames.sourceDone {
				t.Fatal("actual lexical member completion", ordinal)
			}
		}
		b.FailAfter(-1)
		astTestMust(t, m.Close())
		if b.Used() != 0 {
			t.Fatal("exact owning frame/storage release", ordinal, b.Used())
		}
		if e == nil {
			first = int(ordinal)
			break
		}
	}
	if first < 1 {
		t.Fatal("actual frame/factory reservations were not exercised", first)
	}
	t.Log("first success", first)
}

func TestASTFormatFlatHistorySurvivesNativeSourceFinalize(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	b := arena.NewBudget(1 << 30)
	m, e := NewASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
	astTestMust(t, e)
	root, e := m.FormatLineBreak()
	astTestMust(t, e)
	f, e := m.GetFormatManager()
	astTestMust(t, e)
	b.FailAfter(1)
	_, body := m.FormatFlat(root)
	if !errors.Is(body, arena.ErrInjectedFailure) {
		t.Fatal("actual AST factory allocation refusal", body)
	}
	frame := f.formatFrames
	if frame == nil || frame.bodyCause != body || frame.header == 0 {
		t.Fatal("actual acquired frame")
	}
	originalHeader, originalInvocation := frame.header, frame.invocation
	// Native canonical iteration and root collection need no arena reservation.
	b.FailAfter(0)
	astTestMust(t, f.SourceFinalize())
	if f.Closed || f.table.size != 0 {
		t.Fatal("source finalization must drain nodes and retain the enclosing owner")
	}
	if f.formatFrames != frame || frame.header != originalHeader || frame.invocation != originalInvocation || frame.bodyCause != body {
		t.Fatal("history cannot retire before containing owner ends")
	}
	if b.Used() != m.Stats().AllocatedBytes {
		t.Fatal("immediate finalized context graph", b.Used(), m.Stats())
	}
	b.FailAfter(-1)
	astTestMust(t, m.Close())
	if frame.header != 0 || b.Used() != 0 {
		t.Fatal("history retires only at owner end", frame.header, b.Used())
	}
}

func TestASTFormatFlatNativeDeepSharedCacheWithOnlyFrameReservation(t *testing.T) {
	m := astTestManager(t)
	f, e := m.GetFormatManager()
	astTestMust(t, e)
	var leaves []ASTHandle
	for i := uint32(0); i < 129; i++ {
		h, e := m.FormatUnsigned(i)
		astTestMust(t, e)
		leaves = append(leaves, h)
	}
	root := leaves[0]
	var expected ASTHandle
	branches := append(append([]ASTHandle(nil), leaves[1:]...), leaves...)
	for _, leaf := range branches {
		expected = root
		root, e = m.FormatChoice(root, leaf)
		astTestMust(t, e)
	}
	refs := make([]uint32, len(leaves))
	for i, h := range leaves {
		refs[i], e = f.GetRefCount(h)
		astTestMust(t, e)
	}
	before := m.Stats()
	used := m.BudgetField.Used()
	for attempt := 0; attempt < 2; attempt++ {
		// The actual recursion frame remains fallible. Native map/slice growth and
		// cached native-profile factories need no subsequent arena reservation.
		if !ASTSourceDebug {
			m.BudgetField.FailAfter(1)
		}
		flat, e := m.FormatFlat(root)
		m.BudgetField.FailAfter(-1)
		astTestMust(t, e)
		if flat != expected {
			t.Fatal("source CHOICE returns original first alternative", flat, expected)
		}
		if f.formatFrames != nil {
			t.Fatal("successful traversal retained frame history")
		}
		for i, h := range leaves {
			got, e := f.GetRefCount(h)
			astTestMust(t, e)
			if got != refs[i] {
				t.Fatal("borrowed traversal changed reference counts", i, got, refs[i])
			}
		}
		if !ASTSourceDebug && (m.BudgetField.Used() != used || m.Stats() != before) {
			t.Fatal("native traversal charged container backing")
		}
	}
}
