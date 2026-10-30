package sorting

import "testing"

func TestUnsignedNetworkRegisteredSourceCases(t *testing.T) {
	for _, tc := range []struct {
		input []uint32
		want  []uint32
	}{
		{[]uint32{0, 1, 0, 1, 1, 0}, []uint32{0, 0, 0, 0, 0, 1, 1, 1}},
		{[]uint32{0, 1, 2, 1, 1, 3}, []uint32{0, 0, 0, 1, 1, 1, 2, 3}},
	} {
		got := sourceUnsignedSort(tc.input)
		if len(got) != len(tc.want) {
			t.Fatalf("source output length %v: %d want %d", tc.input, len(got), len(tc.want))
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("source sort %v: output[%d]=%d want %d", tc.input, i, got[i], tc.want[i])
			}
		}
	}

	for _, n := range []uint32{5, 8} {
		limit := uint32(1) << n
		for mask := uint32(0); mask < limit; mask++ {
			input := make([]uint32, n)
			ones := uint32(0)
			for i := range input {
				input[i] = (mask >> uint(i)) & 1
				ones += input[i]
			}
			got := sourceUnsignedSort(input)
			padded := uint32(1)
			for padded < n {
				padded <<= 1
			}
			if uint32(len(got)) != padded {
				t.Fatalf("source padding n=%d mask=%d len=%d want=%d", n, mask, len(got), padded)
			}
			for i, value := range got {
				want := uint32(0)
				if uint32(i) >= padded-ones {
					want = 1
				}
				if value != want {
					t.Fatalf("source exhaustive n=%d mask=%d output[%d]=%d want %d", n, mask, i, value, want)
				}
			}
		}
	}
}

func TestUnsignedNetworkSourceStatePersistsAcrossCalls(t *testing.T) {
	var network Uint32Network
	first := network.Sort([]uint32{1, 0, 1, 0})
	second := network.Sort([]uint32{1, 0, 1, 0})
	for _, output := range [][]uint32{first, second} {
		want := []uint32{0, 0, 1, 1}
		for i := range want {
			if output[i] != want[i] {
				t.Fatalf("reused source network output %v: got %v want %v", []uint32{1, 0, 1, 0}, output, want)
			}
		}
	}
	if len(network.currentv) != 8 || len(network.nextv) != 8 {
		t.Fatalf("source mapping columns lengths = %d/%d, want 8/8 after two calls", len(network.currentv), len(network.nextv))
	}
}

func TestUnsignedNetworkEmptyAndSingletonSourceCases(t *testing.T) {
	for _, input := range [][]uint32{nil, {7}} {
		got := sourceUnsignedSort(input)
		if len(got) != len(input) {
			t.Fatalf("source short input %v: got %v", input, got)
		}
		for i := range input {
			if got[i] != input[i] {
				t.Fatalf("source short input %v changed to %v", input, got)
			}
		}
		if len(input) == 0 && got != nil {
			t.Fatalf("empty source input returned non-nil output %v", got)
		}
		if len(input) == 1 && &got[0] == &input[0] {
			t.Fatal("source output aliases the input")
		}
	}
}

func sourceUnsignedSort(input []uint32) []uint32 {
	var network Uint32Network
	return network.Sort(input)
}
