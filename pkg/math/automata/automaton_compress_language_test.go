package automata

import "testing"

func TestAutomatonCompressSourceCheapDuplicateAndLanguage(t *testing.T) {
	a := New(0, []uint32{2}, []Move{
		EpsilonMove(0, 1),
		ConditionMove(0, 2, 6),
		ConditionMove(1, 2, 6),
	})
	before := make(map[string]bool)
	words := [][]uint32{nil, {5}, {6}, {6, 6}}
	for _, word := range words {
		before[wordKey(word)] = a.Accepts(word)
	}
	a.Compress()
	if a.MoveCount() != 1 || a.Moves()[0] != ConditionMove(0, 2, 6) {
		t.Fatalf("cheap duplicate rule = %#v", a.Moves())
	}
	for _, word := range words {
		if got := a.Accepts(word); got != before[wordKey(word)] {
			t.Fatalf("word %v acceptance changed from %v to %v", word, before[wordKey(word)], got)
		}
	}
}

func wordKey(word []uint32) string {
	key := ""
	for _, symbol := range word {
		key += string(rune(symbol + 1))
	}
	return key
}
