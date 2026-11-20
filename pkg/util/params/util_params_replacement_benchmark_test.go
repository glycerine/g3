package params

import "testing"

// Exercise a solver's mostly scalar parameter blocks and the less common
// replacement of an owning rational. Each iteration touches 32 distinct blocks.
func BenchmarkParamsReplacementBlocks(b *testing.B) {
	for _, name := range []string{"Scalar", "Numeral", "ReusedNumeral"} {
		b.Run(name, func(b *testing.B) {
			c, sy, q := paramsTestOwner(b)
			key, err := sy.Intern("value")
			if err != nil {
				b.Fatal(err)
			}
			r, err := q.NewValue()
			if err != nil {
				b.Fatal(err)
			}
			if err = q.SetFractionSigned(&r, 1, 3); err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := q.Del(&r); err != nil {
					b.Fatal(err)
				}
			})
			var blocks [32]ParamsRef
			for i := range blocks {
				blocks[i], err = c.NewValue()
				if err != nil {
					b.Fatal(err)
				}
			}
			b.Cleanup(func() {
				for i := range blocks {
					if err := c.Del(&blocks[i]); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				for i := range blocks {
					if name != "Scalar" {
						if err := c.SetRat(&blocks[i], key, &r); err != nil {
							b.Fatal(err)
						}
					}
					if err := c.SetUInt(&blocks[i], key, uint32(i)); err != nil {
						b.Fatal(err)
					}
				}
				if name != "ReusedNumeral" {
					for i := range blocks {
						if err := c.Del(&blocks[i]); err != nil {
							b.Fatal(err)
						}
						blocks[i], err = c.NewValue()
						if err != nil {
							b.Fatal(err)
						}
					}
				}
			}
		})
	}
}
