package core

import (
	"strings"
	"testing"
)

func TestCollectStatisticsOutputIsSourceMapOrdered(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	applyStatistics(t, r, f.goal(t, f.app(t, f.f, f.a)))
	text := f.output.String()
	if !strings.HasPrefix(text, "(\n") || !strings.HasSuffix(text, ")\n") {
		t.Fatal(text)
	}
	previous := ""
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, " :") {
			continue
		}
		key := strings.SplitN(strings.TrimPrefix(line, " :"), "    ", 2)[0]
		if previous != "" && key < previous {
			t.Fatal("not source std::map order", previous, key, text)
		}
		previous = key
	}
	for _, want := range []string{" :function-applications    4", " :uninterpreted-functions    1", " :uninterpreted-sort-occurrences    3"} {
		if !strings.Contains(text, want) {
			t.Fatal("missing", want, text)
		}
	}
}
