package api

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedFloatingPointReplayConstructors(t *testing.T) {
	var log strings.Builder
	log.WriteString("R\nP 0\nC 6\n= 0x41\n")
	for id := 571; id <= 590; id++ {
		log.WriteString("R\nP 0x41\n")
		if id == 582 {
			log.WriteString("U 8\nU 24\n")
		}
		fmt.Fprintf(&log, "C %d\n= 0x%x\n", id, id)
	}
	log.WriteString("R\nP 0x41\nP 0x246\nC 591\n= 0x301\n") // sort 582
	log.WriteString("R\nP 0x41\nP 0x246\nI 1\nC 592\n= 0x302\n")
	log.WriteString("R\nP 0x41\nP 0x246\nI 1\nC 593\n= 0x303\n")
	log.WriteString("R\nP 0x41\nF 1.5\nP 0x246\nC 595\n= 0x304\n")
	log.WriteString("R\nP 0x41\nD 1.5\nP 0x246\nC 596\n= 0x305\n")
	log.WriteString("R\nP 0x41\nI 1\nP 0x246\nC 597\n= 0x306\n")
	log.WriteString("R\nP 0x41\nI 0\nI 0\nU 0\nP 0x246\nC 598\n= 0x307\n")
	log.WriteString("R\nP 0x41\nI 0\nI 0\nU 0\nP 0x246\nC 599\n= 0x308\n")
	log.WriteString("R\nP 0x41\nU 1\nC 35\n= 0x309\nR\nP 0x41\nU 8\nC 35\n= 0x30a\nR\nP 0x41\nU 23\nC 35\n= 0x30b\n")
	log.WriteString("R\nP 0x41\nU 0\nP 0x309\nC 151\n= 0x30c\nR\nP 0x41\nU 127\nP 0x30a\nC 151\n= 0x30d\nR\nP 0x41\nU 0\nP 0x30b\nC 151\n= 0x30e\n")
	log.WriteString("R\nP 0x41\nP 0x30c\nP 0x30d\nP 0x30e\nC 594\n= 0x30f\n")
	log.WriteString("R\nP 0x41\n")
	for id := 571; id <= 590; id++ {
		fmt.Fprintf(&log, "P 0x%x\n", id)
	}
	for id := 0x301; id <= 0x308; id++ {
		fmt.Fprintf(&log, "P 0x%x\n", id)
	}
	log.WriteString("P 0x30f\n")
	log.WriteString("C 900\n")
	r, err := NewZ3Replayer(strings.NewReader(log.String()), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-floating-point-constructors", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		handles := make([]ast.ASTHandle, 29)
		for i := range handles {
			v, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			handles[i] = v.handle
		}
		for i := 1; i < 11; i += 2 {
			if handles[i] != handles[i+1] {
				t.Fatalf("rounding aliases %d", i)
			}
		}
		for i := 12; i < 20; i += 2 {
			if handles[i] != handles[i+1] {
				t.Fatalf("sort aliases %d", i)
			}
		}
		if handles[11] != handles[14] {
			t.Fatal("explicit and single precision sorts differ")
		}
		for i, kind := range []ast.FPAClassificationKind{ast.FPAIsNaN, ast.FPAIsInfinite, ast.FPAIsZero} {
			found, yes, e := owner.results.manager.ClassifyFPANumeral(handles[20+i], kind)
			if e != nil {
				return e
			}
			if !found || !yes {
				t.Fatal("special value", i)
			}
		}
		if handles[23] != handles[24] {
			t.Fatal("float and double values differ")
		}
		if handles[26] != handles[27] {
			t.Fatal("component constructors differ")
		}
		kind, e := owner.results.manager.GetDeclKind(handles[28])
		if e != nil {
			return e
		}
		if kind != ast.FPAOpFP {
			t.Fatal("floating-point constructor kind", kind)
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("floating-point constructors not checked")
	}
}

func TestGeneratedFloatingPointReplayOperations(t *testing.T) {
	var log strings.Builder
	log.WriteString("R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 585\n= 0x42\nR\nP 0x41\nC 572\n= 0x43\n")
	log.WriteString("R\nP 0x41\nI 1\nP 0x42\nC 597\n= 0x44\nR\nP 0x41\nI 2\nP 0x42\nC 597\n= 0x45\n")
	for id := 600; id <= 623; id++ {
		log.WriteString("R\nP 0x41\n")
		if id >= 602 && id <= 607 || id == 609 {
			log.WriteString("P 0x43\n")
		}
		log.WriteString("P 0x44\n")
		if id >= 602 && id <= 606 || id == 608 || id >= 610 && id <= 616 {
			log.WriteString("P 0x45\n")
		}
		if id == 606 {
			log.WriteString("P 0x44\n")
		}
		fmt.Fprintf(&log, "C %d\n= 0x%x\n", id, id)
	}
	log.WriteString("R\nP 0x41\nP 0x25a\nC 287\n= 0x501\nR\nP 0x41\nI 3\nP 0x42\nC 597\n= 0x502\n")
	log.WriteString("R\nP 0x41\n")
	for id := 600; id <= 623; id++ {
		fmt.Fprintf(&log, "P 0x%x\n", id)
	}
	log.WriteString("P 0x501\nP 0x502\n")
	log.WriteString("C 900\n")
	r, err := NewZ3Replayer(strings.NewReader(log.String()), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-floating-point-operations", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		kinds := []int32{ast.FPAOpAbs, ast.FPAOpNeg, ast.FPAOpAdd, ast.FPAOpSub, ast.FPAOpMul, ast.FPAOpDiv, ast.FPAOpFMA, ast.FPAOpSqrt, ast.FPAOpRem, ast.FPAOpRoundToIntegral, ast.FPAOpMin, ast.FPAOpMax, ast.FPAOpLe, ast.FPAOpLt, ast.FPAOpGe, ast.FPAOpGt, ast.FPAOpEq, int32(ast.FPAIsNormal), int32(ast.FPAIsSubnormal), int32(ast.FPAIsZero), int32(ast.FPAIsInfinite), int32(ast.FPAIsNaN), int32(ast.FPAIsNegative), int32(ast.FPAIsPositive)}
		for i, want := range kinds {
			value, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			got, e := owner.results.manager.GetDeclKind(value.handle)
			if e != nil {
				return e
			}
			if got != want {
				t.Fatalf("operation %d: got %d, want %d", 600+i, got, want)
			}
		}
		sum, e := r.astObject(owner, 25)
		if e != nil {
			return e
		}
		three, e := r.astObject(owner, 26)
		if e != nil {
			return e
		}
		if sum.handle != three.handle {
			t.Fatal("floating-point addition did not simplify to three")
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("floating-point operations not checked")
	}
}

func TestGeneratedFloatingPointReplayConversions(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 585\n= 0x42\nR\nP 0x41\nC 572\n= 0x43\n" +
		"R\nP 0x41\nI 1\nP 0x42\nC 597\n= 0x44\nR\nP 0x41\nU 32\nC 35\n= 0x45\n" +
		"R\nP 0x41\nU 1065353216\nP 0x45\nC 151\n= 0x46\nR\nP 0x41\nU 1\nP 0x45\nC 151\n= 0x47\n" +
		"R\nP 0x41\nC 34\n= 0x48\nR\nP 0x41\nS \"1\"\nP 0x48\nC 148\n= 0x49\n" +
		"R\nP 0x41\nP 0x46\nP 0x42\nC 624\n= 0x50\nR\nP 0x41\nP 0x43\nP 0x44\nP 0x42\nC 625\n= 0x51\n" +
		"R\nP 0x41\nP 0x43\nP 0x49\nP 0x42\nC 626\n= 0x52\nR\nP 0x41\nP 0x43\nP 0x47\nP 0x42\nC 627\n= 0x53\n" +
		"R\nP 0x41\nP 0x43\nP 0x47\nP 0x42\nC 628\n= 0x54\nR\nP 0x41\nP 0x43\nP 0x44\nU 32\nC 629\n= 0x55\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nU 32\nC 630\n= 0x56\nR\nP 0x41\nP 0x44\nC 631\n= 0x57\nR\nP 0x41\nP 0x44\nC 649\n= 0x58\n" +
		"R\nP 0x41\nP 0x44\nP 0x47\nP 0x49\nP 0x46\nP 0x50\nP 0x51\nP 0x52\nP 0x53\nP 0x54\nP 0x55\nP 0x56\nP 0x57\nP 0x58\nC 900\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-floating-point-conversions", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		values := make([]ast.ASTHandle, 13)
		for i := range values {
			v, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			values[i] = v.handle
		}
		for i := 4; i < len(values); i++ {
			ref, e := owner.context.Simplify(values[i])
			if e != nil {
				return e
			}
			got, e := owner.results.manager.RefGet(ref)
			if e != nil {
				_ = owner.results.manager.DelRef(&ref)
				return e
			}
			want := values[0]
			if i == 9 || i == 10 {
				want = values[1]
			}
			if i == 11 {
				want = values[2]
			}
			if i == 12 {
				want = values[3]
			}
			if got != want {
				t.Fatalf("conversion %d produced %v, want %v", i, got, want)
			}
			if e = owner.results.manager.DelRef(&ref); e != nil {
				return e
			}
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("floating-point conversions not checked")
	}
}

func TestGeneratedFloatingPointReplayRejectsWrongConversionSort(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 585\n= 0x42\nR\nP 0x41\nC 572\n= 0x43\n" +
		"R\nP 0x41\nU 32\nC 35\n= 0x44\nR\nP 0x41\nU 1\nP 0x44\nC 151\n= 0x45\n" +
		"R\nP 0x41\nP 0x43\nP 0x45\nP 0x42\nC 625\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err == nil || !strings.Contains(err.Error(), ast.ErrASTDomain.Error()) {
		t.Fatal("wrong conversion source sort", err)
	}
}

func TestGeneratedFloatingPointReplayInspectionOutputs(t *testing.T) {
	var log strings.Builder
	log.WriteString("R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 585\n= 0x42\nR\nP 0x41\nD -1.5\nP 0x42\nC 596\n= 0x43\n")
	for id := 632; id <= 640; id++ {
		arg := "0x43"
		if id <= 633 {
			arg = "0x42"
		}
		fmt.Fprintf(&log, "R\nP 0x41\nP %s\nC %d\n", arg, id)
	}
	log.WriteString("R\nP 0x41\nP 0x43\nC 641\n= 0x44\nR\nP 0x41\nP 0x43\nC 642\n= 0x45\n")
	log.WriteString("R\nP 0x41\nP 0x43\nI 7\nC 643\nC 900\nR\nP 0x41\nP 0x43\nC 644\n")
	log.WriteString("R\nP 0x41\nP 0x43\nU 7\nC 645\nC 901\nR\nP 0x41\nP 0x43\nI 1\nC 646\n")
	log.WriteString("R\nP 0x41\nP 0x43\nI 7\nI 1\nC 647\nC 902\nR\nP 0x41\nP 0x43\nI 1\nC 648\n= 0x46\n")
	log.WriteString("R\nP 0x41\nP 0x44\nP 0x45\nP 0x46\nC 903\n")
	r, err := NewZ3Replayer(strings.NewReader(log.String()), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for i, want := range []int64{1, 4194304, 127} {
		id, expected := uint32(900+i), want
		if err = r.RegisterCommand(id, "check-floating-point-output", func(r *Z3Replayer) error {
			var value int64
			if id == 901 {
				n, e := r.Uint64(2)
				if e != nil {
					return e
				}
				value = int64(n)
			} else {
				n, e := r.Int64(2)
				if e != nil {
					return e
				}
				value = n
			}
			if value != expected {
				t.Fatalf("output %d: %d, want %d", id, value, expected)
			}
			checked++
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.RegisterCommand(903, "check-floating-point-bits", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for i, want := range []string{"1", "4194304", "127"} {
			v, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			got, e := owner.context.NumeralString(v.handle)
			if e != nil {
				return e
			}
			if got != want {
				t.Fatalf("component %d: %s, want %s", i, got, want)
			}
		}
		checked++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checked != 4 {
		t.Fatal("floating-point inspection checks", checked)
	}
}
