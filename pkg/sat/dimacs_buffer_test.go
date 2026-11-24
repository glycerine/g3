package sat

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

type fragmentedDIMACSInput struct {
	data     []byte
	maxChunk int
	finalErr error
	empty    bool
}

func (r *fragmentedDIMACSInput) Read(p []byte) (int, error) {
	if r.empty {
		r.empty = false
		return 0, nil
	}
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data[:min(len(r.data), r.maxChunk)])
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.finalErr
	}
	return n, nil
}

func TestDIMACSIntegersAcrossBufferBoundaries(t *testing.T) {
	for _, n := range []int{1, 3, 7, 64, 4096} {
		for _, padding := range []int{0, 1, 2, 15, 63, 4092, 4094, 4095, 4096} {
			for _, finalErr := range []error{nil, io.ErrUnexpectedEOF} {
				source := "c" + strings.Repeat("a", padding) + "\n-23 +4294967298-5 0\v62 0"
				r := &fragmentedDIMACSInput{[]byte(source), n, finalErr, true}
				got, err := ReadDIMACSCNF(r)
				if err != nil || got.VariableCount != 62 || len(got.Clauses) != 2 ||
					len(got.Clauses[0]) != 3 || got.Clauses[0][0] != -23 ||
					got.Clauses[0][1] != 2 || got.Clauses[0][2] != -5 ||
					len(got.Clauses[1]) != 1 || got.Clauses[1][0] != 62 {
					t.Fatalf("chunk %d, padding %d, final error %v: got %+v, %v", n, padding, finalErr, got, err)
				}
			}
		}
	}
}

func TestDIMACSBorrowedReaderPosition(t *testing.T) {
	for _, tc := range []struct {
		input, unread string
		wantErr       error
	}{
		{"1-3 0\n23 xRemainder", "xRemainder", ErrDIMACSParse},
		{"1-3 0\n", "\n", nil},
	} {
		for _, size := range []int{4096, 8192} {
			r := bufio.NewReaderSize(strings.NewReader(tc.input), size)
			_, err := ReadDIMACSCNF(r)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("reader size %d, input %q: got %v, want %v", size, tc.input, err, tc.wantErr)
			}
			if err := r.UnreadByte(); err != nil {
				t.Fatal(err)
			}
			left, err := io.ReadAll(r)
			if err != nil || string(left) != tc.unread {
				t.Fatalf("reader size %d, input %q: got %q, %v", size, tc.input, left, err)
			}
		}
	}
}

type dimacsReadFunc func([]byte) (int, error)

func (read dimacsReadFunc) Read(p []byte) (int, error) { return read(p) }

func TestDIMACSRechecksSolverAfterReadingInput(t *testing.T) {
	solver, _, _ := satSolverCheckFixture(t)
	reads := 0
	input := dimacsReadFunc(func(p []byte) (int, error) {
		reads++
		switch reads {
		case 1:
			return copy(p, "3 "), nil
		case 2:
			satOK(t, solver.Close())
			return copy(p, "2 x"), io.EOF
		}
		t.Fatal("unexpected read")
		return 0, io.EOF
	})
	if err := ParseDIMACS(input, solver); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("reading the next literal after close: got %v, want closed solver", err)
	}
}

func BenchmarkDIMACSIntegers(b *testing.B) {
	data := bytes.Repeat([]byte("13 -75 122 +742 909-313 0\n"), 1000)
	for _, mode := range []string{"raw", "buffered"} {
		b.Run(mode, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var input io.Reader = bytes.NewReader(data)
				if mode == "buffered" {
					input = bufio.NewReader(input)
				}
				stream := newDIMACSStreamBuffer(input)
				sum := int32(0)
				for stream.skipWhitespace(); stream.value != -1; stream.skipWhitespace() {
					n, err := stream.parseInt()
					if err != nil {
						b.Fatal(err)
					}
					sum += n
				}
				if sum != 1398000 {
					b.Fatalf("literal sum %d", sum)
				}
			}
		})
	}
}
