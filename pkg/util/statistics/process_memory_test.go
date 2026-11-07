package statistics

import (
	"bytes"
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

func processMemoryField(t *testing.T, output, name string) float64 {
	t.Helper()
	match := regexp.MustCompile(regexp.QuoteMeta(name) + ` +([0-9.]+)`).FindStringSubmatch(output)
	if match == nil {
		t.Fatalf("no %s in %q", name, output)
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestAddProcessMemoryReflectsLiveHeap(t *testing.T) {
	read := func() string {
		s := New()
		AddProcessMemory(s)
		var out bytes.Buffer
		if err := s.DisplaySMT2(&out); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	runtime.GC()
	before := processMemoryField(t, read(), ":memory")
	const mega = 64
	held := make([]byte, mega<<20)
	for i := range held {
		held[i] = byte(i)
	}
	output := read()
	runtime.KeepAlive(held)
	if after := processMemoryField(t, output, ":memory"); after < before+mega*0.9 {
		t.Fatalf(":memory went from %v to %v MB holding %d MB", before, after, mega)
	}
	if peak := processMemoryField(t, output, ":max-memory"); peak < processMemoryField(t, output, ":memory") {
		t.Fatalf(":max-memory %v below :memory in %q", peak, output)
	}
	if count := processMemoryField(t, output, ":num-allocs"); count < 100 {
		t.Fatalf(":num-allocs %v is not a process count", count)
	}
}
