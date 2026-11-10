package lp

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The scenarios mirror src/test/stl_unordered_map_order_control.cpp; its
// output (libstdc++ ground truth) is the test vector.

type stdOrderLCG struct{ state uint64 }

func (g *stdOrderLCG) next() uint32 {
	g.state = g.state*6364136223846793005 + 1442695040888963407
	return uint32(g.state >> 33)
}

func stdOrderLine(tag string, m *StdUnorderedRationalMap) string {
	var b strings.Builder
	fmt.Fprintf(&b, "order %s %d", tag, m.BucketCount())
	for _, k := range m.Keys() {
		fmt.Fprintf(&b, " %d", k)
	}
	return b.String()
}

func TestStdUnorderedMapMatchesLibstdcxx(t *testing.T) {
	f, err := os.Open("test_vectors/libstdcxx_unordered_map_order.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var want []string
	var policy stdPrimeRehashPolicy
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "prime ") {
			fields := strings.Fields(line)
			n, _ := strconv.ParseUint(fields[1], 10, 64)
			p, _ := strconv.ParseUint(fields[2], 10, 64)
			if got := policy.nextBucket(n); got != p {
				t.Fatalf("_M_next_bkt(%d) = %d, want %d", n, got, p)
			}
			continue
		}
		want = append(want, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	var got []string
	{
		m := NewStdUnorderedRationalMap()
		for k := uint32(0); k < 40; k++ {
			m.Put(k, nil)
			got = append(got, stdOrderLine(fmt.Sprintf("asc%d", k), m))
		}
	}
	{
		m := NewStdUnorderedRationalMap()
		m.Put(1, nil)
		m.Put(0, nil)
		got = append(got, stdOrderLine("pair10", m))
		m2 := NewStdUnorderedRationalMap()
		m2.Put(0, nil)
		m2.Put(1, nil)
		got = append(got, stdOrderLine("pair01", m2))
	}
	g := &stdOrderLCG{state: 88172645463325252}
	{
		m := NewStdUnorderedRationalMap()
		for step := 1; step <= 3000; step++ {
			k := g.next() % 97
			if _, ok := m.Get2(k); ok {
				m.Del(k)
			} else {
				m.Put(k, nil)
			}
			if step%25 == 0 {
				got = append(got, stdOrderLine(fmt.Sprintf("toggle%d", step), m))
			}
		}
		m.Clear()
		m.Put(5, nil)
		m.Put(200, nil)
		m.Put(3, nil)
		got = append(got, stdOrderLine("afterclear", m))
	}
	{
		m := NewStdUnorderedRationalMap()
		keys := make([]uint32, 0, 5000)
		for k := uint32(0); k < 5000; k++ {
			keys = append(keys, k*7+g.next()%7)
		}
		for i := len(keys); i > 1; i-- {
			j := int(g.next() % uint32(i))
			keys[i-1], keys[j] = keys[j], keys[i-1]
		}
		for i, k := range keys {
			m.Put(k, nil)
			if (i+1)%1000 == 0 {
				got = append(got, stdOrderLine(fmt.Sprintf("large%d", i+1), m))
			}
		}
		for i := 0; i < len(keys); i += 3 {
			m.Del(keys[i])
		}
		got = append(got, stdOrderLine("largeerase", m))
	}
	if len(got) != len(want) {
		t.Fatalf("%d order lines, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d:\n got %.300s\nwant %.300s", i, got[i], want[i])
		}
	}
}
