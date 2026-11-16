//go:build g3trace

package main

// AST id fidelity: g3 must create and delete AST nodes in the same order as
// z3 4.7.1, so that recycled ids (and everything ordered by id) agree. The
// mk_var_bug trace records every node creation ("mk_ast: id") and deletion
// ("del_ast: id"), for all managers, in program order. For each vector in
// test_vectors/ast_event_digests.txt this test runs the g3trace build with
// -tr:mk_var_bug and checks the event sequence (its first prefix events,
// or all of them for prefix 0) against the recorded count and SHA-256, so
// it runs without z3. With a z3 TRACE build it also compares the events
// directly and reports the first difference:
//
//	G3_CPP_TRACE_REFERENCE=<repo>/build_trace/z3 go test -tags g3trace ./cmd/g3 -run TestASTEventOrder
//
// G3_UPDATE_AST_EVENT_DIGESTS=1 (with G3_CPP_TRACE_REFERENCE) rewrites the
// digests from z3's events. The test is skipped under -short.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const astEventDigestFile = "test_vectors/ast_event_digests.txt"

type astEventDigest struct {
	name   string
	prefix int // 0: the whole run
	count  int
	sum    string
}

func readASTEventDigests(t *testing.T) (header []string, digests []astEventDigest) {
	f, err := os.Open(astEventDigestFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			header = append(header, line)
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 4 {
			t.Fatalf("%s: malformed line %q", astEventDigestFile, line)
		}
		prefix, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			t.Fatalf("%s: malformed line %q", astEventDigestFile, line)
		}
		digests = append(digests, astEventDigest{fields[0], prefix, count, fields[3]})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return header, digests
}

// astEvents runs bin on input and returns its mk_ast/del_ast events.
func astEvents(t *testing.T, bin, input string) []string {
	var events []string
	for _, r := range runTraced(t, bin, input, []string{"mk_var_bug"}) {
		if !strings.HasPrefix(r.site, "mk_var_bug ") {
			continue
		}
		for _, line := range strings.Split(r.text, "\n") {
			if strings.HasPrefix(line, "mk_ast: ") || strings.HasPrefix(line, "del_ast: ") {
				events = append(events, line)
			}
		}
	}
	return events
}

func astEventSum(events []string) string {
	h := sha256.New()
	for _, e := range events {
		h.Write([]byte(e))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func astEventPrefix(events []string, prefix int) []string {
	if prefix > 0 && len(events) > prefix {
		return events[:prefix]
	}
	return events
}

func TestASTEventOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("AST event order comparison runs only without -short")
	}
	reference := os.Getenv("G3_CPP_TRACE_REFERENCE")
	update := os.Getenv("G3_UPDATE_AST_EVENT_DIGESTS") != ""
	if update && reference == "" {
		t.Fatal("G3_UPDATE_AST_EVENT_DIGESTS needs G3_CPP_TRACE_REFERENCE")
	}
	header, digests := readASTEventDigests(t)
	t.Cleanup(cleanupTraceG3)
	g3 := goldenTraceG3(t)
	for i := range digests {
		d := &digests[i]
		t.Run(d.name, func(t *testing.T) {
			input, err := filepath.Abs(filepath.Join("test_vectors", d.name))
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if reference != "" {
				want = astEventPrefix(astEvents(t, reference, input), d.prefix)
				if update {
					d.count, d.sum = len(want), astEventSum(want)
				}
			}
			got := astEventPrefix(astEvents(t, g3, input), d.prefix)
			if want != nil {
				n := len(got)
				if len(want) < n {
					n = len(want)
				}
				for j := 0; j < n; j++ {
					if got[j] != want[j] {
						lo := j - 8
						if lo < 0 {
							lo = 0
						}
						t.Fatalf("event %d differs: g3 %q, z3 %q\ng3 events %d..%d: %q\nz3 events %d..%d: %q",
							j+1, got[j], want[j], lo+1, j+1, got[lo:j+1], lo+1, j+1, want[lo:j+1])
					}
				}
				if len(got) != len(want) {
					t.Fatalf("g3 has %d events, z3 %d", len(got), len(want))
				}
			}
			if len(got) != d.count || astEventSum(got) != d.sum {
				t.Fatalf("g3 events: %d, sha256 %s; recorded: %d, sha256 %s", len(got), astEventSum(got), d.count, d.sum)
			}
		})
	}
	if update && !t.Failed() {
		var b strings.Builder
		for _, line := range header {
			b.WriteString(line + "\n")
		}
		for _, d := range digests {
			fmt.Fprintf(&b, "%s %d %d %s\n", d.name, d.prefix, d.count, d.sum)
		}
		if err := os.WriteFile(astEventDigestFile, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
