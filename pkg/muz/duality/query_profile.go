package duality

import "io"

// SetProfileOutput borrows a destination for duality.profile reports. A nil
// writer restores the source stdout destination. Query results are separate.
func (b *Backend) SetProfileOutput(out io.Writer) error {
	if err := b.open(); err != nil {
		return err
	}
	b.queries.profileOutput = out
	return nil
}
