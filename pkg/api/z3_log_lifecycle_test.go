package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenZ3LogWritesHeaderAndOwnsFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "interaction.log")
	log, err := OpenZ3LogWithBuildDate(filename, "Oct 29 2026")
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Message("ready"); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	want := "V \"4.7.1.0 Oct 29 2026\"\nM \"ready\"\n"
	if string(content) != want {
		t.Fatalf("got %q, want %q", content, want)
	}
	if err := log.Message("closed"); err != ErrZ3LogClosed {
		t.Fatalf("got %v", err)
	}
}

func TestProcessZ3LogOpenReplacesAndAppendWithoutOpenIsNoop(t *testing.T) {
	_ = Z3CloseLog()
	if err := Z3AppendLog("ignored"); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	first := filepath.Join(directory, "first.log")
	second := filepath.Join(directory, "second.log")
	if ok, err := Z3OpenLogWithBuildDate(first, "first"); !ok || err != nil {
		t.Fatalf("open first: %t %v", ok, err)
	}
	if err := Z3AppendLog("one"); err != nil {
		t.Fatal(err)
	}
	if ok, err := Z3OpenLogWithBuildDate(second, "second"); !ok || err != nil {
		t.Fatalf("open second: %t %v", ok, err)
	}
	if err := Z3AppendLog("two"); err != nil {
		t.Fatal(err)
	}
	if err := Z3CloseLog(); err != nil {
		t.Fatal(err)
	}
	firstContent, _ := os.ReadFile(first)
	secondContent, _ := os.ReadFile(second)
	if !strings.Contains(string(firstContent), "M \"one\"") || strings.Contains(string(firstContent), "two") {
		t.Fatalf("first content %q", firstContent)
	}
	if !strings.Contains(string(secondContent), "M \"two\"") {
		t.Fatalf("second content %q", secondContent)
	}
}
