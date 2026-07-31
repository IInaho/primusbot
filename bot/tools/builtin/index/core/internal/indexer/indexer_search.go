package indexer

import graphpkg "nekocode/bot/tools/builtin/index/core/internal/graph"

// SearchFTS performs full-text symbol search through the backing index DB.
func (i *Indexer) SearchFTS(term string, limit int) ([]*graphpkg.Node, error) {
	return i.db.SearchFTS(term, limit)
}
