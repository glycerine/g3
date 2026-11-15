package lp

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The tests below replay the scenarios of the g++ 13 control programs
// src/test/stl_controls/stl_unordered_map_order_control.cpp
// (test_vectors/libstdcxx_unordered_map_order.txt) and
// stl_unordered_string_map_order_control
// (test_vectors/libstdcxx_unordered_string_map_order.txt) through the
// lp_solver/mps_reader map types of std_unordered_lp_maps.go (owner task
// 2026-11-14), so each of them is pinned to libstdc++'s node order. Never
// edit the recorded outputs; regenerate them only from the C++ programs.

// stdUintOrderTarget adapts one unsigned-keyed map type to the control
// scenarios.
type stdUintOrderTarget struct {
	put     func(uint32)
	del     func(uint32)
	has     func(uint32) bool
	clear   func()
	keys    func() []uint32
	buckets func() uint64
}

func stdUintOrderLines(t *testing.T, fresh func() stdUintOrderTarget) []string {
	t.Helper()
	line := func(tag string, m stdUintOrderTarget) string {
		var b strings.Builder
		fmt.Fprintf(&b, "order %s %d", tag, m.buckets())
		for _, k := range m.keys() {
			fmt.Fprintf(&b, " %d", k)
		}
		return b.String()
	}
	var got []string
	m := fresh()
	for k := uint32(0); k < 40; k++ {
		m.put(k)
		got = append(got, line(fmt.Sprintf("asc%d", k), m))
	}
	m = fresh()
	m.put(1)
	m.put(0)
	got = append(got, line("pair10", m))
	m = fresh()
	m.put(0)
	m.put(1)
	got = append(got, line("pair01", m))
	g := &stdOrderLCG{state: 88172645463325252}
	m = fresh()
	for step := 1; step <= 3000; step++ {
		k := g.next() % 97
		if m.has(k) {
			m.del(k)
		} else {
			m.put(k)
		}
		if step%25 == 0 {
			got = append(got, line(fmt.Sprintf("toggle%d", step), m))
		}
	}
	if m.clear != nil {
		m.clear()
		m.put(5)
		m.put(200)
		m.put(3)
		got = append(got, line("afterclear", m))
	}
	m = fresh()
	keys := make([]uint32, 0, 5000)
	for k := uint32(0); k < 5000; k++ {
		keys = append(keys, k*7+g.next()%7)
	}
	for i := len(keys); i > 1; i-- {
		j := int(g.next() % uint32(i))
		keys[i-1], keys[j] = keys[j], keys[i-1]
	}
	for i, k := range keys {
		m.put(k)
		if (i+1)%1000 == 0 {
			got = append(got, line(fmt.Sprintf("large%d", i+1), m))
		}
	}
	for i := 0; i < len(keys); i += 3 {
		m.del(keys[i])
	}
	got = append(got, line("largeerase", m))
	return got
}

func stdUintControlLines(t *testing.T) []string {
	t.Helper()
	f, err := os.Open("test_vectors/libstdcxx_unordered_map_order.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var want []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	for sc.Scan() {
		if line := sc.Text(); !strings.HasPrefix(line, "prime ") {
			want = append(want, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return want
}

func TestStdUnorderedLPMapsUnsignedKeysMatchLibstdcxx(t *testing.T) {
	want := stdUintControlLines(t)
	targets := map[string]func() stdUintOrderTarget{
		"StdUnorderedUintMap": func() stdUintOrderTarget {
			m := NewStdUnorderedUintMap()
			return stdUintOrderTarget{
				put: func(k uint32) { m.Put(k, k+1) },
				del: m.Del,
				has: func(k uint32) bool {
					v, ok := m.Get2(k)
					if ok && v != k+1 {
						t.Fatal("value", k, v)
					}
					return ok
				},
				clear: m.Clear, keys: m.Keys, buckets: m.BucketCount,
			}
		},
		"StdUnorderedFloatRow": func() stdUintOrderTarget {
			m := NewStdUnorderedFloatRow()
			return stdUintOrderTarget{
				put: func(k uint32) { m.Set(k, float64(k)/2) },
				del: m.Del,
				has: func(k uint32) bool {
					v, ok := m.Get2(k)
					if ok && v != float64(k)/2 {
						t.Fatal("value", k, v)
					}
					return ok
				},
				keys: m.Keys, buckets: m.slots.BucketCount,
			}
		},
		"StdUnorderedFloatRows": func() stdUintOrderTarget {
			m := NewStdUnorderedFloatRows()
			return stdUintOrderTarget{
				put:  func(k uint32) { m.Set(k, NewStdUnorderedFloatRow()) },
				del:  m.Del,
				has:  func(k uint32) bool { _, ok := m.Get2(k); return ok },
				keys: m.Keys, buckets: m.slots.BucketCount,
			}
		},
		"StdUnorderedRationalRows": func() stdUintOrderTarget {
			m := NewStdUnorderedRationalRows()
			return stdUintOrderTarget{
				put:  func(k uint32) { m.Set(k, NewStdUnorderedRationalMap()) },
				del:  m.Del,
				has:  func(k uint32) bool { _, ok := m.Get2(k); return ok },
				keys: m.Keys, buckets: m.slots.BucketCount,
			}
		},
	}
	for name, fresh := range targets {
		got := stdUintOrderLines(t, fresh)
		// Types without Clear skip the control's afterclear line.
		w := want
		if fresh().clear == nil {
			w = nil
			for _, line := range want {
				if !strings.HasPrefix(line, "order afterclear ") {
					w = append(w, line)
				}
			}
		}
		if len(got) != len(w) {
			t.Fatalf("%s: %d order lines, want %d", name, len(got), len(w))
		}
		for i := range w {
			if got[i] != w[i] {
				t.Fatalf("%s line %d:\n got %.300s\nwant %.300s", name, i, got[i], w[i])
			}
		}
	}
}

// The string-keyed m_rows / m_row_columns types follow the recorded
// "mps<i>" insertion lines of the string-map control.
func TestStdUnorderedLPMapsStringKeysMatchLibstdcxx(t *testing.T) {
	data, err := os.ReadFile("test_vectors/libstdcxx_unordered_string_map_order.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "order" {
			want[fields[1]] = strings.Join(fields[2:], " ")
		}
	}
	mps := []string{"XONE", "YTWO", "ZTHREE", "ALPHA", "BETA", "GAMMA"}
	floats := NewStdUnorderedStringFloatMap()
	rationals := NewStdUnorderedStringRationalMap()
	rows := newFloatMPSRowMap()
	for i, w := range mps {
		floats.Set(w, float64(i))
		rationals.Set(w, nil)
		rows.Set(w, newFloatMPSRow(w, uint32(i), mpsLess))
		tag := fmt.Sprintf("mps%d", i)
		for name, keys := range map[string][]string{"float": floats.Keys(), "rational": rationals.Keys(), "rows": rows.slots.Keys()} {
			var b strings.Builder
			fmt.Fprintf(&b, "%d", floats.slots.BucketCount())
			for _, k := range keys {
				fmt.Fprintf(&b, " %s", stdStringHex(k))
			}
			if b.String() != want[tag] {
				t.Fatalf("%s %s:\n got %s\nwant %s", name, tag, b.String(), want[tag])
			}
		}
	}
	for name, v := range floats.All() {
		if row, ok := rows.Get2(name); !ok || float64(row.index) != v {
			t.Fatal("values follow their keys", name, v)
		}
	}
}
