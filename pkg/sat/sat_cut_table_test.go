package sat

import "testing"

func TestSATCutShiftTableAgainstProjectionOracle(t *testing.T) {
	supers := [][]uint32{{1}, {1, 3}, {1, 3, 8}, {1, 3, 8, 10}, {1, 3, 8, 10, 15}}
	for _, domain := range supers {
		super := testSATCut(domain, 0)
		for subset := uint32(1); subset < uint32(1)<<uint32(len(domain)); subset++ {
			var ids []uint32
			for i, id := range domain {
				if subset>>uint32(i)&1 != 0 {
					ids = append(ids, id)
				}
			}
			inputs := uint32(len(ids))
			for table := uint64(0); table < uint64(1)<<(uint64(1)<<inputs); table++ {
				if inputs > 3 && table > 255 {
					break
				}
				cut := testSATCut(ids, table)
				got := cut.ShiftTable(super)
				for assignment := uint32(0); assignment < uint32(1)<<uint32(len(domain)); assignment++ {
					var projected uint32
					for i, id := range ids {
						for j, sid := range domain {
							if id == sid {
								projected |= (assignment >> uint32(j) & 1) << uint32(i)
							}
						}
					}
					if got>>assignment&1 != table>>projected&1 {
						t.Fatalf("domain %v ids %v table %x assignment %x", domain, ids, table, assignment)
					}
				}
			}
		}
	}
}

func TestSATCutEvalPreservesSourceChannels(t *testing.T) {
	c := testSATCut([]uint32{0, 2, 3}, 0b10100110)
	env := []satCutVal{{t: 0xaaaaaaaaaaaaaaaa}, {}, {t: 0xcccccccccccccccc}, {t: 0xf0f0f0f0f0f0f0f0}}
	got := c.Eval(env)
	var want uint64
	for lane := uint32(0); lane < 64; lane++ {
		offset := env[0].t>>lane&1 | (env[2].t>>lane&1)<<1 | (env[3].t>>lane&1)<<2
		want |= c.Table() >> offset & 1 << lane
	}
	if got.t != want || got.f != want {
		t.Fatalf("eval t=%016x f=%016x want=%016x", got.t, got.f, want)
	}
	id := newSATCut(2)
	env[2].f = 0x1234
	if got := id.Eval(env); got != env[2] {
		t.Fatal("identity fast path", got, env[2])
	}
}
