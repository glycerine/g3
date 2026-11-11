package ast

// Source: src/ast/ast.cpp:1297-1334,1474-1479.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func astTracePath(t *testing.T) string {
	t.Helper()
	f, e := os.CreateTemp(".", "ast-trace-*")
	if e != nil {
		t.Fatal(e)
	}
	name, e := filepath.Abs(f.Name())
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Remove(name) })
	return name
}
func TestASTTraceFileActualOpenTruncateFormattingAndOwningClose(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	name := astTracePath(t)
	if e := os.WriteFile(name, []byte("old contents"), 0600); e != nil {
		t.Fatal(e)
	}
	f, e := newASTTraceFile(b, uparams.LiteralParamsStringView(name))
	paramsMust(t, e)
	if f == nil || f.stream.Buffer != nil || f.stream.File == nil || f.stream.OwnerBytes != 0 || b.Used() != uint64(reflect.TypeOf(f).Elem().Size()) {
		t.Fatal("one real header, embedded borrowed formatter, actual owned fd")
	}
	paramsMust(t, f.stream.SetShowPos(true))
	paramsMust(t, f.stream.AppendInt64(17))
	paramsMust(t, f.stream.AppendCString("\n"))
	paramsMust(t, f.stream.Flush())
	out, e := os.ReadFile(name)
	paramsMust(t, e)
	if string(out) != "+17\n" {
		t.Fatal("real truncate and formatter output", string(out))
	}
	borrowed := f.stream.File
	paramsMust(t, f.Close())
	if _, e = borrowed.Write([]byte("x")); e == nil {
		t.Fatal("actual owned file must close")
	}
	paramsMust(t, f.Close())
	if b.Used() != 0 {
		t.Fatal("actual header refund", b.Used())
	}
}
func TestASTTraceFileFailedOpenIsNonNullBadStreamWithoutHiddenMemoryOutput(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	name := astTracePath(t) + "/missing"
	f, e := newASTTraceFile(b, uparams.LiteralParamsStringView(name))
	paramsMust(t, e)
	if f == nil || f.stream.File != nil || f.stream.Buffer != nil {
		t.Fatal("source fstream remains nonnull after actual failed open")
	}
	bad, e := f.stream.Bad()
	paramsMust(t, e)
	if bad {
		t.Fatal("fstream open failure sets failbit, not badbit")
	}
	failed, e := f.stream.Fail()
	paramsMust(t, e)
	if !failed {
		t.Fatal("actual failbit from filebuf open")
	}
	b.FailAfter(0)
	paramsMust(t, f.stream.AppendCString("[mk-app] #"))
	paramsMust(t, f.stream.AppendUint32(17))
	paramsMust(t, f.stream.Flush())
	if b.Used() != f.header || f.stream.Stats().Position != 0 {
		t.Fatal("badbit suppresses output without changing owner or reserving unused buffer")
	}
	paramsMust(t, f.Close())
	if b.Used() != 0 {
		t.Fatal("failed-open owner refund")
	}
}
func TestASTTraceFileEveryConstructorReservationAndSourcePointerGuard(t *testing.T) {
	for ordinal := int64(0); ordinal < 2; ordinal++ {
		b := arena.NewBudget(1 << 20)
		name := astTracePath(t)
		b.FailAfter(ordinal)
		f, e := newASTTraceFile(b, uparams.LiteralParamsStringView(name))
		if ordinal == 0 {
			if f != nil || !errors.Is(e, arena.ErrInjectedFailure) || b.Used() != 0 {
				t.Fatal("actual wrapper new-expression first refusal", f, e)
			}
		} else {
			paramsMust(t, e)
			if b.Used() != f.header || f.stream.Buffer != nil {
				t.Fatal("exact current file owner before fault lift")
			}
			paramsMust(t, f.Close())
		}
		b.FailAfter(-1)
		if b.Used() != 0 {
			t.Fatal("exact once-only constructor/header release")
		}
	}
	b := arena.NewBudget(1 << 20)
	f, e := newASTTraceFile(b, uparams.ParamsStringView{})
	if !errors.Is(e, utext.ErrNullCString) || f == nil || b.Used() != f.header {
		t.Fatal("reached null-pointer Go guard retains actual acquired wrapper", e)
	}
	paramsMust(t, f.Close())
	if b.Used() != 0 {
		t.Fatal("explicit failed constructor abort")
	}
}

func TestASTTraceFileFailbitSentryPreservesWidthButCStringStillReadsLength(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	f, e := newASTTraceFile(b, uparams.LiteralParamsStringView(astTracePath(t)+"/missing"))
	paramsMust(t, e)
	defer func() { paramsMust(t, f.Close()) }()
	paramsMust(t, f.stream.SetWidth(17))
	before := f.stream.State()
	paramsMust(t, (uparams.LiteralParamsStringView("payload")).AppendTo(&f.stream))
	if f.stream.State() != before {
		t.Fatal("failed-open sentry must erase padding/output and preserve width")
	}
	if e = (uparams.ParamsStringView{Mode: 2, PresentField: true}).AppendTo(&f.stream); !errors.Is(e, uparams.ErrParamsOwner) {
		t.Fatal("actual CString length read precedes failed sentry", e)
	}
	if b.Used() != f.header || f.stream.Stats().Position != 0 {
		t.Fatal("unchanged exact failed-open cold graph")
	}
}

func TestASTTraceFileFailedSentryOwnedStringKeepsWidthAndReachedLifetime(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	f, e := newASTTraceFile(b, uparams.LiteralParamsStringView(astTracePath(t)+"/missing"))
	paramsMust(t, e)
	var value utext.OwnedString
	paramsMust(t, value.Construct(b, "trace"))
	paramsMust(t, f.stream.SetWidth(17))
	paramsMust(t, value.AppendTo(&f.stream))
	if f.stream.State().Width != 17 {
		t.Fatal("failed ostream sentry consumed std::string width")
	}
	value.SourceDestroy() // actual owned string DT ends the borrow domain.
	if e = value.AppendTo(&f.stream); !errors.Is(e, utext.ErrOwnedStringOwner) {
		t.Fatal("failed sentry erased reached string data/length guard", e)
	}
	paramsMust(t, f.Close())
	if b.Used() != 0 {
		t.Fatal("inline string/file real cold graph", b.Used())
	}
}

func TestASTSourceTraceConstructorNestedFormatWritesSameActualFileBeforeParent(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	b := arena.NewBudget(1 << 30)
	name := astTracePath(t)
	filename := uparams.LiteralParamsStringView(name)
	m, e := NewSourceASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b}, &filename)
	paramsMust(t, e)
	defer m.Close()
	if m.Format == nil || m.Format.TraceStream != m.TraceStream || m.Format.traceOwner || !m.traceOwner {
		t.Fatal("actual borrowed nested format stream identity")
	}
	paramsMust(t, m.TraceStream.Flush())
	got, e := os.ReadFile(name)
	paramsMust(t, e)
	one := "[mk-app] #0 undef\n[mk-app] #1 true\n[mk-app] #2 false\n"
	if string(got) != one+one {
		t.Fatal("nested format initialization must precede parent on same stream", string(got))
	}
	paramsMust(t, m.SourceFinalize())
	got, e = os.ReadFile(name)
	paramsMust(t, e)
	if string(got) != one+one+"[eof]\n" {
		t.Fatal("only owning parent emits source EOF", string(got))
	}
	if b.Used() != m.Stats().AllocatedBytes {
		t.Fatal("immediate source completed owning graph", b.Used(), m.Stats())
	}
}
