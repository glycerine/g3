package rel

// Source: src/muz/rel/dl_instruction.cpp unary-singleton, total-relation,
// and signature-assertion instructions.

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestFiniteRelationConstructionInstructions(t *testing.T) {
	context := NewExecutionContext(nil)
	t.Cleanup(func() { _ = context.Close() })
	empty, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	totalSignature, err := NewSignature([]uint64{2, 3})
	if err != nil {
		t.Fatal(err)
	}
	singleton, err := NewUnarySingletonInstruction(4, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	block := NewInstructionBlock()
	t.Cleanup(func() { _ = block.Close() })
	for _, instruction := range []Instruction{
		singleton,
		NewTotalInstruction(empty, 1),
		NewTotalInstruction(totalSignature, 2),
		NewAssertSignatureInstruction(totalSignature, 2),
	} {
		if err = block.Append(instruction); err != nil {
			t.Fatal(err)
		}
	}
	complete, err := block.Perform(context)
	if err != nil || !complete {
		t.Fatal("construct instructions", complete, err)
	}
	instructionWantFacts(t, instructionFacts(t, context, 0), Fact{2})
	instructionWantFacts(t, instructionFacts(t, context, 1), Fact{})
	instructionWantFacts(t, instructionFacts(t, context, 2),
		// hashtable_table slot order.
		Fact{0, 0}, Fact{0, 1}, Fact{0, 2}, Fact{1, 2}, Fact{1, 0}, Fact{1, 1})
	stats, err := context.Statistics()
	if err != nil || stats.UnarySingleton != 1 || stats.Total != 2 {
		t.Fatal("construct statistics", stats, err)
	}
	wrong, err := NewSignature([]uint64{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	assertion := NewAssertSignatureInstruction(wrong, 2)
	t.Cleanup(func() { _ = assertion.Close() })
	if complete, err = assertion.Perform(context); err == nil || complete {
		t.Fatal("wrong signature assertion accepted", complete, err)
	}
}

func TestUnarySingletonInstructionRejectsOutOfDomainValue(t *testing.T) {
	if instruction, err := NewUnarySingletonInstruction(2, 2, 0); err == nil || instruction != nil {
		t.Fatal("out-of-domain singleton accepted", instruction, err)
	}
}

// muz_rel_05-F3: table_base::complement (reached from mk_full for the
// compiler's mk_total) warns before enumerating a one-column domain larger
// than 1<<18, naming the predicate when the compiler passes one.
func TestTotalInstructionWarnsOnLargeDomain(t *testing.T) {
	var buf bytes.Buffer
	warnings := diagnostic.BorrowProcessWarningService()
	previous := warnings.WarningStream()
	if err := warnings.SetWarningStream(&buf); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = warnings.SetWarningStream(previous) })
	for _, tc := range []struct {
		domain uint64
		named  bool
		want   string
	}{
		{domain: 1<<18 + 1, named: true, want: "WARNING: creating large table of size 262145 for relation z2\n"},
		{domain: 1<<18 + 1, want: "WARNING: creating large table of size 262145\n"},
		{domain: 1 << 18, named: true},
	} {
		buf.Reset()
		signature, err := NewSignature([]uint64{tc.domain})
		if err != nil {
			t.Fatal(err)
		}
		instruction := NewTotalInstruction(signature, 0)
		if tc.named {
			instruction = NewTotalInstructionForPredicate(signature, 0, "z2")
		}
		context := NewExecutionContext(nil)
		complete, err := instruction.Perform(context)
		if err != nil || !complete {
			t.Fatal("mk_total", complete, err)
		}
		if got := buf.String(); got != tc.want {
			t.Fatalf("domain %d: warning %q, want %q", tc.domain, got, tc.want)
		}
		if err = errors.Join(instruction.Close(), context.Close()); err != nil {
			t.Fatal(err)
		}
	}
}
