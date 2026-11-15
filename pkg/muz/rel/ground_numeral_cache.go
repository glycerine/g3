package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// groundNumeralCache keeps the column numerals that
// default_table_filter_interpreted_fn::should_remove builds for every row
// (dl_decl_util::mk_numeral) alive for the life of one compiled
// interpreted-filter instruction, across rows and saturation iterations.
// The source rebuilds each numeral on every row and frees it at the next
// row's args.reset(). Numerals are hash-consed and building one charges no
// resource step, so reusing them leaves every ground condition, rewrite step
// and rlimit count unchanged; only the work of rebuilding identical numerals
// is saved. Go-only performance cache: an interpreted filter over a total
// table (a head term such as (bvadd x #x0001) binds a fresh column to the
// whole BV16 domain) evaluates the condition on 65536 rows per delta row.
type groundNumeralCache struct {
	relations *RelationManager
	index     map[groundNumeralKey]int
	refs      []ast.ASTRef
}

type groundNumeralKey struct {
	sort  ast.ASTHandle
	value Element
}

// groundNumeralCacheLimit bounds the retained numerals; beyond it rows build
// and free their numerals as the source does.
const groundNumeralCacheLimit = 1 << 20

func newGroundNumeralCache(relations *RelationManager) *groundNumeralCache {
	return &groundNumeralCache{relations: relations, index: make(map[groundNumeralKey]int)}
}

// numeral returns the cached numeral for (sort, value). When the cache is
// full (or nil) it returns cached=false and the caller builds and releases
// the numeral itself.
func (c *groundNumeralCache) numeral(sort ast.ASTHandle, value Element) (handle ast.ASTHandle, cached bool, err error) {
	if c == nil {
		return ast.ASTHandle{}, false, nil
	}
	key := groundNumeralKey{sort: sort, value: value}
	if at, found := c.index[key]; found {
		handle, err = c.relations.manager.RefGet(c.refs[at])
		return handle, err == nil, err
	}
	if len(c.refs) >= groundNumeralCacheLimit {
		return ast.ASTHandle{}, false, nil
	}
	reference, err := c.relations.DecodeValue(sort, value)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	handle, err = c.relations.manager.RefGet(reference)
	if err != nil {
		return ast.ASTHandle{}, false, errors.Join(err, c.relations.manager.DelRef(&reference))
	}
	c.index[key] = len(c.refs)
	c.refs = append(c.refs, reference)
	return handle, true, nil
}

// close releases the numerals in reverse creation order (deterministic AST
// id recycling).
func (c *groundNumeralCache) close() (result error) {
	if c == nil {
		return nil
	}
	for i := len(c.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, c.relations.manager.DelRef(&c.refs[i]))
	}
	c.refs = nil
	c.index = nil
	return result
}
