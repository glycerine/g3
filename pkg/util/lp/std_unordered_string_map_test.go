package lp

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The scenarios mirror the g++ 13.3 control program
// src/test/stl_controls/stl_unordered_string_map_order_control.cpp
// (AUDIT.md util_lp_06-F1); its
// output (libstdc++ ground truth) is
// test_vectors/libstdcxx_unordered_string_map_order.txt. Regenerate it only
// by recompiling and rerunning the C++ program, never by editing it.

func stdStringHex(s string) string {
	if s == "" {
		return "-"
	}
	return hex.EncodeToString([]byte(s))
}

func TestStdUnorderedStringMapMatchesLibstdcxx(t *testing.T) {
	data, err := os.ReadFile("test_vectors/libstdcxx_unordered_string_map_order.txt")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	for sc.Scan() {
		want = append(want, sc.Text())
	}
	var got []string
	put := func(m *StdUnorderedStringMap, w string, v uint32) {
		if _, ok := m.Get2(w); !ok {
			got = append(got, fmt.Sprintf("hash %s %d", stdStringHex(w), stdHashBytes(w, stdStringHashSeed)))
		}
		m.Put(w, v)
	}
	dump := func(tag string, m *StdUnorderedStringMap) {
		var b strings.Builder
		fmt.Fprintf(&b, "order %s %d", tag, m.BucketCount())
		for _, k := range m.Keys() {
			fmt.Fprintf(&b, " %s", stdStringHex(k))
		}
		got = append(got, b.String())
	}
	mps := []string{"XONE", "YTWO", "ZTHREE", "ALPHA", "BETA", "GAMMA"}
	columns := NewStdUnorderedStringMap()
	for i, w := range mps {
		put(columns, w, uint32(i))
		dump(fmt.Sprintf("mps%d", i), columns)
	}
	names := NewStdUnorderedStringMap()
	for k, v := range columns.All() {
		put(names, k, v)
	}
	dump("names", names)
	g := &stdOrderLCG{state: 88172645463325252}
	m := NewStdUnorderedStringMap()
	var words []string
	for i := uint32(0); i < 300; i++ {
		n := g.next() % 21
		w := make([]byte, n)
		for j := range w {
			w[j] = byte(g.next() & 255)
		}
		words = append(words, string(w))
		put(m, string(w), i)
		if (i+1)%25 == 0 {
			dump(fmt.Sprintf("rand%d", i+1), m)
		}
	}
	for i := 0; i < len(words); i += 3 {
		m.Del(words[i])
	}
	dump("randerase", m)
	for i := 0; i < len(words); i += 6 {
		put(m, words[i], uint32(i))
	}
	dump("randreinsert", m)
	if len(got) != len(want) {
		t.Fatalf("%d lines, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d:\n got %.300s\nwant %.300s", i, got[i], want[i])
		}
	}
}
