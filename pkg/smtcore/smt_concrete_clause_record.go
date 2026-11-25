package smtcore

import "github.com/glycerine/g3/pkg/ast"

// The auxiliary and learned directories own one pointer per clause, as in
// smt::context. Callers can shorten a live clause independently of the
// solver, so each entry remembers the last literal range the solver published.
type smtClauseDirectoryDetails struct {
	literals []SMTLiteral
	proof    ast.ASTHandle
	owner    SMTJustification
}

const (
	smtRecordRoot uint8 = 1 << iota
	smtRecordRetired
	smtClauseDeleted
	smtRecordWatchStep // the high bits count the watches successfully removed
)

func (c *SMTClause) registerDirectory(root bool, proof ast.ASTHandle, owner SMTJustification) {
	if root {
		c.directoryFlags |= smtRecordRoot
	}
	if detail := c.directoryDetails(); detail != nil {
		detail.literals, detail.proof, detail.owner = c.literals, proof, owner
	} else {
		c.directoryWidth = uint8(len(c.literals))
	}
}

func (c *SMTClause) directoryDetails() *smtClauseDirectoryDetails {
	if c.smtClauseLemmaData == nil {
		return nil
	}
	return c.directory
}

// A caller closing the object and the solver retiring its directory entry
// are independent events. Pop can also fail on a later clause and be retried.
func (c *SMTClause) activeClause() *SMTClause {
	if c == nil || c.directoryFlags&smtRecordRetired != 0 {
		return nil
	}
	return c
}

func (c *SMTClause) directoryLiterals() []SMTLiteral {
	if c.directoryWidth != 0 {
		return c.literals[:c.directoryWidth:c.directoryWidth]
	}
	if d := c.directoryDetails(); d != nil {
		return d.literals
	}
	return nil
}

func (c *SMTClause) setDirectoryLiterals(literals []SMTLiteral) {
	if d := c.directoryDetails(); d != nil {
		d.literals = literals
		return
	}
	c.directoryWidth = uint8(len(literals))
}

func (c *SMTClause) watchProgress() uint8 { return c.directoryFlags / smtRecordWatchStep }
func (c *SMTClause) advanceWatch()        { c.directoryFlags += smtRecordWatchStep }
func (c *SMTClause) resetWatches()        { c.directoryFlags &^= 3 * smtRecordWatchStep }

func (c *SMTClause) retireFromDirectory() error {
	if err := c.Close(); err != nil {
		return err
	}
	c.directoryFlags |= smtRecordRetired
	return nil
}

// Snapshots keep their old literal range until the entire suffix can be
// removed. Drop directory-owned references at that point even if a caller
// keeps the closed clause alive.
func releaseDirectorySuffix(clauses []*SMTClause) {
	for i, c := range clauses {
		c.releaseDirectoryDetails()
		clauses[i] = nil
	}
}

func (c *SMTClause) releaseDirectoryDetails() {
	if c.directoryWidth != 0 {
		return
	}
	if detail := c.directoryDetails(); detail != nil {
		*detail = smtClauseDirectoryDetails{}
		c.directory = nil
	}
}
